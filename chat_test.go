package notebooklm

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

const (
	srcUUID  = "0f8fad5b-d9cb-469f-a165-70867728950e"
	convUUID = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
)

// streamLine wraps an inner chat payload as one rt=c chunk line.
func streamLine(t *testing.T, inner any) string {
	t.Helper()
	innerJSON, err := json.Marshal(inner)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := json.Marshal([]any{[]any{"wrb.fr", nil, string(innerJSON)}})
	if err != nil {
		t.Fatal(err)
	}
	return string(frame)
}

func stream(lines ...string) []byte {
	var b strings.Builder
	b.WriteString(")]}'\n\n")
	for _, l := range lines {
		fmt.Fprintf(&b, "%d\n%s\n", len(l)+1, l)
	}
	return []byte(b.String())
}

// answerInner builds an inner payload: [[text, null, [conv, turn, 1], null, typeBlock], null, null, null, final].
func answerInner(text string, marked, final bool, cites []any) []any {
	var typeBlock any
	if marked {
		typeBlock = []any{nil, nil, nil, cites, 1}
	}
	return []any{[]any{text, nil, []any{convUUID, "turn-1", 1}, nil, typeBlock}, nil, nil, nil, final}
}

func citation(chunkID, sourceID string, runs ...string) []any {
	var elements []any
	pos := 0
	for _, r := range runs {
		end := pos + len(r)
		elements = append(elements, []any{pos, end, []any{[]any{[]any{pos, end, []any{r, nil}}}}})
		pos = end
	}
	detail := []any{nil, nil, 0.9, []any{[]any{nil, 0, pos}}, []any{elements}, []any{[]any{sourceID}}}
	return []any{[]any{chunkID}, detail}
}

func TestParseChatStreamPicksFinalMarkedAnswer(t *testing.T) {
	cites := []any{citation("chunk-1", srcUUID, "Hello ", "world")}
	body := stream(
		streamLine(t, answerInner("Partial answer that is the longest one so far", true, false, nil)),
		streamLine(t, answerInner("Final answer [1]", true, true, cites)),
		`[["e",4,null,null,321]]`,
	)
	got, err := parseChatStream(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.answer != "Final answer [1]" || got.conversationID != convUUID {
		t.Fatalf("got %+v", got)
	}
	want := []ChatReference{{SourceID: srcUUID, CitationNumber: 1, CitedText: "Hello world", StartChar: 0, EndChar: 11, ChunkID: "chunk-1"}}
	if !reflect.DeepEqual(got.references, want) {
		t.Errorf("references:\n got  %+v\n want %+v", got.references, want)
	}
}

func TestParseChatStreamFallbacks(t *testing.T) {
	// No final flag: the longest marked answer wins over longer unmarked text.
	body := stream(
		streamLine(t, answerInner("short", true, false, nil)),
		streamLine(t, answerInner("a longer marked answer", true, false, nil)),
		streamLine(t, answerInner("unmarked thinking text that is even longer than both", false, false, nil)),
	)
	got, err := parseChatStream(body)
	if err != nil || got.answer != "a longer marked answer" {
		t.Fatalf("got (%+v, %v)", got, err)
	}

	// Only unmarked text: it is used.
	got, err = parseChatStream(stream(streamLine(t, answerInner("plain", false, true, nil))))
	if err != nil || got.answer != "plain" {
		t.Fatalf("unmarked: got (%+v, %v)", got, err)
	}

	// Parseable but empty answer is not an error.
	got, err = parseChatStream(stream(streamLine(t, []any{[]any{}})))
	if err != nil || got.answer != "" {
		t.Fatalf("empty: got (%+v, %v)", got, err)
	}
}

func TestParseChatStreamErrors(t *testing.T) {
	cases := map[string]struct {
		body []byte
		want string
	}{
		"terminal only": {stream(`[["di",12],["e",4,null,null,99]]`), "rate limit"},
		"garbage":       {[]byte("<html>login</html>"), "no parseable chunks"},
		"rejected":      {stream(`[["wrb.fr",null,null,null,null,[3]]]`), "rejected by the server (status 3)"},
		"user error":    {stream(`[["wrb.fr",null,null,null,null,[8,null,[["type.googleapis.com/x.UserDisplayableError"]]]]]`), "rate limited"},
		"er frame":      {stream(`[["er",null,500]]`), "error frame"},
	}
	for name, c := range cases {
		_, err := parseChatStream(c.body)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want error containing %q", name, err, c.want)
		}
	}
}

