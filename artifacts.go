package notebooklm

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/saeedata/notebooklm-go/rpc"
)

// ArtifactsAPI provides artifact generation, retrieval, and download operations.
type ArtifactsAPI struct {
	client *Client
}

// List returns all artifacts in a notebook.
func (a *ArtifactsAPI) List(ctx context.Context, notebookID string) ([]*Artifact, error) {
	params := []any{[]any{2}, notebookID, `NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"`}
	result, err := a.client.rpcCallPolicy(ctx, rpc.ListArtifacts, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected)
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}

	arr, _ := result.([]any)
	raw := unwrapRows(arr)
	artifacts := make([]*Artifact, 0, len(raw))
	for _, item := range raw {
		if data, ok := item.([]any); ok {
			if art := ArtifactFromAPIResponse(data); art != nil {
				artifacts = append(artifacts, art)
			}
		}
	}
	return artifacts, nil
}

// Get returns a single artifact by ID.
func (a *ArtifactsAPI) Get(ctx context.Context, notebookID, artifactID string) (*Artifact, error) {
	all, err := a.List(ctx, notebookID)
	if err != nil {
		return nil, err
	}
	for _, art := range all {
		if art.ID == artifactID {
			return art, nil
		}
	}
	return nil, fmt.Errorf("artifact %s not found in notebook %s", artifactID, notebookID)
}

// Delete removes an artifact.
func (a *ArtifactsAPI) Delete(ctx context.Context, notebookID, artifactID string) error {
	params := []any{[]any{2}, artifactID}
	if _, err := a.client.RPCCall(ctx, rpc.DeleteArtifact, params, "/notebook/"+notebookID, true); err != nil {
		return fmt.Errorf("delete artifact %s: %w", artifactID, err)
	}
	return nil
}

// Rename updates the title of an artifact.
func (a *ArtifactsAPI) Rename(ctx context.Context, notebookID, artifactID, newTitle string) error {
	params := []any{[]any{artifactID, newTitle}, []any{[]any{"title"}}}
	if _, err := a.client.rpcCallPolicy(ctx, rpc.RenameArtifact, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected); err != nil {
		return fmt.Errorf("rename artifact %s: %w", artifactID, err)
	}
	return nil
}

// GenerateAudio creates an audio overview.
func (a *ArtifactsAPI) GenerateAudio(ctx context.Context, notebookID string, opts GenerateAudioOptions) (*GenerationStatus, error) {
	ids, err := a.sourceIDsOrAll(ctx, notebookID, opts.SourceIDs)
	if err != nil {
		return nil, err
	}
	return a.createArtifact(ctx, notebookID, audioParams(notebookID, ids, opts))
}

func audioParams(notebookID string, ids []string, opts GenerateAudioOptions) []any {
	config := []any{nil, []any{
		nilIfEmpty(opts.Instructions),
		orDefault(int(opts.Length), int(rpc.AudioDefault)),
		nil,
		rpc.NestIDs(ids, 1),
		languageOrDefault(opts.Language),
		nil,
		orDefault(int(opts.Format), int(rpc.AudioDeepDive)),
	}}
	return artifactParams(notebookID, rpc.ArtifactTypeAudio, ids, 6, config)
}

// GenerateVideo creates a video overview.
func (a *ArtifactsAPI) GenerateVideo(ctx context.Context, notebookID string, opts GenerateVideoOptions) (*GenerationStatus, error) {
	ids, err := a.sourceIDsOrAll(ctx, notebookID, opts.SourceIDs)
	if err != nil {
		return nil, err
	}
	return a.createArtifact(ctx, notebookID, videoParams(notebookID, ids, opts))
}

