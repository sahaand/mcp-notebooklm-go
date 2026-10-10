// Package notebooklm provides a Go client for the unofficial NotebookLM API.
// It reverse-engineers Google's internal batchexecute protocol to provide
// programmatic access to NotebookLM features.
package notebooklm

import (
	"strings"
	"time"

	"github.com/saeedata/notebooklm-go/rpc"
)

// SourceType represents the type of a source added to a notebook.
type SourceType string

const (
	SourceTypeGoogleDocs         SourceType = "GOOGLE_DOCS"
	SourceTypeGoogleSlides       SourceType = "GOOGLE_SLIDES"
	SourceTypeGoogleSpreadsheet  SourceType = "GOOGLE_SPREADSHEET"
	SourceTypeGoogleDrive        SourceType = "GOOGLE_DRIVE"
	SourceTypePDF                SourceType = "PDF"
	SourceTypePastedText         SourceType = "PASTED_TEXT"
	SourceTypeWebPage            SourceType = "WEB_PAGE"
	SourceTypeGoogleDriveAudio   SourceType = "GOOGLE_DRIVE_AUDIO"
	SourceTypeGoogleDriveVideo   SourceType = "GOOGLE_DRIVE_VIDEO"
	SourceTypeYouTube            SourceType = "YOUTUBE"
	SourceTypeMarkdown           SourceType = "MARKDOWN"
	SourceTypeDocx               SourceType = "DOCX"
	SourceTypePowerPoint         SourceType = "POWERPOINT"
	SourceTypeCSV                SourceType = "CSV"
	SourceTypeEPUB               SourceType = "EPUB"
	SourceTypeExcel              SourceType = "EXCEL"
	SourceTypeGmail              SourceType = "GMAIL"
	SourceTypeGeminiChat         SourceType = "GEMINI_CHAT"
	SourceTypeAIModeChat         SourceType = "AI_MODE_CHAT"
	SourceTypeExpertIntelligence SourceType = "EXPERT_INTELLIGENCE"
	SourceTypeImage              SourceType = "IMAGE"
	SourceTypeMedia              SourceType = "MEDIA"
	SourceTypeUnknown            SourceType = "UNKNOWN"
)

// sourceTypeCodes maps the type code at source metadata[4] to a SourceType.
// Code 14 is overloaded server-side (native Sheet vs Drive-hosted PDF); it is
// reported as GOOGLE_DRIVE here.
var sourceTypeCodes = map[int]SourceType{
	1:  SourceTypeGoogleDocs,
	2:  SourceTypeGoogleSlides,
	3:  SourceTypePDF,
	4:  SourceTypePastedText,
	5:  SourceTypeWebPage,
	6:  SourceTypePowerPoint,
	7:  SourceTypeGoogleSpreadsheet,
	8:  SourceTypeMarkdown,
	9:  SourceTypeYouTube,
	10: SourceTypeMedia,
	11: SourceTypeDocx,
	12: SourceTypeExcel,
	13: SourceTypeImage,
	14: SourceTypeGoogleDrive,
	15: SourceTypeGmail,
	16: SourceTypeCSV,
	17: SourceTypeEPUB,
	18: SourceTypeGeminiChat,
	19: SourceTypeAIModeChat,
	20: SourceTypeExpertIntelligence,
}

// ArtifactKind represents the high-level type of an artifact.
type ArtifactKind string

const (
	ArtifactKindAudio       ArtifactKind = "AUDIO"
	ArtifactKindVideo       ArtifactKind = "VIDEO"
	ArtifactKindReport      ArtifactKind = "REPORT"
	ArtifactKindQuiz        ArtifactKind = "QUIZ"
	ArtifactKindFlashcards  ArtifactKind = "FLASHCARDS"
	ArtifactKindMindMap     ArtifactKind = "MIND_MAP"
	ArtifactKindInfographic ArtifactKind = "INFOGRAPHIC"
	ArtifactKindSlideDeck   ArtifactKind = "SLIDE_DECK"
	ArtifactKindDataTable   ArtifactKind = "DATA_TABLE"
	ArtifactKindUnknown     ArtifactKind = "UNKNOWN"
)