func TestCitedTextTrimsOverlap(t *testing.T) {
	elements := []any{
		[]any{0, 5, []any{[]any{[]any{0, 5, []any{"Hello", nil}}}}},
		[]any{3, 9, []any{[]any{[]any{3, 9, []any{"lo you", nil}}}}},
	}
	text, start, end := citedText(fromJSON(t, mustMarshal(elements)).([]any))
	if text != "Hello you" || start != 0 || end != 9 {
		t.Errorf("got (%q, %d, %d)", text, start, end)
	}
}

func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestBuildChatRequest(t *testing.T) {
	reqURL, body, err := buildChatRequest("nb-1", "What?", []string{"s1", "s2"}, "", "csrf", "sid", 200000)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(reqURL)
	if !strings.HasSuffix(u.Path, "/GenerateFreeFormStreamed") {
		t.Errorf("path %s", u.Path)
	}
	q := u.Query()
	if q.Get("_reqid") != "200000" || q.Get("rt") != "c" || q.Get("f.sid") != "sid" || q.Get("bl") == "" {
		t.Errorf("query %v", q)
	}
	form, err := url.ParseQuery(body)
	if err != nil {
		t.Fatal(err)
	}
	if form.Get("at") != "csrf" {
		t.Errorf("at = %q", form.Get("at"))
	}
	want := `[null,"[[[[\"s1\"]],[[\"s2\"]]],\"What?\",null,[2,null,[1],[1]],null,null,null,\"nb-1\",1]"]`
	if got := form.Get("f.req"); got != want {
		t.Errorf("f.req\n got  %s\n want %s", got, want)
	}

	_, body, _ = buildChatRequest("nb-1", "Q", nil, convUUID, "", "", 100000)
	form, _ = url.ParseQuery(body)
	if !strings.Contains(form.Get("f.req"), `\"`+convUUID+`\"`) || form.Has("at") {
		t.Errorf("follow-up body %s", body)
	}
}

func TestParseConversationTurns(t *testing.T) {
	// Newest first: answer 2, question 2, answer 1, question 1.
	result := fromJSON(t, `[[
		[null,null,2,null,[["Answer two"]]],
		[null,null,1,"Question two"],
		[null,null,2,null,[["Answer one"]]],
		[null,null,1,"Question one"]
	]]`)
	got := parseConversationTurns(result)
	want := []ConversationTurn{
		{Query: "Question one", Answer: "Answer one", TurnNumber: 1},
		{Query: "Question two", Answer: "Answer two", TurnNumber: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
	// A pending question without an answer yet.
	got = parseConversationTurns(fromJSON(t, `[[[null,null,1,"Unanswered"]]]`))
	if len(got) != 1 || got[0].Query != "Unanswered" || got[0].Answer != "" {
		t.Errorf("pending: got %+v", got)
	}
	if got := parseConversationTurns(nil); len(got) != 0 {
		t.Errorf("nil: got %+v", got)
	}
}

func TestLastConversationID(t *testing.T) {
	if got := lastConversationID(fromJSON(t, `[[["`+convUUID+`"]]]`)); got != convUUID {
		t.Errorf("got %q", got)
	}
	if got := lastConversationID(fromJSON(t, `[[]]`)); got != "" {
		t.Errorf("empty: got %q", got)
	}
}