func videoParams(notebookID string, ids []string, opts GenerateVideoOptions) []any {
	format := orDefault(int(opts.Format), int(rpc.VideoExplainer))
	video := []any{rpc.NestIDs(ids, 1), languageOrDefault(opts.Language), nilIfEmpty(opts.Instructions), nil, format}
	if format != int(rpc.VideoCinematic) {
		if opts.StylePrompt != "" {
			video = append(video, nil, opts.StylePrompt) // custom style
		} else {
			video = append(video, orDefault(int(opts.Style), int(rpc.VideoStyleAutoSelect)))
		}
	}
	return artifactParams(notebookID, rpc.ArtifactTypeVideo, ids, 8, []any{nil, nil, video})
}

// reportPresets holds the title, description and directive sent for each
// built-in report format.
var reportPresets = map[rpc.ReportFormat][3]string{
	rpc.ReportBriefingDoc: {"Briefing Doc", "Key insights and important quotes",
		"Create a comprehensive briefing document that includes an Executive Summary, detailed analysis of key themes, important quotes with context, and actionable insights."},
	rpc.ReportStudyGuide: {"Study Guide", "Short-answer quiz, essay questions, glossary",
		"Create a comprehensive study guide that includes key concepts, short-answer practice questions, essay prompts for deeper exploration, and a glossary of important terms."},
	rpc.ReportBlogPost: {"Blog Post", "Insightful takeaways in readable article format",
		"Write an engaging blog post that presents the key insights in an accessible, reader-friendly format. Include an attention-grabbing introduction, well-organized sections, and a compelling conclusion with takeaways."},
}

// GenerateReport creates a report artifact.
func (a *ArtifactsAPI) GenerateReport(ctx context.Context, notebookID string, opts GenerateReportOptions) (*GenerationStatus, error) {
	ids, err := a.sourceIDsOrAll(ctx, notebookID, opts.SourceIDs)
	if err != nil {
		return nil, err
	}
	params, err := reportParams(notebookID, ids, opts)
	if err != nil {
		return nil, err
	}
	return a.createArtifact(ctx, notebookID, params)
}

func reportParams(notebookID string, ids []string, opts GenerateReportOptions) ([]any, error) {
	format := opts.Format
	if format == "" {
		format = rpc.ReportBriefingDoc
	}
	var title, description, directive string
	if format == rpc.ReportCustom {
		title, description, directive = "Custom Report", "Custom format", opts.CustomPrompt
		if directive == "" {
			directive = "Create a report based on the provided sources."
		}
	} else {
		preset, ok := reportPresets[format]
		if !ok {
			return nil, fmt.Errorf("unsupported report format %q (use briefing_doc, study_guide, blog_post or custom)", format)
		}
		title, description, directive = preset[0], preset[1], preset[2]
		if opts.ExtraInstructions != "" {
			directive += "\n\n" + opts.ExtraInstructions
		}
	}
	config := []any{nil, []any{title, description, nil, rpc.NestIDs(ids, 1), languageOrDefault(opts.Language), directive, nil, true}}
	return artifactParams(notebookID, rpc.ArtifactTypeReport, ids, 7, config), nil
}

// GenerateQuiz creates a quiz, or flashcards when opts.Flashcards is set.
func (a *ArtifactsAPI) GenerateQuiz(ctx context.Context, notebookID string, opts GenerateQuizOptions) (*GenerationStatus, error) {
	ids, err := a.sourceIDsOrAll(ctx, notebookID, opts.SourceIDs)
	if err != nil {
		return nil, err
	}
	return a.createArtifact(ctx, notebookID, quizParams(notebookID, ids, opts))
}

func quizParams(notebookID string, ids []string, opts GenerateQuizOptions) []any {
	options := []any{orDefault(int(opts.Quantity), int(rpc.QuizStandard)), orDefault(int(opts.Difficulty), int(rpc.QuizMedium))}
	inner := []any{rpc.ArtifactQuizVariant, nil, nilIfEmpty(opts.Instructions), nil, nil, nil, nil, options}
	if opts.Flashcards {
		inner = []any{rpc.ArtifactFlashcardsVariant, nil, nilIfEmpty(opts.Instructions), nil, nil, nil, options}
	}
	return artifactParams(notebookID, rpc.ArtifactTypeQuiz, ids, 9, []any{nil, inner})
}

