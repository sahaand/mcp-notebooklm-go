package notebooklm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/saeedata/notebooklm-go/rpc"
)

// Answers can take a while to generate; the stream is read in one response.
const chatTimeout = 3 * time.Minute

var chatUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ChatAPI provides conversation operations for NotebookLM notebooks.
type ChatAPI struct {
	client *Client
}

// AskOptions configures a chat question.
type AskOptions struct {
	// SourceIDs filters the question to specific sources. Empty means all sources.
	SourceIDs []string
	// ConversationID continues an existing conversation (for follow-ups).
	// Empty continues the notebook's current conversation, creating one if
	// none exists — the web UI's default behavior.
	ConversationID string
}

// Ask submits a question to a notebook and returns the answer with citations.
func (c *ChatAPI) Ask(ctx context.Context, notebookID, question string, opts AskOptions) (*AskResult, error) {
	sourceIDs := opts.SourceIDs
	if len(sourceIDs) == 0 {
		sources, err := c.client.Sources.List(ctx, notebookID)
		if err != nil {
			return nil, fmt.Errorf("ask: %w", err)
		}
		for _, s := range sources {
			sourceIDs = append(sourceIDs, s.ID)
		}
	}

	body, err := c.postQuery(ctx, notebookID, question, sourceIDs, opts.ConversationID, true)
	if err != nil {
		return nil, err
	}
	parsed, err := parseChatStream(body)
	if err != nil {
		return nil, err
	}

	result := &AskResult{
		Answer:         parsed.answer,
		ConversationID: opts.ConversationID,
		IsFollowUp:     opts.ConversationID != "",
		References:     parsed.references,
	}
	if result.ConversationID == "" {
		// The server records the turn on the notebook's current conversation;
		// read its ID back rather than trusting the stream's session ID.
		id, err := c.GetLastConversationID(ctx, notebookID)
		if err != nil || id == "" {
			id = parsed.conversationID
		}
		result.ConversationID = id
	}
	return result, nil
}

func (c *ChatAPI) postQuery(ctx context.Context, notebookID, question string, sourceIDs []string, conversationID string, retry bool) ([]byte, error) {
	c.client.mu.Lock()
	cookieHeader := c.client.auth.CookieHeader()
	csrf := c.client.auth.CSRFToken
	sid := c.client.auth.SessionID
	c.client.mu.Unlock()

	reqURL, body, err := buildChatRequest(notebookID, question, sourceIDs, conversationID, csrf, sid, c.client.nextReqID())
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create query request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("X-Same-Domain", "1")

	httpClient := *c.client.httpClient
	httpClient.Timeout = chatTimeout
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query request failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		if retry {
			if refreshErr := c.client.RefreshAuth(ctx); refreshErr == nil {
				return c.postQuery(ctx, notebookID, question, sourceIDs, conversationID, false)
			}
		}
		return nil, fmt.Errorf("authentication failed (HTTP %d) — run 'notebooklm login'", resp.StatusCode)
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, errors.New("chat rate limit exceeded — please wait before retrying")
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("chat request failed (HTTP %d)", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read query response: %w", err)
	}
	if os.Getenv("NOTEBOOKLM_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG] chat status=%d body=%s\n", resp.StatusCode, data)
	}
	return data, nil
}

// buildChatRequest assembles the URL and form body for the streamed chat
// endpoint. The params are
// [[[[id]],...], question, history, [2,null,[1],[1]], conversation_id, null, null, notebook_id, 1]
// sent as f.req=[null, "<params json>"].
func buildChatRequest(notebookID, question string, sourceIDs []string, conversationID, csrf, sessionID string, reqid int) (string, string, error) {
	var convID any
	if conversationID != "" {
		convID = conversationID
	}
	params := []any{
		rpc.NestIDs(sourceIDs, 2),
		question,
		nil, // conversation history; the server keeps it for known conversations
		[]any{2, nil, []any{1}, []any{1}},
		convID,
		nil,
		nil,
		notebookID,
		1,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", "", fmt.Errorf("marshal chat params: %w", err)
	}
	freq, err := json.Marshal([]any{nil, string(paramsJSON)})
	if err != nil {
		return "", "", fmt.Errorf("marshal chat request: %w", err)
	}
	body := "f.req=" + url.QueryEscape(string(freq))
	if csrf != "" {
		body += "&at=" + url.QueryEscape(csrf)
	}
	body += "&"

	q := url.Values{
		"bl":     {rpc.DefaultBuildLabel},
		"hl":     {"en"},
		"_reqid": {strconv.Itoa(reqid)},
		"rt":     {"c"},
	}
	if sessionID != "" {
		q.Set("f.sid", sessionID)
	}
	return rpc.QueryURL + "?" + q.Encode(), body, nil
}

