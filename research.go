package notebooklm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/saeedata/notebooklm-go/rpc"
)

// ResearchAPI provides web research operations.
type ResearchAPI struct {
	client *Client
}

// Research status codes reported by POLL_RESEARCH at task_info[4].
const (
	researchInProgress    = 1
	researchCompleted     = 2
	researchNoResults     = 3
	researchCancelled     = 4
	researchCompletedDeep = 6
)

// ResearchSource is one source found by a research run. A deep run also
// returns its written report as a source with ReportMarkdown set and no URL.
type ResearchSource struct {
	URL            string `json:"url,omitempty"`
	Title          string `json:"title"`
	ResultType     int    `json:"result_type,omitempty"`
	ReportMarkdown string `json:"report_markdown,omitempty"`
}

// ResearchResult represents the state of a research run.
type ResearchResult struct {
	// TaskID identifies the run for Poll, Import and Cancel. For deep research
	// this is the run's report ID, not the session ID the start call returns
	// first (polling by the session ID finds nothing).
	TaskID string `json:"task_id"`
	// Status is "in_progress", "completed", "failed" or "not_found".
	Status  string           `json:"status"`
	Query   string           `json:"query,omitempty"`
	Summary string           `json:"summary,omitempty"`
	Sources []ResearchSource `json:"sources,omitempty"`
	// Report is the deep-research report markdown, when present.
	Report string `json:"report,omitempty"`
}

// StartFast initiates a fast web research query.
func (r *ResearchAPI) StartFast(ctx context.Context, notebookID, query string) (*ResearchResult, error) {
	return r.Start(ctx, notebookID, query, "web", "fast")
}

// StartDeep initiates a deep web research query.
func (r *ResearchAPI) StartDeep(ctx context.Context, notebookID, query string) (*ResearchResult, error) {
	return r.Start(ctx, notebookID, query, "web", "deep")
}

// Start initiates a research run. source is "web" or "drive"; mode is "fast"
// or "deep" (deep supports web only).
func (r *ResearchAPI) Start(ctx context.Context, notebookID, query, source, mode string) (*ResearchResult, error) {
	sourceType := 1
	switch strings.ToLower(source) {
	case "web", "":
	case "drive":
		sourceType = 2
	default:
		return nil, fmt.Errorf("invalid research source %q (use web or drive)", source)
	}

	var method rpc.RPCMethod
	var params []any
	switch strings.ToLower(mode) {
	case "fast", "":
		method, params = rpc.StartFastResearch, []any{[]any{query, sourceType}, nil, 1, notebookID}
	case "deep":
		if sourceType != 1 {
			return nil, fmt.Errorf("deep research only supports web sources")
		}
		method, params = rpc.StartDeepResearch, []any{nil, []any{1}, []any{query, sourceType}, 5, notebookID}
	default:
		return nil, fmt.Errorf("invalid research mode %q (use fast or deep)", mode)
	}

	result, err := r.client.RPCCall(ctx, method, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("start %s research: %w", mode, err)
	}
	arr, _ := result.([]any)
	if len(arr) == 0 {
		return nil, fmt.Errorf("start %s research: response has no task ID", mode)
	}
	taskID, _ := arr[0].(string)
	if reportID, ok := stringAt(arr, 1); ok && reportID != "" {
		taskID = reportID
	}
	if taskID == "" {
		return nil, fmt.Errorf("start %s research: response has no task ID", mode)
	}
	return &ResearchResult{TaskID: taskID, Status: "in_progress", Query: query}, nil
}

// Poll returns the current state of a research run.
func (r *ResearchAPI) Poll(ctx context.Context, notebookID, taskID string) (*ResearchResult, error) {
	params := []any{nil, nil, notebookID}
	result, err := r.client.rpcCallPolicy(ctx, rpc.PollResearch, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected)
	if err != nil {
		return nil, fmt.Errorf("poll research: %w", err)
	}
	for _, task := range parseResearchTasks(result) {
		if task.TaskID == taskID {
			return task, nil
		}
	}
	return &ResearchResult{TaskID: taskID, Status: "not_found"}, nil
}

// Import adds research results to the notebook as sources and returns them.
func (r *ResearchAPI) Import(ctx context.Context, notebookID, taskID string, sources []ResearchSource) ([]Source, error) {
	entries := make([]any, 0, len(sources))
	for _, s := range sources {
		if s.ReportMarkdown != "" {
			entries = append(entries, []any{nil, []any{s.Title, s.ReportMarkdown}, nil, 3, nil, nil, nil, nil, nil, nil, 3})
		} else if s.URL != "" {
			entries = append(entries, []any{nil, nil, []any{s.URL, s.Title}, nil, nil, nil, nil, nil, nil, nil, 2})
		}
	}
	if len(entries) == 0 {
		return nil, nil
	}
	params := []any{nil, []any{1}, taskID, notebookID, entries}
	result, err := r.client.RPCCall(ctx, rpc.ImportResearch, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("import research: %w", err)
	}
	return parseImportedSources(result, notebookID), nil
}