// GenerateInfographic creates an infographic artifact.
func (a *ArtifactsAPI) GenerateInfographic(ctx context.Context, notebookID string, opts GenerateInfographicOptions) (*GenerationStatus, error) {
	ids, err := a.sourceIDsOrAll(ctx, notebookID, opts.SourceIDs)
	if err != nil {
		return nil, err
	}
	return a.createArtifact(ctx, notebookID, infographicParams(notebookID, ids, opts))
}

func infographicParams(notebookID string, ids []string, opts GenerateInfographicOptions) []any {
	config := []any{[]any{
		nilIfEmpty(opts.Instructions),
		languageOrDefault(opts.Language),
		nil,
		orDefault(int(opts.Orientation), int(rpc.InfographicLandscape)),
		orDefault(int(opts.DetailLevel), int(rpc.InfographicStandard)),
		orDefault(int(opts.Style), int(rpc.InfographicStyleAutoSelect)),
	}}
	return artifactParams(notebookID, rpc.ArtifactTypeInfographic, ids, 14, config)
}

// GenerateSlideDeck creates a slide deck artifact.
func (a *ArtifactsAPI) GenerateSlideDeck(ctx context.Context, notebookID string, opts GenerateSlideDeckOptions) (*GenerationStatus, error) {
	ids, err := a.sourceIDsOrAll(ctx, notebookID, opts.SourceIDs)
	if err != nil {
		return nil, err
	}
	return a.createArtifact(ctx, notebookID, slideDeckParams(notebookID, ids, opts))
}

func slideDeckParams(notebookID string, ids []string, opts GenerateSlideDeckOptions) []any {
	config := []any{[]any{
		nilIfEmpty(opts.Instructions),
		languageOrDefault(opts.Language),
		orDefault(int(opts.Format), int(rpc.SlideDeckDetailed)),
		orDefault(int(opts.Length), int(rpc.SlideDeckDefault)),
	}}
	return artifactParams(notebookID, rpc.ArtifactTypeSlideDeck, ids, 16, config)
}

// GenerateDataTable creates a data table artifact.
func (a *ArtifactsAPI) GenerateDataTable(ctx context.Context, notebookID string, opts GenerateDataTableOptions) (*GenerationStatus, error) {
	ids, err := a.sourceIDsOrAll(ctx, notebookID, opts.SourceIDs)
	if err != nil {
		return nil, err
	}
	return a.createArtifact(ctx, notebookID, dataTableParams(notebookID, ids, opts))
}

func dataTableParams(notebookID string, ids []string, opts GenerateDataTableOptions) []any {
	config := []any{nil, []any{nilIfEmpty(opts.Instructions), languageOrDefault(opts.Language)}}
	return artifactParams(notebookID, rpc.ArtifactTypeDataTable, ids, 18, config)
}

// GenerateInteractiveMindMap creates an interactive mind map Studio artifact.
// For the classic mind map stored as a note, use NotesAPI.GenerateMindMap.
func (a *ArtifactsAPI) GenerateInteractiveMindMap(ctx context.Context, notebookID string, sourceIDs []string, instructions string) (*GenerationStatus, error) {
	ids, err := a.sourceIDsOrAll(ctx, notebookID, sourceIDs)
	if err != nil {
		return nil, err
	}
	return a.createArtifact(ctx, notebookID, interactiveMindMapParams(notebookID, ids, instructions))
}

func interactiveMindMapParams(notebookID string, ids []string, instructions string) []any {
	inner := []any{rpc.ArtifactInteractiveMindMapVariant}
	if instructions != "" {
		inner = []any{rpc.ArtifactInteractiveMindMapVariant, nil, instructions}
	}
	return artifactParams(notebookID, rpc.ArtifactTypeQuiz, ids, 9, []any{nil, inner})
}