// ChatMode represents predefined chat configurations.
type ChatMode string

const (
	ChatModeDefault       ChatMode = "DEFAULT"
	ChatModeLearningGuide ChatMode = "LEARNING_GUIDE"
	ChatModeConcise       ChatMode = "CONCISE"
	ChatModeDetailed      ChatMode = "DETAILED"
)

// Notebook represents a NotebookLM notebook.
type Notebook struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	SourcesCount int       `json:"sources_count"`
	IsOwner      bool      `json:"is_owner"`
}

// NotebookFromAPIResponse parses a notebook from the raw API response array.
// Response layout: [0]=title, [1]=sources[], [2]=id, ...
func NotebookFromAPIResponse(data []any) Notebook {
	nb := Notebook{}
	if len(data) == 0 {
		return nb
	}
	if title, ok := data[0].(string); ok {
		nb.Title = title
	}
	if len(data) > 2 {
		if id, ok := data[2].(string); ok {
			nb.ID = id
		}
	}
	return nb
}

// SuggestedTopic represents an AI-suggested conversation topic.
type SuggestedTopic struct {
	Question string `json:"question"`
	Prompt   string `json:"prompt"`
}

// NotebookDescription represents an AI-generated summary with suggested topics.
type NotebookDescription struct {
	Summary         string           `json:"summary"`
	SuggestedTopics []SuggestedTopic `json:"suggested_topics"`
}

// SourceSummary is a lightweight source representation for metadata export.
type SourceSummary struct {
	Kind  SourceType `json:"kind"`
	Title string     `json:"title"`
	URL   string     `json:"url,omitempty"`
}

// NotebookMetadata combines notebook details with simplified source list.
type NotebookMetadata struct {
	Notebook Notebook        `json:"notebook"`
	Sources  []SourceSummary `json:"sources"`
}

// Source represents a source added to a notebook.
type Source struct {
	ID         string           `json:"id"`
	NotebookID string           `json:"notebook_id"`
	Title      string           `json:"title"`
	URL        string           `json:"url,omitempty"`
	TypeCode   int              `json:"type_code"`
	CreatedAt  time.Time        `json:"created_at"`
	Status     rpc.SourceStatus `json:"status"`
}

// SourceGuide is the AI-generated summary and keywords for a source.
type SourceGuide struct {
	Summary  string   `json:"summary"`
	Keywords []string `json:"keywords"`
}

// SourceFulltext is the full indexed text NotebookLM holds for a source.
type SourceFulltext struct {
	SourceID string `json:"source_id"`
	Title    string `json:"title"`
	URL      string `json:"url,omitempty"`
	TypeCode int    `json:"type_code"`
	Content  string `json:"content"`
}

// DiscoveredSource is one web result from a source-discovery search.
type DiscoveredSource struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// DiscoverResult is the outcome of a source-discovery search.
type DiscoverResult struct {
	TaskID  string             `json:"task_id"`
	Summary string             `json:"summary"`
	Sources []DiscoveredSource `json:"sources"`
	// Failed is set when the server reported an in-band error (no results,
	// query too long, every result already in the notebook, ...).
	Failed bool `json:"failed"`
}

// Kind returns the SourceType enum for this source.
func (s *Source) Kind() SourceType {
	if t, ok := sourceTypeCodes[s.TypeCode]; ok {
		return t
	}
	return SourceTypeUnknown
}

// IsReady returns true if the source has finished processing.
func (s *Source) IsReady() bool { return s.Status == rpc.SourceReady }

// IsProcessing returns true if the source is still being processed.
func (s *Source) IsProcessing() bool {
	return s.Status == rpc.SourceProcessing || s.Status == rpc.SourcePreparing
}

// IsError returns true if source processing failed.
func (s *Source) IsError() bool { return s.Status == rpc.SourceError }

// SourceFromAPIResponse parses a source from the raw API response.
func SourceFromAPIResponse(data []any, notebookID string) Source {
	src := Source{NotebookID: notebookID}
	if len(data) == 0 {
		return src
	}
	if id, ok := data[0].(string); ok {
		src.ID = id
	}
	return src
}