// Configure sets chat behavior for a notebook. customPrompt is used only with
// rpc.ChatCustom.
func (c *ChatAPI) Configure(ctx context.Context, notebookID string, goal rpc.ChatGoal, responseLength rpc.ChatResponseLength, customPrompt string) error {
	goalArray := []any{int(goal)}
	if goal == rpc.ChatCustom {
		goalArray = []any{int(goal), customPrompt}
	}
	settings := []any{goalArray, []any{int(responseLength)}}
	params := []any{notebookID, []any{[]any{nil, nil, nil, nil, nil, nil, nil, settings}}}
	if _, err := c.client.rpcCallPolicy(ctx, rpc.RenameNotebook, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected); err != nil {
		return fmt.Errorf("configure chat: %w", err)
	}
	return nil
}

// SetMode applies a predefined chat mode.
func (c *ChatAPI) SetMode(ctx context.Context, notebookID string, mode ChatMode) error {
	var goal rpc.ChatGoal
	var length rpc.ChatResponseLength

	switch mode {
	case ChatModeDefault:
		goal = rpc.ChatDefault
		length = rpc.ChatResponseDefault
	case ChatModeLearningGuide:
		goal = rpc.ChatLearningGuide
		length = rpc.ChatResponseLonger
	case ChatModeConcise:
		goal = rpc.ChatDefault
		length = rpc.ChatResponseShorter
	case ChatModeDetailed:
		goal = rpc.ChatDefault
		length = rpc.ChatResponseLonger
	default:
		return fmt.Errorf("unknown chat mode: %s", mode)
	}

	return c.Configure(ctx, notebookID, goal, length, "")
}

// GetConversationTurns returns up to limit of the most recent Q&A pairs of a
// conversation, oldest first.
func (c *ChatAPI) GetConversationTurns(ctx context.Context, notebookID, conversationID string, limit int) ([]ConversationTurn, error) {
	params := []any{[]any{}, nil, nil, conversationID, limit * 2} // the server counts questions and answers separately
	result, err := c.client.rpcCallPolicy(ctx, rpc.GetConversationTurns, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected)
	if err != nil {
		return nil, fmt.Errorf("get conversation turns: %w", err)
	}
	turns := parseConversationTurns(result)
	if len(turns) > limit {
		turns = turns[len(turns)-limit:]
	}
	return turns, nil
}

// GetLastConversationID returns the most recent conversation ID for a notebook,
// or "" if it has none.
func (c *ChatAPI) GetLastConversationID(ctx context.Context, notebookID string) (string, error) {
	params := []any{[]any{}, nil, notebookID, 1}
	result, err := c.client.rpcCallPolicy(ctx, rpc.GetLastConversationID, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected)
	if err != nil {
		return "", fmt.Errorf("get last conversation ID: %w", err)
	}
	return lastConversationID(result), nil
}

// lastConversationID reads [[[conv_id], ...], ...]: the first [string] row wins.
func lastConversationID(result any) string {
	groups, _ := result.([]any)
	for _, g := range groups {
		group, _ := g.([]any)
		for _, r := range group {
			if row, ok := r.([]any); ok && len(row) > 0 {
				if id, ok := row[0].(string); ok {
					return id
				}
			}
		}
	}
	return ""
}

// parseConversationTurns reads a GET_CONVERSATION_TURNS response. Turns are at
// result[0], newest first; each is [?, ?, role, question, answer] with role 1
// for a question (text at [3]) and 2 for an answer (text at [4][0][0]).
// Returns Q&A pairs oldest first.
func parseConversationTurns(result any) []ConversationTurn {
	arr, _ := result.([]any)
	raw, _ := listAt(arr, 0)
	turns := make([]ConversationTurn, 0)
	for i := len(raw) - 1; i >= 0; i-- {
		row, _ := raw[i].([]any)
		if role, _ := intAt(row, 2); role != 1 || len(row) < 4 {
			continue
		}
		t := ConversationTurn{TurnNumber: len(turns) + 1}
		t.Query, _ = row[3].(string)
		if i > 0 {
			if next, _ := raw[i-1].([]any); len(next) > 4 {
				if role, _ := intAt(next, 2); role == 2 {
					t.Answer = turnAnswerText(next)
					i--
				}
			}
		}
		turns = append(turns, t)
	}
	return turns
}