// WaitForCompletion polls until an artifact generation task finishes. A media
// artifact counts as finished once its download URL is also available.
func (a *ArtifactsAPI) WaitForCompletion(ctx context.Context, notebookID, taskID string, timeout time.Duration) (*GenerationStatus, error) {
	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second
	maxInterval := 10 * time.Second

	for {
		art, err := a.Get(ctx, notebookID, taskID)
		if err != nil {
			return nil, err
		}

		status := &GenerationStatus{
			TaskID: taskID,
			Status: rpc.ArtifactStatusToStr(art.Status),
			URL:    art.URL,
		}

		if art.IsCompleted() && art.IsMediaReady() {
			return status, nil
		}
		if art.IsFailed() {
			return status, fmt.Errorf("artifact generation failed")
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for artifact %s completion", taskID)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
		interval = time.Duration(math.Min(float64(interval*2), float64(maxInterval)))
	}
}

// Download saves a completed artifact to destPath. Media artifacts (audio,
// video, infographic, slide deck as PDF) are fetched from their URL; reports
// are written as Markdown, data tables as CSV, and quizzes / flashcards /
// interactive mind maps as their HTML page.
func (a *ArtifactsAPI) Download(ctx context.Context, notebookID, artifactID, destPath string) error {
	art, err := a.Get(ctx, notebookID, artifactID)
	if err != nil {
		return err
	}
	if !art.IsCompleted() {
		return fmt.Errorf("artifact %s is not ready (status: %s)", artifactID, rpc.ArtifactStatusToStr(art.Status))
	}

	var data []byte
	switch art.TypeCode {
	case rpc.ArtifactTypeReport:
		md := reportMarkdown(art.row)
		if md == "" {
			return fmt.Errorf("artifact %s has no report content", artifactID)
		}
		data = []byte(md)
	case rpc.ArtifactTypeDataTable:
		csvData, err := dataTableCSV(art.row)
		if err != nil {
			return fmt.Errorf("artifact %s: %w", artifactID, err)
		}
		data = csvData
	case rpc.ArtifactTypeQuiz:
		html, err := a.GetInteractiveHTML(ctx, notebookID, artifactID)
		if err != nil {
			return err
		}
		if html == "" {
			return fmt.Errorf("artifact %s has no content", artifactID)
		}
		data = []byte(html)
	default:
		if art.URL == "" {
			return fmt.Errorf("artifact %s has no download URL", artifactID)
		}
		data, err = a.client.Download(ctx, art.URL)
		if err != nil {
			return fmt.Errorf("download artifact: %w", err)
		}
	}
	return os.WriteFile(destPath, data, 0644)
}

// GetSuggestedReports returns AI-suggested report formats for a notebook.
func (a *ArtifactsAPI) GetSuggestedReports(ctx context.Context, notebookID string) ([]ReportSuggestion, error) {
	params := []any{[]any{2}, notebookID}
	result, err := a.client.RPCCall(ctx, rpc.GetSuggestedReports, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("get suggested reports: %w", err)
	}
	return parseReportSuggestions(result), nil
}

// parseReportSuggestions reads [[row, ...]] or [row, ...], each row being
// [title, description, ?, ?, prompt, audience_level, ...].
func parseReportSuggestions(result any) []ReportSuggestion {
	arr, _ := result.([]any)
	rows := unwrapRows(arr)
	suggestions := make([]ReportSuggestion, 0, len(rows))
	for _, r := range rows {
		row, _ := r.([]any)
		if len(row) < 5 {
			continue
		}
		s := ReportSuggestion{}
		s.Title, _ = row[0].(string)
		s.Description, _ = row[1].(string)
		s.Prompt, _ = row[4].(string)
		if len(row) > 5 {
			switch v := row[5].(type) {
			case string:
				s.AudienceLevel = v
			case float64:
				s.AudienceLevel = fmt.Sprint(int(v))
			}
		}
		suggestions = append(suggestions, s)
	}
	return suggestions
}

// ExportArtifact exports an artifact to Google Docs or Sheets and returns the
// URL of the created document, if the response carries one.
func (a *ArtifactsAPI) ExportArtifact(ctx context.Context, notebookID, artifactID, title string, exportType rpc.ExportType) (string, error) {
	if title == "" {
		title = "Export"
	}
	params := []any{nil, artifactID, nil, title, int(exportType)}
	result, err := a.client.rpcCallPolicy(ctx, rpc.ExportArtifact, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected)
	if err != nil {
		return "", fmt.Errorf("export artifact: %w", err)
	}
	return firstURL(result, 8), nil
}

// ShareArtifact makes a notebook's artifacts publicly viewable by link and
// returns the artifact's share URL.
func (a *ArtifactsAPI) ShareArtifact(ctx context.Context, notebookID, artifactID string) (string, error) {
	params := []any{[]any{1}, notebookID, artifactID}
	if _, err := a.client.RPCCall(ctx, rpc.ShareArtifact, params, "/notebook/"+notebookID, true); err != nil {
		return "", fmt.Errorf("share artifact: %w", err)
	}
	return "https://notebooklm.google.com/notebook/" + notebookID + "?artifactId=" + url.QueryEscape(artifactID), nil
}

// GetInteractiveHTML returns the HTML page of a quiz, flashcards or
// interactive mind map artifact (result[0][9][0]).
func (a *ArtifactsAPI) GetInteractiveHTML(ctx context.Context, notebookID, artifactID string) (string, error) {
	result, err := a.client.RPCCall(ctx, rpc.GetInteractiveHTML, []any{artifactID}, "/notebook/"+notebookID, true)
	if err != nil {
		return "", fmt.Errorf("get interactive HTML: %w", err)
	}
	arr, _ := result.([]any)
	first, _ := listAt(arr, 0)
	content, _ := listAt(first, 9)
	if len(content) > 0 {
		html, _ := content[0].(string)
		return html, nil
	}
	return "", nil
}

// ReviseSlide revises one slide (0-based slideIndex) of a slide deck using
// natural language instructions. The revision runs as a new generation task.
func (a *ArtifactsAPI) ReviseSlide(ctx context.Context, notebookID, artifactID string, slideIndex int, instructions string) (*GenerationStatus, error) {
	params := []any{[]any{2}, artifactID, []any{[]any{[]any{slideIndex, instructions}}}}
	result, err := a.client.rpcCallPolicy(ctx, rpc.ReviseSlide, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected)
	if err != nil {
		return nil, fmt.Errorf("revise slide: %w", err)
	}
	return parseGenerationResult(result, "slide revision")
}

func (a *ArtifactsAPI) createArtifact(ctx context.Context, notebookID string, params []any) (*GenerationStatus, error) {
	result, err := a.client.rpcCallPolicy(ctx, rpc.CreateArtifact, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected)
	if err != nil {
		return nil, fmt.Errorf("create artifact: %w", err)
	}
	return parseGenerationResult(result, "artifact generation")
}

// parseGenerationResult reads the new artifact ID at result[0][0] and its
// status at result[0][4]. A null result means the feature is unavailable.
func parseGenerationResult(result any, what string) (*GenerationStatus, error) {
	if result == nil {
		return nil, fmt.Errorf("%s is unavailable for this notebook or account", what)
	}
	arr, _ := result.([]any)
	first, _ := listAt(arr, 0)
	if len(first) == 0 {
		return nil, fmt.Errorf("%s: unrecognized response shape", what)
	}
	id, _ := first[0].(string)
	if id == "" {
		return nil, fmt.Errorf("%s is unavailable for this notebook or account", what)
	}
	status := rpc.ArtifactPending
	if code, ok := intAt(first, 4); ok {
		status = rpc.ArtifactStatus(code)
	}
	return &GenerationStatus{TaskID: id, Status: rpc.ArtifactStatusToStr(status)}, nil
}

// sourceIDsOrAll returns ids, or every source in the notebook when ids is empty.
func (a *ArtifactsAPI) sourceIDsOrAll(ctx context.Context, notebookID string, ids []string) ([]string, error) {
	if len(ids) > 0 {
		return ids, nil
	}
	sources, err := a.client.Sources.List(ctx, notebookID)
	if err != nil {
		return nil, err
	}
	all := make([]string, 0, len(sources))
	for _, s := range sources {
		all = append(all, s.ID)
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("notebook %s has no sources", notebookID)
	}
	return all, nil
}

// artifactClientOptions is the request-context block sent as CREATE_ARTIFACT
// param 0.
func artifactClientOptions() []any {
	return []any{2, nil, nil, []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}}, []any{[]any{1, 4, 8, 2, 3, 6}}}
}