// Artifact represents an AI-generated artifact (audio, video, report, etc.).
type Artifact struct {
	ID        string               `json:"id"`
	Title     string               `json:"title"`
	TypeCode  rpc.ArtifactTypeCode `json:"type_code"`
	Status    rpc.ArtifactStatus   `json:"status"`
	CreatedAt time.Time            `json:"created_at"`
	// URL is the downloadable media for audio, video, infographic and slide
	// deck (PDF) artifacts; empty for other types or while still generating.
	URL string `json:"url,omitempty"`
	// PPTXURL is the PowerPoint export of a slide deck.
	PPTXURL string `json:"pptx_url,omitempty"`
	// Variant distinguishes quiz (2), flashcards (1) and interactive mind map
	// (4) within rpc.ArtifactTypeQuiz.
	Variant int `json:"variant,omitempty"`

	row []any // raw list row, for content stored inline (reports, data tables)
}

// Kind returns the high-level ArtifactKind for this artifact.
func (a *Artifact) Kind() ArtifactKind {
	switch a.TypeCode {
	case rpc.ArtifactTypeAudio:
		return ArtifactKindAudio
	case rpc.ArtifactTypeVideo:
		return ArtifactKindVideo
	case rpc.ArtifactTypeReport:
		return ArtifactKindReport
	case rpc.ArtifactTypeQuiz:
		switch a.Variant {
		case rpc.ArtifactFlashcardsVariant:
			return ArtifactKindFlashcards
		case rpc.ArtifactInteractiveMindMapVariant:
			return ArtifactKindMindMap
		}
		return ArtifactKindQuiz
	case rpc.ArtifactTypeMindMap:
		return ArtifactKindMindMap
	case rpc.ArtifactTypeInfographic:
		return ArtifactKindInfographic
	case rpc.ArtifactTypeSlideDeck:
		return ArtifactKindSlideDeck
	case rpc.ArtifactTypeDataTable:
		return ArtifactKindDataTable
	}
	return ArtifactKindUnknown
}

// IsCompleted returns true if the artifact has finished generating.
func (a *Artifact) IsCompleted() bool { return a.Status == rpc.ArtifactCompleted }

// IsProcessing returns true if the artifact is still generating.
func (a *Artifact) IsProcessing() bool { return a.Status == rpc.ArtifactProcessing }

// IsPending returns true if the artifact is queued.
func (a *Artifact) IsPending() bool { return a.Status == rpc.ArtifactPending }

// IsFailed returns true if artifact generation failed.
func (a *Artifact) IsFailed() bool { return a.Status == rpc.ArtifactFailed }

// IsMediaReady reports whether a media artifact's download URL is populated.
// Media types can report completed shortly before their URL appears.
func (a *Artifact) IsMediaReady() bool {
	switch a.TypeCode {
	case rpc.ArtifactTypeAudio, rpc.ArtifactTypeVideo, rpc.ArtifactTypeInfographic, rpc.ArtifactTypeSlideDeck:
		return a.URL != ""
	}
	return true
}

// ArtifactFromAPIResponse parses an artifact row from LIST_ARTIFACTS:
//
//	[0] id  [1] title  [2] type  [4] status  [6] audio metadata  [7] report markdown
//	[8] video metadata  [9][1][0] variant  [14] infographic  [15][0] created (seconds)
//	[16] slide deck  [18] data table
func ArtifactFromAPIResponse(data []any) *Artifact {
	if len(data) == 0 {
		return nil
	}
	art := &Artifact{row: data}
	art.ID, _ = data[0].(string)
	if len(data) > 1 {
		art.Title, _ = data[1].(string)
	}
	if code, ok := intAt(data, 2); ok {
		art.TypeCode = rpc.ArtifactTypeCode(code)
	}
	if code, ok := intAt(data, 4); ok {
		art.Status = rpc.ArtifactStatus(code)
	}
	if options, ok := listAt(data, 9); ok {
		if gen, ok := listAt(options, 1); ok {
			art.Variant, _ = intAt(gen, 0)
		}
	}
	if ts, ok := listAt(data, 15); ok && len(ts) > 0 {
		if secs, ok := ts[0].(float64); ok {
			art.CreatedAt = time.Unix(int64(secs), 0).UTC()
		}
	}
	switch art.TypeCode {
	case rpc.ArtifactTypeAudio:
		art.URL = artifactAudioURL(data)
	case rpc.ArtifactTypeVideo:
		art.URL = artifactVideoURL(data)
	case rpc.ArtifactTypeInfographic:
		art.URL = artifactInfographicURL(data)
	case rpc.ArtifactTypeSlideDeck:
		if deck, ok := listAt(data, 16); ok {
			art.URL = artifactURLAt(deck, 3)
			art.PPTXURL = artifactURLAt(deck, 4)
		}
	}
	return art
}