func turnAnswerText(row []any) string {
	l1, _ := listAt(row, 4)
	l2, _ := listAt(l1, 0)
	if len(l2) > 0 {
		s, _ := l2[0].(string)
		return s
	}
	return ""
}

// ---- streamed answer parsing ----------------------------------------------

type chatParse struct {
	answer         string
	references     []ChatReference
	conversationID string
}

type chatChunk struct {
	text           string
	isAnswer       bool
	isFinal        bool
	references     []ChatReference
	conversationID string
	parseable      bool
	terminal       bool
}

// parseChatStream parses the streamed chat response: )]}'-prefixed chunks of
// [["wrb.fr", null, "<inner json>"], ...] frames. Chunks are cumulative; the
// marked answer chunk flagged final (inner[4] == true) wins, falling back to
// the longest marked answer, then the longest unmarked text.
func parseChatStream(data []byte) (*chatParse, error) {
	text := rpc.StripAntiXSSI(string(data))
	var final, bestMarked, bestUnmarked *chatChunk
	var convID string
	parseable, terminal := 0, false

	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if _, err := strconv.Atoi(line); err == nil {
			i++
			if i >= len(lines) {
				break
			}
			line = lines[i]
		}
		chunk, err := parseChatChunk(line)
		if err != nil {
			return nil, err
		}
		if chunk.parseable {
			parseable++
		}
		terminal = terminal || chunk.terminal
		if chunk.conversationID != "" {
			convID = chunk.conversationID
		}
		if chunk.text == "" {
			continue
		}
		c := chunk
		if chunk.isAnswer {
			if chunk.isFinal {
				final = &c
			}
			if bestMarked == nil || len(chunk.text) > len(bestMarked.text) {
				bestMarked = &c
			}
		} else if bestUnmarked == nil || len(chunk.text) > len(bestUnmarked.text) {
			bestUnmarked = &c
		}
	}

	if parseable == 0 {
		if terminal {
			return nil, errors.New("chat rate limit reached: the stream ended without an answer payload; check the conversation history before retrying")
		}
		return nil, errors.New("no parseable chunks in streamed chat response; the response was empty or the API format changed")
	}

	res := &chatParse{conversationID: convID, references: []ChatReference{}}
	for _, pick := range []*chatChunk{final, bestMarked, bestUnmarked} {
		if pick != nil {
			res.answer, res.references = pick.text, pick.references
			break
		}
	}
	return res, nil
}

// parseChatChunk decodes one chunk line. It returns at the first frame that
// carries answer text.
func parseChatChunk(line string) (chatChunk, error) {
	var out chatChunk
	var items []any
	if err := json.Unmarshal([]byte(line), &items); err != nil {
		return out, nil
	}
	for _, it := range items {
		item, _ := it.([]any)
		if len(item) < 2 {
			continue
		}
		switch item[0] {
		case "er":
			code := any(nil)
			if len(item) > 2 {
				code = item[2]
			}
			return out, fmt.Errorf("chat request failed: the server returned an error frame (code %v); the turn may have been recorded", code)
		case "e":
			out.terminal = true
			continue
		case "wrb.fr":
		default:
			continue
		}
		if len(item) < 3 {
			continue
		}
		innerJSON, ok := item[2].(string)
		if !ok {
			if len(item) > 5 {
				if payload, ok := item[5].([]any); ok && len(payload) > 0 {
					return out, chatRejection(payload)
				}
			}
			continue
		}
		var inner []any
		if err := json.Unmarshal([]byte(innerJSON), &inner); err != nil {
			continue
		}
		out.parseable = true
		if len(inner) > 4 && inner[4] == true {
			out.isFinal = true
		}
		if len(inner) == 0 {
			continue
		}
		first, ok := inner[0].([]any)
		if !ok {
			return out, errors.New("streamed chat answer row is not a list; the API format may have changed")
		}
		if convBlock, ok := listAt(first, 2); ok && len(convBlock) > 0 {
			if id, ok := convBlock[0].(string); ok {
				out.conversationID = id
			}
		}
		if len(first) == 0 {
			continue
		}
		text, _ := first[0].(string)
		if text == "" {
			continue
		}
		out.text = text
		typeBlock, _ := listAt(first, 4)
		if marker, ok := intAt(typeBlock, 4); ok && marker == 1 {
			out.isAnswer = true
		}
		out.references = parseCitations(typeBlock)
		return out, nil
	}
	return out, nil
}