// artifactParams builds [client_options, notebook_id, descriptor] where the
// descriptor is [null, null, type, [[[id]], ...], null..., config] with the
// type-specific config at index slot.
func artifactParams(notebookID string, typeCode rpc.ArtifactTypeCode, ids []string, slot int, config any) []any {
	descriptor := make([]any, slot+1)
	descriptor[2] = int(typeCode)
	descriptor[3] = rpc.NestIDs(ids, 2)
	descriptor[slot] = config
	return []any{artifactClientOptions(), notebookID, descriptor}
}

// unwrapRows returns the rows of a list response that is either [[row, ...]]
// or already [row, ...].
func unwrapRows(result []any) []any {
	if len(result) == 1 {
		if inner, ok := result[0].([]any); ok {
			if len(inner) == 0 {
				return inner
			}
			if _, isRow := inner[0].([]any); isRow {
				return inner
			}
		}
	}
	return result
}

func orDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func languageOrDefault(lang string) string {
	if lang == "" {
		return "en"
	}
	return lang
}

// reportMarkdown reads a report's markdown at row[7] (a string, or [markdown, ...]).
func reportMarkdown(row []any) string {
	if len(row) <= 7 {
		return ""
	}
	switch v := row[7].(type) {
	case string:
		return v
	case []any:
		if len(v) > 0 {
			s, _ := v[0].(string)
			return s
		}
	}
	return ""
}