func isArtifactURL(v any) bool {
	s, ok := v.(string)
	return ok && (strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://"))
}

func artifactURLAt(v []any, i int) string {
	if i < len(v) && isArtifactURL(v[i]) {
		return v[i].(string)
	}
	return ""
}

// artifactAudioURL picks from the media list at row[6][5] ([url, kind, mime]
// items), preferring audio/mp4.
func artifactAudioURL(row []any) string {
	block, _ := listAt(row, 6)
	media, _ := listAt(block, 5)
	fallback := ""
	for _, m := range media {
		item, _ := m.([]any)
		if len(item) == 0 || !isArtifactURL(item[0]) {
			continue
		}
		if fallback == "" {
			fallback = item[0].(string)
		}
		if len(item) > 2 && item[2] == "audio/mp4" {
			return item[0].(string)
		}
	}
	return fallback
}

// artifactVideoURL scans the lists under row[8] for [url, kind, mime] items,
// preferring video/mp4 of kind 4, then any video/mp4, then any URL.
func artifactVideoURL(row []any) string {
	variants, _ := listAt(row, 8)
	fallback := ""
	for _, v := range variants {
		media, _ := v.([]any)
		for _, m := range media {
			item, _ := m.([]any)
			if len(item) == 0 || !isArtifactURL(item[0]) {
				continue
			}
			url := item[0].(string)
			if fallback == "" {
				fallback = url
			}
			if len(item) > 2 && item[2] == "video/mp4" {
				if kind, _ := intAt(item, 1); kind == 4 {
					return url
				}
				fallback = url
			}
		}
	}
	return fallback
}

// artifactInfographicURL reads the first image at row[14][2][i][1][0], falling
// back to any top-level block shaped [?, ?, [[?, [url, ...]], ...]].
func artifactInfographicURL(row []any) string {
	if block, ok := listAt(row, 14); ok {
		items, _ := listAt(block, 2)
		for _, it := range items {
			item, _ := it.([]any)
			if image, ok := listAt(item, 1); ok {
				if u := artifactURLAt(image, 0); u != "" {
					return u
				}
			}
		}
	}
	for _, b := range row {
		block, _ := b.([]any)
		content, _ := listAt(block, 2)
		if len(content) == 0 {
			continue
		}
		first, _ := content[0].([]any)
		if image, ok := listAt(first, 1); ok {
			if u := artifactURLAt(image, 0); u != "" {
				return u
			}
		}
	}
	return ""
}

// GenerationStatus represents the status of an artifact generation task.
type GenerationStatus struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	URL       string `json:"url,omitempty"`
	Error     string `json:"error,omitempty"`
	ErrorCode any    `json:"error_code,omitempty"`
}

// IsComplete returns true if generation finished successfully.
func (g *GenerationStatus) IsComplete() bool { return g.Status == "completed" }

// IsFailed returns true if generation failed.
func (g *GenerationStatus) IsFailed() bool { return g.Status == "failed" }

// IsPending returns true if generation is queued.
func (g *GenerationStatus) IsPending() bool { return g.Status == "pending" }

// IsInProgress returns true if generation is running.
func (g *GenerationStatus) IsInProgress() bool { return g.Status == "in_progress" }