// chatRejection turns the google.rpc.Status attached to a null answer frame
// into an error.
func chatRejection(payload []any) error {
	msg := ""
	if len(payload) > 1 {
		msg, _ = payload[1].(string)
	}
	suffix := ""
	if msg != "" {
		suffix = " The server said: " + msg
	}
	if entries, ok := listAt(payload, 2); ok {
		for _, e := range entries {
			if entry, ok := e.([]any); ok && len(entry) > 0 {
				if t, _ := entry[0].(string); strings.Contains(t, "UserDisplayableError") {
					return errors.New("chat request was rate limited or rejected by the API; wait a few seconds and try again." + suffix)
				}
			}
		}
	}
	return fmt.Errorf("chat request was rejected by the server (status %v); the question may be too long or malformed.%s", payload[0], suffix)
}

// parseCitations reads the citation list at answer[4][3]. Each citation is
// [[chunk_id, ...], detail] where detail[5] holds the source ID (a nested UUID)
// and detail[4][0] the cited fragment's structural elements.
// CitationNumber is the 1-based position in the list, matching the [N]
// markers in the answer text.
func parseCitations(typeBlock []any) []ChatReference {
	refs := []ChatReference{}
	cites, _ := listAt(typeBlock, 3)
	for i, cv := range cites {
		cite, _ := cv.([]any)
		detail, ok := listAt(cite, 1)
		if !ok {
			continue
		}
		var sourceIDData any
		if len(detail) > 5 {
			sourceIDData = detail[5]
		}
		sourceID := findUUID(sourceIDData, 10)
		if sourceID == "" {
			continue
		}
		ref := ChatReference{SourceID: sourceID, CitationNumber: i + 1}
		if chunk, ok := listAt(cite, 0); ok && len(chunk) > 0 {
			ref.ChunkID, _ = chunk[0].(string)
		}
		if fragment, ok := listAt(detail, 4); ok {
			if elements, ok := listAt(fragment, 0); ok {
				ref.CitedText, ref.StartChar, ref.EndChar = citedText(elements)
			}
		}
		refs = append(refs, ref)
	}
	return refs
}

// citedText joins the text of a fragment's paragraph elements
// ([start, end, [[[start, end, [text, style]], ...], ...]]), trimming overlaps.
// Offsets are in UTF-16 code units, as sent by the server.
func citedText(elements []any) (string, int, int) {
	type block struct {
		start, end int
		text       string
	}
	var blocks []block
	for _, ev := range elements {
		el, _ := ev.([]any)
		start, ok1 := intAt(el, 0)
		end, ok2 := intAt(el, 1)
		para, ok3 := listAt(el, 2)
		if !ok1 || !ok2 || !ok3 {
			continue
		}
		var sb strings.Builder
		runs, _ := listAt(para, 0)
		for _, rv := range runs {
			run, _ := rv.([]any)
			if textRun, ok := listAt(run, 2); ok && len(textRun) > 0 {
				s, _ := textRun[0].(string)
				sb.WriteString(s)
			}
		}
		blocks = append(blocks, block{start, end, sb.String()})
	}
	if len(blocks) == 0 {
		return "", 0, 0
	}
	start, end := blocks[0].start, blocks[0].end
	for _, b := range blocks {
		start, end = min(start, b.start), max(end, b.end)
	}
	var sb strings.Builder
	cursor := start
	for _, b := range blocks {
		if b.end <= cursor {
			continue
		}
		text := b.text
		if b.start < cursor {
			u := utf16.Encode([]rune(text))
			if skip := cursor - b.start; skip < len(u) {
				text = string(utf16.Decode(u[skip:]))
			} else {
				text = ""
			}
		}
		sb.WriteString(text)
		cursor = b.end
	}
	return sb.String(), start, end
}

func findUUID(v any, depth int) string {
	if depth <= 0 {
		return ""
	}
	switch x := v.(type) {
	case string:
		if chatUUIDPattern.MatchString(x) {
			return x
		}
	case []any:
		for _, item := range x {
			if id := findUUID(item, depth-1); id != "" {
				return id
			}
		}
	}
	return ""
}