// dataTableCSV renders the data table at row[18]. Rows are at
// [0][0][0][0][4][2], each [start, end, [cell, ...]]; the first row is the header.
func dataTableCSV(row []any) ([]byte, error) {
	var node any
	if len(row) > 18 {
		node = row[18]
	}
	for _, i := range []int{0, 0, 0, 0, 4, 2} {
		list, _ := node.([]any)
		if i >= len(list) {
			return nil, fmt.Errorf("no data table content")
		}
		node = list[i]
	}
	rows, _ := node.([]any)
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	written := 0
	for _, r := range rows {
		section, _ := r.([]any)
		cells, ok := listAt(section, 2)
		if !ok {
			continue
		}
		record := make([]string, len(cells))
		for i, c := range cells {
			record[i] = cellText(c)
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
		written++
	}
	w.Flush()
	if written == 0 {
		return nil, fmt.Errorf("empty data table")
	}
	return buf.Bytes(), w.Error()
}

// cellText concatenates the strings in a nested cell, skipping position markers.
func cellText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		var sb strings.Builder
		for _, item := range x {
			sb.WriteString(cellText(item))
		}
		return sb.String()
	}
	return ""
}

func firstURL(v any, depth int) string {
	if depth <= 0 {
		return ""
	}
	switch x := v.(type) {
	case string:
		if strings.HasPrefix(x, "https://") {
			return x
		}
	case []any:
		for _, item := range x {
			if u := firstURL(item, depth-1); u != "" {
				return u
			}
		}
	}
	return ""
}