// Cancel stops an in-flight research run. The server does not report whether
// the run existed; poll afterwards to confirm.
func (r *ResearchAPI) Cancel(ctx context.Context, notebookID, taskID string) error {
	params := []any{nil, nil, taskID}
	if _, err := r.client.RPCCall(ctx, rpc.CancelResearch, params, "/notebook/"+notebookID, true); err != nil {
		return fmt.Errorf("cancel research: %w", err)
	}
	return nil
}

// WaitForCompletion polls research until it's complete or times out.
func (r *ResearchAPI) WaitForCompletion(ctx context.Context, notebookID, taskID string, timeout time.Duration) (*ResearchResult, error) {
	deadline := time.Now().Add(timeout)
	interval := 3 * time.Second

	for {
		res, err := r.Poll(ctx, notebookID, taskID)
		if err != nil {
			return nil, err
		}
		switch res.Status {
		case "completed":
			return res, nil
		case "failed":
			return nil, fmt.Errorf("research task %s failed or found no results", taskID)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for research %s", taskID)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// parseResearchTasks reads POLL_RESEARCH: [[task, ...]] or [task, ...], each
// task being [id, info] with info = [?, [query, source_type], mode,
// [sources, summary], status].
func parseResearchTasks(result any) []*ResearchResult {
	tasks, _ := result.([]any)
	if len(tasks) > 0 {
		if first, ok := tasks[0].([]any); ok && len(first) > 0 {
			if _, nested := first[0].([]any); nested {
				tasks = first
			}
		}
	}
	out := make([]*ResearchResult, 0, len(tasks))
	for _, t := range tasks {
		task, _ := t.([]any)
		id, _ := stringAt(task, 0)
		info, ok := listAt(task, 1)
		if id == "" || !ok {
			continue
		}
		res := &ResearchResult{TaskID: id, Status: "in_progress"}
		if q, ok := listAt(info, 1); ok {
			res.Query, _ = stringAt(q, 0)
		}
		if bundle, ok := listAt(info, 3); ok {
			res.Summary, _ = stringAt(bundle, 1)
			rows, _ := listAt(bundle, 0)
			for _, sv := range rows {
				src, ok := parseResearchSource(sv)
				if !ok {
					continue
				}
				if src.ReportMarkdown != "" && res.Report == "" {
					res.Report = src.ReportMarkdown
				}
				res.Sources = append(res.Sources, src)
			}
		}
		switch code, _ := intAt(info, 4); code {
		case researchCompleted, researchCompletedDeep:
			res.Status = "completed"
		case researchNoResults, researchCancelled:
			res.Status = "failed"
		}
		out = append(out, res)
	}
	return out
}

// parseResearchSource reads [url|null, title|[title, markdown], hint,
// result_type, ?, ?, [text, kind, ...]]; content kind 3 marks a report.
func parseResearchSource(v any) (ResearchSource, bool) {
	row, _ := v.([]any)
	if len(row) < 2 {
		return ResearchSource{}, false
	}
	var src ResearchSource
	src.URL, _ = stringAt(row, 0)
	switch t := row[1].(type) {
	case string:
		src.Title = t
	case []any:
		src.Title, _ = stringAt(t, 0)
		src.ReportMarkdown, _ = stringAt(t, 1)
	}
	src.ResultType, _ = intAt(row, 3)
	if content, ok := listAt(row, 6); ok {
		if kind, _ := intAt(content, 1); kind == 3 {
			if md, ok := stringAt(content, 0); ok && md != "" {
				src.ReportMarkdown = md
			}
		}
	}
	if src.URL == "" && src.ReportMarkdown == "" && src.Title == "" {
		return ResearchSource{}, false
	}
	return src, true
}

// parseImportedSources reads IMPORT_RESEARCH: [[[id], title, ...], ...],
// possibly wrapped in one more list.
func parseImportedSources(result any, notebookID string) []Source {
	rows, _ := result.([]any)
	if len(rows) > 0 {
		if first, ok := rows[0].([]any); ok && len(first) > 0 && isImportRow(first[0]) {
			rows = first
		}
	}
	sources := make([]Source, 0, len(rows))
	for _, r := range rows {
		row, _ := r.([]any)
		if !isImportRow(row) {
			continue
		}
		if src := sourceFromEntry(row, notebookID, false); src.ID != "" {
			sources = append(sources, src)
		}
	}
	return sources
}

func isImportRow(v any) bool {
	row, ok := v.([]any)
	if !ok || len(row) < 2 {
		return false
	}
	_, isList := row[0].([]any)
	return row[0] == nil || isList
}

func stringAt(v []any, i int) (string, bool) {
	if i < 0 || i >= len(v) {
		return "", false
	}
	s, ok := v[i].(string)
	return s, ok
}
