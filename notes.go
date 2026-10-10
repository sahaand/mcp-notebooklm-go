package notebooklm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/saeedata/notebooklm-go/rpc"
)

// NotesAPI provides note and mind map operations.
type NotesAPI struct {
	client *Client
}

// List returns all notes in a notebook. Mind maps (stored as notes with a
// JSON body) and deleted notes are excluded; see ListMindMaps.
func (n *NotesAPI) List(ctx context.Context, notebookID string) ([]Note, error) {
	rows, err := n.fetchRows(ctx, notebookID)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	notes := make([]Note, 0, len(rows))
	for _, row := range rows {
		if note, ok := noteFromRow(row, notebookID); ok && !isMindMapContent(note.Content) {
			notes = append(notes, note)
		}
	}
	return notes, nil
}

// ListMindMaps returns the notebook's mind maps. Each is a note whose Content
// is the mind map's JSON tree.
func (n *NotesAPI) ListMindMaps(ctx context.Context, notebookID string) ([]Note, error) {
	rows, err := n.fetchRows(ctx, notebookID)
	if err != nil {
		return nil, fmt.Errorf("list mind maps: %w", err)
	}
	maps := make([]Note, 0)
	for _, row := range rows {
		if note, ok := noteFromRow(row, notebookID); ok && isMindMapContent(note.Content) {
			maps = append(maps, note)
		}
	}
	return maps, nil
}

// Create creates a new note in a notebook. NotebookLM creates an empty note
// first and sets its content with a follow-up update.
func (n *NotesAPI) Create(ctx context.Context, notebookID, title, content string) (*Note, error) {
	params := []any{notebookID, "", []any{1}, nil, title}
	result, err := n.client.RPCCall(ctx, rpc.CreateNote, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("create note: %w", err)
	}
	noteID := createdNoteID(result)
	if noteID == "" {
		return nil, fmt.Errorf("create note: no note ID in response")
	}
	if err := n.update(ctx, notebookID, noteID, title, content); err != nil {
		return nil, fmt.Errorf("create note: note %s was created but setting its content failed: %w", noteID, err)
	}
	return &Note{ID: noteID, NotebookID: notebookID, Title: title, Content: content}, nil
}

// Update replaces a note's content and title. An empty title keeps the
// note's current title.
func (n *NotesAPI) Update(ctx context.Context, notebookID, noteID, title, content string) (*Note, error) {
	if title == "" {
		rows, err := n.fetchRows(ctx, notebookID)
		if err != nil {
			return nil, fmt.Errorf("update note %s: %w", noteID, err)
		}
		found := false
		for _, row := range rows {
			if note, ok := noteFromRow(row, notebookID); ok && note.ID == noteID {
				title, found = note.Title, true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("update note %s: note not found in notebook %s", noteID, notebookID)
		}
	}
	if err := n.update(ctx, notebookID, noteID, title, content); err != nil {
		return nil, fmt.Errorf("update note %s: %w", noteID, err)
	}
	return &Note{ID: noteID, NotebookID: notebookID, Title: title, Content: content}, nil
}

func (n *NotesAPI) update(ctx context.Context, notebookID, noteID, title, content string) error {
	params := []any{notebookID, noteID, []any{[]any{[]any{content, title, []any{}, 0}}}}
	_, err := n.client.rpcCallPolicy(ctx, rpc.UpdateNote, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected)
	return err
}

// Delete removes a note (or mind map) from a notebook. Deleting a note that
// does not exist succeeds.
func (n *NotesAPI) Delete(ctx context.Context, notebookID, noteID string) error {
	params := []any{notebookID, nil, []any{noteID}}
	_, err := n.client.rpcCallPolicy(ctx, rpc.DeleteNote, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected)
	var rpcErr *rpc.RPCError
	if errors.As(err, &rpcErr) && rpcErr.GRPCCode == rpc.GRPCNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete note %s: %w", noteID, err)
	}
	return nil
}

// fetchRows returns the normalized note rows ([id, inner, ...]) of a notebook,
// including deleted rows and mind maps.
func (n *NotesAPI) fetchRows(ctx context.Context, notebookID string) ([][]any, error) {
	result, err := n.client.rpcCallPolicy(ctx, rpc.GetNotesAndMindMaps, []any{notebookID}, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected)
	if err != nil {
		return nil, err
	}
	return noteRows(result)
}

// GenerateMindMapOptions configures mind map generation.
type GenerateMindMapOptions struct {
	SourceIDs    []string // empty means all sources
	Language     string   // default "en"
	Instructions string
}

