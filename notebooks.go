package notebooklm

import (
	"context"
	"fmt"

	"github.com/saeedata/notebooklm-go/rpc"
)

// NotebooksAPI provides notebook CRUD operations.
type NotebooksAPI struct {
	client *Client
}

// List returns all notebooks.
func (n *NotebooksAPI) List(ctx context.Context) ([]Notebook, error) {
	params := []any{nil, 1, nil, []any{2}}
	result, err := n.client.RPCCall(ctx, rpc.ListNotebooks, params, "/", false)
	if err != nil {
		return nil, fmt.Errorf("list notebooks: %w", err)
	}

	arr, _ := result.([]any)
	if len(arr) == 0 {
		return nil, nil
	}
	raw, _ := arr[0].([]any)
	if raw == nil {
		raw = arr
	}

	notebooks := make([]Notebook, 0, len(raw))
	for _, item := range raw {
		if data, ok := item.([]any); ok {
			notebooks = append(notebooks, NotebookFromAPIResponse(data))
		}
	}
	return notebooks, nil
}

// Create creates a new notebook with the given title.
func (n *NotebooksAPI) Create(ctx context.Context, title string) (*Notebook, error) {
	result, err := n.client.RPCCall(ctx, rpc.CreateNotebook, createNotebookParams(title), "/", false)
	if err != nil {
		return nil, fmt.Errorf("create notebook: %w", err)
	}

	data, ok := result.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response type for create notebook")
	}
	nb := NotebookFromAPIResponse(data)
	return &nb, nil
}

// Get fetches a notebook by ID.
func (n *NotebooksAPI) Get(ctx context.Context, notebookID string) (*Notebook, error) {
	params := getNotebookParams(notebookID)
	result, err := n.client.RPCCall(ctx, rpc.GetNotebook, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("get notebook %s: %w", notebookID, err)
	}

	arr, _ := result.([]any)
	var nbData []any
	if len(arr) > 0 {
		nbData, _ = arr[0].([]any)
	}
	if nbData == nil {
		nbData, _ = result.([]any)
	}
	nb := NotebookFromAPIResponse(nbData)
	return &nb, nil
}

// Delete deletes a notebook by ID.
func (n *NotebooksAPI) Delete(ctx context.Context, notebookID string) error {
	params := []any{[]any{notebookID}, []any{2}}
	_, err := n.client.RPCCall(ctx, rpc.DeleteNotebook, params, "/", true)
	if err != nil {
		return fmt.Errorf("delete notebook %s: %w", notebookID, err)
	}
	return nil
}

// Rename renames a notebook and returns the updated notebook.
func (n *NotebooksAPI) Rename(ctx context.Context, notebookID, newTitle string) (*Notebook, error) {
	params := []any{notebookID, []any{[]any{nil, nil, nil, []any{nil, newTitle}}}}
	_, err := n.client.RPCCall(ctx, rpc.RenameNotebook, params, "/", true)
	if err != nil {
		return nil, fmt.Errorf("rename notebook %s: %w", notebookID, err)
	}
	return n.Get(ctx, notebookID)
}

// GetDescription returns AI-generated summary and suggested topics for a notebook.
func (n *NotebooksAPI) GetDescription(ctx context.Context, notebookID string) (*NotebookDescription, error) {
	params := []any{notebookID, []any{2}}
	result, err := n.client.RPCCall(ctx, rpc.Summarize, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("get description for notebook %s: %w", notebookID, err)
	}

	desc := &NotebookDescription{}
	arr, _ := result.([]any)
	if len(arr) == 0 {
		return desc, nil
	}

	outer, _ := arr[0].([]any)
	if outer == nil {
		return desc, nil
	}

	// Summary at outer[0][0]
	if len(outer) > 0 {
		if s0, ok := outer[0].([]any); ok && len(s0) > 0 {
			if s, ok := s0[0].(string); ok {
				desc.Summary = s
			}
		}
	}

	// Topics at outer[1][0]
	if len(outer) > 1 {
		if t1, ok := outer[1].([]any); ok && len(t1) > 0 {
			if topics, ok := t1[0].([]any); ok {
				for _, topic := range topics {
					if t, ok := topic.([]any); ok && len(t) >= 2 {
						q, _ := t[0].(string)
						p, _ := t[1].(string)
						desc.SuggestedTopics = append(desc.SuggestedTopics, SuggestedTopic{
							Question: q,
							Prompt:   p,
						})
					}
				}
			}
		}
	}

	return desc, nil
}

// RemoveFromRecent removes a notebook from the recently viewed list.
func (n *NotebooksAPI) RemoveFromRecent(ctx context.Context, notebookID string) error {
	params := []any{notebookID}
	_, err := n.client.RPCCall(ctx, rpc.RemoveRecentlyViewed, params, "/", true)
	return err
}

// GetMetadata returns notebook details combined with simplified source list.
func (n *NotebooksAPI) GetMetadata(ctx context.Context, notebookID string) (*NotebookMetadata, error) {
	type result struct {
		nb  *Notebook
		src []Source
		err error
	}
	nbCh := make(chan result, 1)
	srcCh := make(chan result, 1)

	go func() {
		nb, err := n.Get(ctx, notebookID)
		nbCh <- result{nb: nb, err: err}
	}()
	go func() {
		sources, err := n.client.Sources.List(ctx, notebookID)
		srcCh <- result{src: sources, err: err}
	}()

	nbRes := <-nbCh
	srcRes := <-srcCh

	if nbRes.err != nil {
		return nil, nbRes.err
	}
	if srcRes.err != nil {
		return nil, srcRes.err
	}

	summaries := make([]SourceSummary, 0, len(srcRes.src))
	for _, s := range srcRes.src {
		summaries = append(summaries, SourceSummary{
			Kind:  s.Kind(),
			Title: s.Title,
			URL:   s.URL,
		})
	}

	return &NotebookMetadata{
		Notebook: *nbRes.nb,
		Sources:  summaries,
	}, nil
}

func createNotebookParams(title string) []any {
	return []any{title, nil, nil, rpc.TemplateBlock()}
}
