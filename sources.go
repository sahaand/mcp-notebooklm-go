package notebooklm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/saeedata/notebooklm-go/rpc"
)

// ErrSourceNotFound is returned (wrapped) when a source ID is not in the notebook.
var ErrSourceNotFound = errors.New("source not found")

var youtubeVideoIDPattern = regexp.MustCompile(`(?:youtube\.com/watch\?(?:.*&)?v=|youtu\.be/|youtube\.com/shorts/|youtube\.com/embed/|youtube\.com/live/)([a-zA-Z0-9_-]{11})`)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Discovery modes accepted by DiscoverSources.
var discoverModes = map[string]int{
	"default":     1, // LLM-ranked search
	"raw":         2, // plain search
	"curious":     3, // backend picks the topic; query is ignored
	"curious_raw": 4,
}

const uploadTimeout = 10 * time.Minute

// SourcesAPI provides source management operations.
type SourcesAPI struct {
	client *Client
}

// List returns all sources in a notebook.
func (s *SourcesAPI) List(ctx context.Context, notebookID string) ([]Source, error) {
	result, err := s.client.RPCCall(ctx, rpc.GetNotebook, getNotebookParams(notebookID), "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("list sources for notebook %s: %w", notebookID, err)
	}
	return parseSourceList(result, notebookID)
}