// ConversationTurn represents a single Q&A pair in a conversation.
type ConversationTurn struct {
	Query      string `json:"query"`
	Answer     string `json:"answer"`
	TurnNumber int    `json:"turn_number"`
}

// ChatReference represents a citation in a chat response.
type ChatReference struct {
	SourceID       string `json:"source_id"`
	CitationNumber int    `json:"citation_number"`
	CitedText      string `json:"cited_text"`
	StartChar      int    `json:"start_char"`
	EndChar        int    `json:"end_char"`
	ChunkID        string `json:"chunk_id"`
}

// AskResult represents the result of a chat question.
type AskResult struct {
	Answer         string          `json:"answer"`
	ConversationID string          `json:"conversation_id"`
	TurnNumber     int             `json:"turn_number"`
	IsFollowUp     bool            `json:"is_follow_up"`
	References     []ChatReference `json:"references,omitempty"`
}

// Note represents a user note in a notebook.
type Note struct {
	ID         string    `json:"id"`
	NotebookID string    `json:"notebook_id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// SharedUser represents a user who has access to a notebook.
type SharedUser struct {
	Email       string              `json:"email"`
	Permission  rpc.SharePermission `json:"permission"`
	DisplayName string              `json:"display_name,omitempty"`
	AvatarURL   string              `json:"avatar_url,omitempty"`
}

// ShareStatus represents the current sharing configuration of a notebook.
type ShareStatus struct {
	NotebookID  string             `json:"notebook_id"`
	IsPublic    bool               `json:"is_public"`
	Access      rpc.ShareAccess    `json:"access"`
	ViewLevel   rpc.ShareViewLevel `json:"view_level"`
	SharedUsers []SharedUser       `json:"shared_users,omitempty"`
	ShareURL    string             `json:"share_url,omitempty"`
	// MaxShareLimit is the maximum number of individual shares (0 if not reported).
	MaxShareLimit int `json:"max_share_limit,omitempty"`
	// PublicSharingAllowed is nil when the server did not say.
	PublicSharingAllowed *bool `json:"public_sharing_allowed,omitempty"`
}

// ReportSuggestion represents an AI-suggested report format.
type ReportSuggestion struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	Prompt        string `json:"prompt"`
	AudienceLevel string `json:"audience_level,omitempty"`
}

// GenerateAudioOptions configures audio overview generation.
type GenerateAudioOptions struct {
	SourceIDs    []string
	Language     string
	Instructions string
	Format       rpc.AudioFormat
	Length       rpc.AudioLength
}

// GenerateVideoOptions configures video overview generation.
type GenerateVideoOptions struct {
	SourceIDs    []string
	Language     string
	Instructions string
	Format       rpc.VideoFormat
	Style        rpc.VideoStyle
	// StylePrompt describes a custom visual style; setting it selects
	// rpc.VideoStyleCustom and overrides Style.
	StylePrompt string
}

// GenerateReportOptions configures report generation.
type GenerateReportOptions struct {
	Format            rpc.ReportFormat
	SourceIDs         []string
	Language          string
	CustomPrompt      string
	ExtraInstructions string
}

// GenerateQuizOptions configures quiz/flashcard generation.
type GenerateQuizOptions struct {
	SourceIDs    []string
	Language     string // unused by the API; quizzes follow the account language
	Instructions string
	Quantity     rpc.QuizQuantity
	Difficulty   rpc.QuizDifficulty
	Flashcards   bool // true for flashcards, false for quiz
}

// GenerateInfographicOptions configures infographic generation.
type GenerateInfographicOptions struct {
	SourceIDs    []string
	Language     string
	Instructions string
	Orientation  rpc.InfographicOrientation
	DetailLevel  rpc.InfographicDetail
	Style        rpc.InfographicStyle
}

// GenerateSlideDeckOptions configures slide deck generation.
type GenerateSlideDeckOptions struct {
	SourceIDs    []string
	Language     string
	Instructions string
	Format       rpc.SlideDeckFormat
	Length       rpc.SlideDeckLength
}

// GenerateDataTableOptions configures data table generation.
type GenerateDataTableOptions struct {
	SourceIDs    []string
	Language     string
	Instructions string
}