// GenerateMindMap generates a mind map from notebook sources and saves it as a
// note, as the web app does. Generation is synchronous; the returned note's
// Content is the mind map's JSON tree.
func (n *NotesAPI) GenerateMindMap(ctx context.Context, notebookID string, opts GenerateMindMapOptions) (*Note, error) {
	ids, err := n.client.Artifacts.sourceIDsOrAll(ctx, notebookID, opts.SourceIDs)
	if err != nil {
		return nil, fmt.Errorf("generate mind map: %w", err)
	}
	params := mindMapParams(ids, languageOrDefault(opts.Language), opts.Instructions)
	result, err := n.client.rpcCallPolicy(ctx, rpc.GenerateMindMap, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected)
	if err != nil {
		return nil, fmt.Errorf("generate mind map: %w", err)
	}
	content, ok := mindMapJSON(result)
	if !ok {
		return nil, fmt.Errorf("generate mind map: no mind map in response")
	}
	title := "Mind Map"
	var tree struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(content), &tree) == nil && tree.Name != "" {
		title = tree.Name
	}
	return n.Create(ctx, notebookID, title, content)
}

func mindMapParams(sourceIDs []string, language, instructions string) []any {
	return []any{
		rpc.NestIDs(sourceIDs, 2),
		nil, nil, nil, nil,
		[]any{"interactive_mindmap", []any{[]any{"[CONTEXT]", instructions}}, language},
		nil,
		[]any{2, nil, []any{1}},
	}
}

// mindMapJSON returns the mind map JSON at result[0][0] (a string, or a decoded
// value that is re-encoded).
func mindMapJSON(result any) (string, bool) {
	arr, _ := result.([]any)
	inner, _ := listAt(arr, 0)
	if len(inner) == 0 {
		return "", false
	}
	if s, ok := inner[0].(string); ok {
		return s, true
	}
	b, err := json.Marshal(inner[0])
	if err != nil {
		return "", false
	}
	return string(b), true
}

// noteRows extracts note rows from a GET_NOTES_AND_MIND_MAPS response. Rows come
// either as the response itself or wrapped in its first element, and a row is
// either legacy [id, content, ...] or current [null, [id, content, meta, ?, title]];
// both are normalized to [id, ...].
func noteRows(result any) ([][]any, error) {
	arr, ok := result.([]any)
	if result != nil && !ok {
		return nil, fmt.Errorf("unrecognized notes response shape")
	}
	if len(arr) == 0 {
		return nil, nil
	}
	container := arr
	if !isNoteRowLike(arr[0]) {
		container, _ = arr[0].([]any)
	}
	rows := make([][]any, 0, len(container))
	for _, item := range container {
		if !isNoteRowLike(item) {
			continue
		}
		row := item.([]any)
		if _, legacy := row[0].(string); !legacy {
			nested := row[1].([]any)
			row = append([]any{nested[0], nested}, row[2:]...)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func isNoteRowLike(v any) bool {
	row, ok := v.([]any)
	if !ok || len(row) == 0 {
		return false
	}
	if _, ok := row[0].(string); ok {
		return true
	}
	if row[0] != nil || len(row) < 2 {
		return false
	}
	nested, ok := row[1].([]any)
	if !ok || len(nested) == 0 {
		return false
	}
	_, ok = nested[0].(string)
	return ok
}

// noteFromRow decodes a normalized note row. Content is row[1] (legacy string)
// or row[1][1]; title is row[1][4]; creation time is row[1][2][2][0]. Deleted
// rows ([id, null, 2]) are reported as not ok.
func noteFromRow(row []any, notebookID string) (Note, bool) {
	note := Note{NotebookID: notebookID}
	note.ID, _ = row[0].(string)
	if note.ID == "" || len(row) < 2 || row[1] == nil {
		return note, false
	}
	switch inner := row[1].(type) {
	case string:
		note.Content = inner
	case []any:
		if len(inner) > 1 {
			note.Content, _ = inner[1].(string)
		}
		if len(inner) > 4 {
			note.Title, _ = inner[4].(string)
		}
		if meta, ok := listAt(inner, 2); ok {
			if ts, ok := listAt(meta, 2); ok && len(ts) > 0 {
				if secs, ok := ts[0].(float64); ok {
					note.CreatedAt = time.Unix(int64(secs), 0).UTC()
				}
			}
		}
	}
	return note, true
}

// isMindMapContent reports whether a note body is a serialized mind map.
func isMindMapContent(content string) bool {
	return strings.HasPrefix(content, "{") &&
		(strings.Contains(content, `"children":`) || strings.Contains(content, `"nodes":`))
}

// createdNoteID reads the new note ID from a CREATE_NOTE response:
// result[0][0], or result[0] when that is the ID itself.
func createdNoteID(result any) string {
	arr, _ := result.([]any)
	if len(arr) == 0 {
		return ""
	}
	switch first := arr[0].(type) {
	case string:
		return first
	case []any:
		if len(first) > 0 {
			id, _ := first[0].(string)
			return id
		}
	}
	return ""
}