// Get fetches a single source by ID. NotebookLM has no single-source metadata
// RPC, so this filters the notebook's source list.
func (s *SourcesAPI) Get(ctx context.Context, notebookID, sourceID string) (*Source, error) {
	sources, err := s.List(ctx, notebookID)
	if err != nil {
		return nil, err
	}
	for i := range sources {
		if sources[i].ID == sourceID {
			return &sources[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s in notebook %s", ErrSourceNotFound, sourceID, notebookID)
}

// AddURL adds a web URL or YouTube video as a source.
func (s *SourcesAPI) AddURL(ctx context.Context, notebookID, rawURL string) (*Source, error) {
	params := addURLParams(notebookID, rawURL)
	if youtubeVideoID(rawURL) != "" {
		params = addYouTubeParams(notebookID, rawURL)
	}
	return s.add(ctx, notebookID, params, "URL")
}

// AddText adds pasted text content as a source.
func (s *SourcesAPI) AddText(ctx context.Context, notebookID, title, content string) (*Source, error) {
	return s.add(ctx, notebookID, addTextParams(notebookID, title, content), "text")
}

// AddDrive adds a Google Drive file as a source. fileRef is a Drive file ID or
// a Drive/Docs URL. The server re-derives the title on import, so a non-empty
// title is applied with a follow-up rename.
func (s *SourcesAPI) AddDrive(ctx context.Context, notebookID, fileRef, title, mimeType string) (*Source, error) {
	fileID := driveFileID(fileRef)
	if fileID == "" {
		return nil, fmt.Errorf("add drive source: no Drive file ID in %q", fileRef)
	}
	if mimeType == "" {
		mimeType = "application/vnd.google-apps.document"
	}
	src, err := s.add(ctx, notebookID, addDriveParams(notebookID, fileID, mimeType, title), "drive")
	if err != nil {
		return nil, err
	}
	if title != "" && src.Title != title {
		if renamed, err := s.Rename(ctx, notebookID, src.ID, title); err == nil {
			src = renamed
		}
	}
	return src, nil
}

func (s *SourcesAPI) add(ctx context.Context, notebookID string, params []any, kind string) (*Source, error) {
	result, err := s.client.RPCCall(ctx, rpc.AddSource, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("add %s source: %w", kind, err)
	}
	src, ok := parseSourceResult(result, notebookID)
	if !ok {
		return nil, fmt.Errorf("add %s source: unrecognized response shape", kind)
	}
	return &src, nil
}

// AddFile uploads a local file and adds it as a source.
//
// The file is registered first (which returns its source ID), then uploaded
// through a resumable upload session bound to that ID. The returned source is
// usually still processing; use WaitUntilReady to wait for it.
func (s *SourcesAPI) AddFile(ctx context.Context, notebookID, filePath string) (*Source, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}
	filename := filepath.Base(filePath)

	result, err := s.client.RPCCall(ctx, rpc.AddSourceFile, registerFileParams(notebookID, filename), "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("register file source: %w", err)
	}
	sourceID := registeredFileSourceID(result, filename)
	if sourceID == "" {
		return nil, fmt.Errorf("register file source: no source ID in response")
	}

	if err := s.uploadFile(ctx, notebookID, sourceID, filename, data); err != nil {
		return nil, fmt.Errorf("upload file (source %s registered but empty): %w", sourceID, err)
	}

	if src, err := s.Get(ctx, notebookID, sourceID); err == nil {
		return src, nil
	}
	return &Source{ID: sourceID, NotebookID: notebookID, Title: filename, Status: rpc.SourceProcessing}, nil
}

// Delete removes a source from a notebook. Deleting an absent source succeeds.
func (s *SourcesAPI) Delete(ctx context.Context, notebookID, sourceID string) error {
	params := []any{rpc.NestIDs([]string{sourceID}, 1)}
	if _, err := s.client.RPCCall(ctx, rpc.DeleteSource, params, "/notebook/"+notebookID, true); err != nil {
		return fmt.Errorf("delete source %s: %w", sourceID, err)
	}
	return nil
}

// Rename updates the title of a source.
func (s *SourcesAPI) Rename(ctx context.Context, notebookID, sourceID, newTitle string) (*Source, error) {
	params := []any{nil, []any{sourceID}, []any{[]any{[]any{newTitle}}}}
	result, err := s.client.rpcCallPolicy(ctx, rpc.UpdateSource, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected)
	if err != nil {
		return nil, fmt.Errorf("rename source %s: %w", sourceID, err)
	}
	if src, ok := parseSourceResult(result, notebookID); ok {
		return &src, nil
	}
	return s.Get(ctx, notebookID, sourceID)
}

// Refresh re-fetches content for a URL or Drive source.
func (s *SourcesAPI) Refresh(ctx context.Context, notebookID, sourceID string) error {
	params := []any{nil, []any{sourceID}, []any{2}}
	if _, err := s.client.rpcCallPolicy(ctx, rpc.RefreshSource, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected); err != nil {
		return fmt.Errorf("refresh source %s: %w", sourceID, err)
	}
	return nil
}

// WaitUntilReady polls a source until it's ready or an error occurs.
// Uses exponential backoff with a maximum interval.
func (s *SourcesAPI) WaitUntilReady(ctx context.Context, notebookID, sourceID string, timeout time.Duration) (*Source, error) {
	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second
	maxInterval := 15 * time.Second

	for {
		src, err := s.Get(ctx, notebookID, sourceID)
		if err != nil {
			return nil, err
		}
		if src.IsReady() {
			return src, nil
		}
		if src.IsError() {
			return nil, fmt.Errorf("source %s processing failed", sourceID)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for source %s to be ready", sourceID)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
		interval = time.Duration(math.Min(float64(interval*2), float64(maxInterval)))
	}
}

// GetGuide returns the AI-generated summary and keywords for a source.
func (s *SourcesAPI) GetGuide(ctx context.Context, notebookID, sourceID string) (*SourceGuide, error) {
	params := []any{[]any{[]any{[]any{sourceID}}}}
	result, err := s.client.RPCCall(ctx, rpc.GetSourceGuide, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("get source guide %s: %w", sourceID, err)
	}
	return parseSourceGuide(result), nil
}

// GetFulltext returns the full indexed text NotebookLM holds for a source.
func (s *SourcesAPI) GetFulltext(ctx context.Context, notebookID, sourceID string) (*SourceFulltext, error) {
	params := []any{[]any{sourceID}, []any{2}, []any{2}}
	result, err := s.client.RPCCall(ctx, rpc.GetSource, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("get source fulltext %s: %w", sourceID, err)
	}
	ft, ok := parseSourceFulltext(result, sourceID)
	if !ok {
		return nil, fmt.Errorf("%w: %s in notebook %s", ErrSourceNotFound, sourceID, notebookID)
	}
	return ft, nil
}

// CheckFreshness returns true if the source content is up to date, false if it
// needs a refresh.
func (s *SourcesAPI) CheckFreshness(ctx context.Context, notebookID, sourceID string) (bool, error) {
	params := []any{nil, []any{sourceID}, []any{2}}
	result, err := s.client.RPCCall(ctx, rpc.CheckSourceFreshness, params, "/notebook/"+notebookID, true)
	if err != nil {
		return false, fmt.Errorf("check source freshness %s: %w", sourceID, err)
	}
	fresh, ok := interpretFreshness(result)
	if !ok {
		return false, fmt.Errorf("check source freshness %s: unrecognized response shape", sourceID)
	}
	return fresh, nil
}

// DiscoverSources searches the web for sources related to query. mode is one
// of "default", "raw", "curious" or "curious_raw" (empty means "default"); the
// curious modes ignore query and let the backend pick the topic. Results can
// be added with AddURL.
func (s *SourcesAPI) DiscoverSources(ctx context.Context, notebookID, query, mode string) (*DiscoverResult, error) {
	if mode == "" {
		mode = "default"
	}
	code, ok := discoverModes[strings.ToLower(mode)]
	if !ok {
		return nil, fmt.Errorf("invalid discover mode %q (use default, raw, curious or curious_raw)", mode)
	}
	if code >= 3 {
		query = ""
	} else if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("discover sources: query must not be empty in %q mode", mode)
	}

	params := []any{[]any{query, 1}, nil, code, notebookID}
	result, err := s.client.RPCCall(ctx, rpc.DiscoverSources, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("discover sources: %w", err)
	}
	res, ok := parseDiscoverResult(result)
	if !ok {
		return nil, fmt.Errorf("discover sources: response has no job ID")
	}
	return res, nil
}

func (s *SourcesAPI) uploadFile(ctx context.Context, notebookID, sourceID, filename string, data []byte) error {
	s.client.mu.Lock()
	cookieHeader := s.client.auth.CookieHeader()
	s.client.mu.Unlock()

	origin := uploadOrigin(rpc.UploadURL)
	body, err := json.Marshal(map[string]string{
		"PROJECT_ID":  notebookID,
		"SOURCE_NAME": filename,
		"SOURCE_ID":   sourceID,
	})
	if err != nil {
		return err
	}

	// Start a resumable upload session bound to the registered source.
	startReq, err := http.NewRequestWithContext(ctx, "POST", rpc.UploadURL+"?authuser=0", bytes.NewReader(body))
	if err != nil {
		return err
	}
	startReq.Header.Set("Cookie", cookieHeader)
	startReq.Header.Set("Accept", "*/*")
	startReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	startReq.Header.Set("Origin", origin)
	startReq.Header.Set("Referer", origin+"/")
	startReq.Header.Set("X-Goog-Authuser", "0")
	startReq.Header.Set("X-Goog-Upload-Command", "start")
	startReq.Header.Set("X-Goog-Upload-Header-Content-Length", fmt.Sprintf("%d", len(data)))
	startReq.Header.Set("X-Goog-Upload-Header-Content-Type", uploadContentType(filename))
	startReq.Header.Set("X-Goog-Upload-Protocol", "resumable")

	resp, err := s.client.httpClient.Do(startReq)
	if err != nil {
		return fmt.Errorf("start upload: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("start upload: HTTP %d", resp.StatusCode)
	}
	sessionURL := resp.Header.Get("X-Goog-Upload-URL")
	if err := validateUploadURL(sessionURL); err != nil {
		return fmt.Errorf("start upload: %w", err)
	}

	// Upload the bytes and finalize in one request.
	uploadReq, err := http.NewRequestWithContext(ctx, "POST", sessionURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	uploadReq.Header.Set("Cookie", cookieHeader)
	uploadReq.Header.Set("Accept", "*/*")
	uploadReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	uploadReq.Header.Set("Origin", origin)
	uploadReq.Header.Set("Referer", origin+"/")
	uploadReq.Header.Set("X-Goog-Authuser", "0")
	uploadReq.Header.Set("X-Goog-Upload-Command", "upload, finalize")
	uploadReq.Header.Set("X-Goog-Upload-Offset", "0")

	uploadClient := *s.client.httpClient
	uploadClient.Timeout = uploadTimeout
	uploadResp, err := uploadClient.Do(uploadReq)
	if err != nil {
		return fmt.Errorf("upload data: %w", err)
	}
	defer uploadResp.Body.Close()
	io.Copy(io.Discard, uploadResp.Body)
	if uploadResp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload data: HTTP %d", uploadResp.StatusCode)
	}
	return nil
}

// ---- request payloads ------------------------------------------------------

func getNotebookParams(notebookID string) []any {
	return []any{notebookID, nil, rpc.TemplateBlock(), nil, 0}
}

// sourceSpec returns an 11-slot source spec with the trailing 1 marker that
// every add-source variant carries, and value placed at index slot.
func sourceSpec(slot int, value any) []any {
	spec := make([]any, 11)
	spec[slot] = value
	spec[10] = 1
	return spec
}

func addURLParams(notebookID, rawURL string) []any {
	return []any{[]any{sourceSpec(2, []any{rawURL})}, notebookID, rpc.TemplateBlock()}
}

func addYouTubeParams(notebookID, rawURL string) []any {
	return []any{[]any{sourceSpec(7, []any{rawURL})}, notebookID, rpc.TemplateBlock()}
}

func addTextParams(notebookID, title, content string) []any {
	spec := sourceSpec(1, []any{title, content})
	spec[3] = 2
	return []any{[]any{spec}, notebookID, rpc.TemplateBlock()}
}

// Drive adds have not been migrated to the nested template block upstream;
// they still send the older flat [2], [1, ..., [1]] tail.
func addDriveParams(notebookID, fileID, mimeType, title string) []any {
	spec := sourceSpec(0, []any{fileID, mimeType, 1, title})
	return []any{
		[]any{spec},
		notebookID,
		[]any{2},
		[]any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}},
	}
}

func registerFileParams(notebookID, filename string) []any {
	return []any{[]any{[]any{filename}}, notebookID, rpc.TemplateBlock()}
}

// ---- response parsing ------------------------------------------------------

// parseSourceList reads the sources from a GET_NOTEBOOK response, which are at
// result[0][1]. An empty notebook has a null sources slot.
func parseSourceList(result any, notebookID string) ([]Source, error) {
	arr, _ := result.([]any)
	if len(arr) == 0 {
		return nil, fmt.Errorf("list sources for notebook %s: unrecognized response shape", notebookID)
	}
	info, _ := arr[0].([]any)
	if len(info) < 2 {
		return nil, fmt.Errorf("list sources for notebook %s: unrecognized response shape", notebookID)
	}
	rows, _ := info[1].([]any)
	sources := make([]Source, 0, len(rows))
	for _, item := range rows {
		if entry, ok := item.([]any); ok && sourceEntryID(entry) != "" {
			sources = append(sources, sourceFromEntry(entry, notebookID, false))
		}
	}
	return sources, nil
}

// parseSourceResult decodes the single source returned by add / rename RPCs,
// which may arrive deeply nested ([[[[id], title, ...]]]), medium nested
// ([[[id], title, ...]]) or flat ([id, title, ...]).
func parseSourceResult(result any, notebookID string) (Source, bool) {
	data, _ := result.([]any)
	if len(data) == 0 {
		return Source{}, false
	}
	entry, allowBareHTTP := data, false
	if outer, ok := data[0].([]any); ok && len(outer) > 0 {
		if inner, ok := outer[0].([]any); ok && len(inner) > 0 {
			if _, deep := inner[0].([]any); deep {
				entry, allowBareHTTP = inner, true
			} else {
				entry = outer
			}
		}
	}
	src := sourceFromEntry(entry, notebookID, allowBareHTTP)
	return src, src.ID != ""
}

// sourceFromEntry decodes a normalized source row:
//
//	[0] id envelope: "id", ["id"], or [null, true, ["id"]] for Drive-backed rows
//	[1] title
//	[2] metadata: [2][0] creation seconds, [4] type code, [5][0] YouTube URL, [7][0] URL
//	[3] settings: [3][1] status code
//
// allowBareHTTP lets a bare http(s) URL at metadata[0] act as the URL; only the
// deeply nested add-source shape uses that slot this way.
func sourceFromEntry(entry []any, notebookID string, allowBareHTTP bool) Source {
	src := Source{NotebookID: notebookID, ID: sourceEntryID(entry), Status: rpc.SourceUnknown}
	if len(entry) > 1 {
		src.Title, _ = entry[1].(string)
	}
	if len(entry) > 2 {
		if meta, ok := entry[2].([]any); ok {
			src.URL = sourceURL(meta, allowBareHTTP)
			if code, ok := intAt(meta, 4); ok {
				src.TypeCode = code
			}
			if ts, ok := listAt(meta, 2); ok && len(ts) > 0 {
				if secs, ok := ts[0].(float64); ok {
					src.CreatedAt = time.Unix(int64(secs), 0).UTC()
				}
			}
		}
	}
	if len(entry) > 3 {
		if settings, ok := entry[3].([]any); ok {
			if code, ok := intAt(settings, 1); ok {
				switch st := rpc.SourceStatus(code); st {
				case rpc.SourceProcessing, rpc.SourceReady, rpc.SourceError, rpc.SourcePreparing:
					src.Status = st
				}
			}
		}
	}
	return src
}

func sourceEntryID(entry []any) string {
	if len(entry) == 0 {
		return ""
	}
	switch v := entry[0].(type) {
	case string:
		return v
	case []any:
		if len(v) > 0 {
			if id, ok := v[0].(string); ok {
				return id
			}
		}
		if len(v) == 3 && v[0] == nil {
			if inner, ok := v[2].([]any); ok && len(inner) > 0 {
				id, _ := inner[0].(string)
				return id
			}
		}
	}
	return ""
}

func sourceURL(meta []any, allowBareHTTP bool) string {
	for _, pos := range []int{7, 5} {
		if block, ok := listAt(meta, pos); ok && len(block) > 0 {
			if u, ok := block[0].(string); ok && u != "" {
				return u
			}
		}
	}
	if allowBareHTTP && len(meta) > 0 {
		if u, ok := meta[0].(string); ok && strings.HasPrefix(u, "http") {
			return u
		}
	}
	return ""
}

// registeredFileSourceID extracts the new source ID from an ADD_SOURCE_FILE
// response. Accepted shapes are a singleton envelope ([[id]], [[[[id]]]]), the
// same behind a null prefix ([null, [[id]]]), or a [id, filename] pair.
func registeredFileSourceID(result any, filename string) string {
	if id := singletonID(result, filename); id != "" {
		return id
	}
	if pair, ok := result.([]any); ok && len(pair) == 2 && pair[0] == nil {
		if id := singletonID(pair[1], filename); id != "" {
			return id
		}
	}
	var found []string
	var walk func(node any, depth int)
	walk = func(node any, depth int) {
		list, ok := node.([]any)
		if !ok || depth > 8 {
			return
		}
		if len(list) >= 2 {
			if name, _ := unwrapSingleton(list[1]).(string); strings.TrimSpace(name) == filename {
				if id := idCandidate(list[0], filename); id != "" {
					found = append(found, id)
				}
			}
		}
		for _, child := range list {
			walk(child, depth+1)
		}
	}
	walk(result, 0)
	if len(found) == 1 {
		return found[0]
	}
	return ""
}

func singletonID(v any, filename string) string {
	list, ok := v.([]any)
	if !ok || len(list) != 1 {
		return ""
	}
	return idCandidate(v, filename)
}

func unwrapSingleton(v any) any {
	for i := 0; i < 8; i++ {
		list, ok := v.([]any)
		if !ok || len(list) != 1 {
			break
		}
		v = list[0]
	}
	return v
}

// idCandidate accepts a UUID, or an ID-like token (no spaces or slashes, at
// least one digit, '-' or '_'), that is not the filename itself.
func idCandidate(v any, filename string) string {
	s, ok := unwrapSingleton(v).(string)
	if !ok || len(s) > 1000 {
		return ""
	}
	s = strings.TrimSpace(s)
	if s == "" || s == filename {
		return ""
	}
	if uuidPattern.MatchString(s) {
		return s
	}
	if len(s) < 4 || strings.ContainsAny(s, " \t/") || !strings.ContainsAny(s, "0123456789-_") {
		return ""
	}
	return s
}

// parseSourceGuide reads a GET_SOURCE_GUIDE response: summary at
// result[0][0][1][0], keywords at result[0][0][2][0].
func parseSourceGuide(result any) *SourceGuide {
	guide := &SourceGuide{Keywords: []string{}}
	outer, _ := result.([]any)
	if len(outer) == 0 {
		return guide
	}
	mid, _ := outer[0].([]any)
	if len(mid) == 0 {
		return guide
	}
	inner, _ := mid[0].([]any)
	if block, ok := listAt(inner, 1); ok && len(block) > 0 {
		guide.Summary, _ = block[0].(string)
	}
	if block, ok := listAt(inner, 2); ok && len(block) > 0 {
		if kws, ok := block[0].([]any); ok {
			for _, k := range kws {
				if s, ok := k.(string); ok {
					guide.Keywords = append(guide.Keywords, s)
				}
			}
		}
	}
	return guide
}

// parseSourceFulltext reads a GET_SOURCE response: [descriptor, ?, ?, [body], ...]
// where descriptor is a normalized source row and body is a nested text tree.
func parseSourceFulltext(result any, sourceID string) (*SourceFulltext, bool) {
	arr, _ := result.([]any)
	if len(arr) == 0 {
		return nil, false
	}
	ft := &SourceFulltext{SourceID: sourceID}
	if descriptor, ok := arr[0].([]any); ok {
		src := sourceFromEntry(descriptor, "", false)
		ft.Title, ft.URL, ft.TypeCode = src.Title, src.URL, src.TypeCode
	}
	if block, ok := listAt(arr, 3); ok && len(block) > 0 {
		if body, ok := block[0].([]any); ok {
			ft.Content = strings.Join(collectStrings(body, 100), "\n")
		}
	}
	return ft, true
}

func collectStrings(v []any, depth int) []string {
	if depth <= 0 {
		return nil
	}
	var out []string
	for _, item := range v {
		switch x := item.(type) {
		case string:
			if x != "" {
				out = append(out, x)
			}
		case []any:
			out = append(out, collectStrings(x, depth-1)...)
		}
	}
	return out
}

// interpretFreshness decodes CHECK_SOURCE_FRESHNESS: true / [] / [[null, true, ...]]
// mean fresh, false / [[null, false, ...]] mean stale; anything else is unrecognized.
func interpretFreshness(result any) (fresh, ok bool) {
	switch v := result.(type) {
	case bool:
		return v, true
	case []any:
		if len(v) == 0 {
			return true, true
		}
		if first, isList := v[0].([]any); isList && len(first) > 1 {
			if b, isBool := first[1].(bool); isBool {
				return b, true
			}
		}
	}
	return false, false
}

// parseDiscoverResult reads a DISCOVER_SOURCES response:
// [sources, overview, [job_id], ?, error_code]. Each source row is [url, title, ...].
func parseDiscoverResult(result any) (*DiscoverResult, bool) {
	arr, _ := result.([]any)
	key, _ := listAt(arr, 2)
	if len(key) == 0 {
		return nil, false
	}
	taskID, _ := key[0].(string)
	if taskID == "" {
		return nil, false
	}
	res := &DiscoverResult{TaskID: taskID, Sources: []DiscoveredSource{}}
	if rows, ok := listAt(arr, 0); ok {
		for _, r := range rows {
			row, _ := r.([]any)
			if len(row) == 0 {
				continue
			}
			u, _ := row[0].(string)
			if u == "" {
				continue
			}
			ds := DiscoveredSource{URL: u}
			if len(row) > 1 {
				ds.Title, _ = row[1].(string)
			}
			res.Sources = append(res.Sources, ds)
		}
	}
	if len(arr) > 1 {
		res.Summary, _ = arr[1].(string)
	}
	if code, ok := intAt(arr, 4); ok && code != 0 {
		res.Failed = true
	}
	return res, true
}

// ---- helpers ---------------------------------------------------------------

func listAt(v []any, i int) ([]any, bool) {
	if i < 0 || i >= len(v) {
		return nil, false
	}
	l, ok := v[i].([]any)
	return l, ok
}

// intAt returns v[i] when it is a JSON integer.
func intAt(v []any, i int) (int, bool) {
	if i < 0 || i >= len(v) {
		return 0, false
	}
	f, ok := v[i].(float64)
	if !ok || f != math.Trunc(f) {
		return 0, false
	}
	return int(f), true
}

func youtubeVideoID(rawURL string) string {
	if m := youtubeVideoIDPattern.FindStringSubmatch(rawURL); m != nil {
		return m[1]
	}
	return ""
}

var driveIDPattern = regexp.MustCompile(`/d/([a-zA-Z0-9_-]+)`)

// driveFileID accepts a bare Drive file ID or a Drive / Docs URL
// (.../d/<id>/..., or ...?id=<id>).
func driveFileID(ref string) string {
	ref = strings.TrimSpace(ref)
	if !strings.Contains(ref, "/") {
		return ref
	}
	if m := driveIDPattern.FindStringSubmatch(ref); m != nil {
		return m[1]
	}
	if u, err := url.Parse(ref); err == nil {
		return u.Query().Get("id")
	}
	return ""
}

func uploadContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".md", ".markdown":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	}
	if t := mime.TypeByExtension(filepath.Ext(name)); t != "" {
		return t
	}
	return "application/octet-stream"
}

func uploadOrigin(uploadURL string) string {
	u, err := url.Parse(uploadURL)
	if err != nil {
		return "https://notebooklm.google.com"
	}
	return u.Scheme + "://" + u.Host
}

// validateUploadURL checks the server-provided upload session URL before file
// bytes and cookies are sent to it.
func validateUploadURL(raw string) error {
	if raw == "" {
		return errors.New("no upload session URL returned")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid upload session URL: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme != "https" || (host != "google.com" && !strings.HasSuffix(host, ".google.com")) {
		return fmt.Errorf("refusing upload session URL on unexpected host %q", u.Host)
	}
	return nil
}
