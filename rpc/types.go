// Package rpc provides low-level RPC encoding/decoding for the NotebookLM batchexecute API.
package rpc

// API endpoints
const (
	BatchExecuteURL = "https://notebooklm.google.com/_/LabsTailwindUi/data/batchexecute"
	QueryURL        = "https://notebooklm.google.com/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed"
	UploadURL       = "https://notebooklm.google.com/upload/_/"
	HomeURL         = "https://notebooklm.google.com/"

	// DefaultBuildLabel is the web app build sent as "bl" to the chat endpoint.
	// The server does not currently validate it.
	DefaultBuildLabel = "boq_labs-tailwind-frontend_20260802.02_p0"
)

// RPCMethod represents obfuscated method IDs used by the batchexecute API.
// These are reverse-engineered from network traffic analysis.
type RPCMethod string

const (
	// Notebook operations
	ListNotebooks        RPCMethod = "wXbhsf"
	CreateNotebook       RPCMethod = "CCqFvf"
	GetNotebook          RPCMethod = "rLM1Ne"
	RenameNotebook       RPCMethod = "s0tc2d"
	DeleteNotebook       RPCMethod = "WWINqb"
	RemoveRecentlyViewed RPCMethod = "fejl7e"

	// Source operations
	AddSource            RPCMethod = "izAoDd"
	AddSourceFile        RPCMethod = "o4cbdc" // Register uploaded file as source
	DeleteSource         RPCMethod = "tGMBJ"
	GetSource            RPCMethod = "hizoJc"
	RefreshSource        RPCMethod = "FLmJqe"
	CheckSourceFreshness RPCMethod = "yR9Yof"
	UpdateSource         RPCMethod = "b7Wfje"
	DiscoverSources      RPCMethod = "Es3dTe"

	// Summary and query
	Summarize           RPCMethod = "VfAZjd"
	GetSourceGuide      RPCMethod = "tr032e"
	GetSuggestedReports RPCMethod = "ciyUvf"

	// Artifact operations
	CreateArtifact     RPCMethod = "R7cb6c"
	ListArtifacts      RPCMethod = "gArtLc"
	DeleteArtifact     RPCMethod = "V5N4be"
	RenameArtifact     RPCMethod = "rc3d8d"
	ExportArtifact     RPCMethod = "Krh3pd"
	ShareArtifact      RPCMethod = "RGP97b"
	GetInteractiveHTML RPCMethod = "v9rmvd"
	ReviseSlide        RPCMethod = "KmcKPe"

	// Research
	StartFastResearch RPCMethod = "Ljjv0c"
	StartDeepResearch RPCMethod = "QA9ei"
	PollResearch      RPCMethod = "e3bVqc"
	ImportResearch    RPCMethod = "LBwxtb"

	// Note and mind map operations
	GenerateMindMap     RPCMethod = "yyryJe"
	CreateNote          RPCMethod = "CYK0Xb"
	GetNotesAndMindMaps RPCMethod = "cFji9"
	UpdateNote          RPCMethod = "cYAfTb"
	DeleteNote          RPCMethod = "AH0mwd"

	// Conversation
	GetLastConversationID RPCMethod = "hPTbtc"
	GetConversationTurns  RPCMethod = "khqZz"

	// Sharing operations
	ShareNotebook  RPCMethod = "QDyure"
	GetShareStatus RPCMethod = "JFMDGd"

	// User settings
	GetUserSettings RPCMethod = "ZwVcOc"
	SetUserSettings RPCMethod = "hT54vc"
)

// ArtifactTypeCode represents integer codes for artifact types in RPC calls.
type ArtifactTypeCode int

const (
	ArtifactTypeAudio       ArtifactTypeCode = 1
	ArtifactTypeReport      ArtifactTypeCode = 2
	ArtifactTypeVideo       ArtifactTypeCode = 3
	ArtifactTypeQuiz        ArtifactTypeCode = 4
	ArtifactTypeMindMap     ArtifactTypeCode = 5
	ArtifactTypeInfographic ArtifactTypeCode = 7
	ArtifactTypeSlideDeck   ArtifactTypeCode = 8
	ArtifactTypeDataTable   ArtifactTypeCode = 9
)

// ArtifactStatus represents the processing status of an artifact.
type ArtifactStatus int

const (
	ArtifactProcessing ArtifactStatus = 1
	ArtifactPending    ArtifactStatus = 2
	ArtifactCompleted  ArtifactStatus = 3
	ArtifactFailed     ArtifactStatus = 4
)

// ArtifactStatusToStr converts artifact status code to human-readable string.
func ArtifactStatusToStr(status ArtifactStatus) string {
	switch status {
	case ArtifactProcessing:
		return "in_progress"
	case ArtifactPending:
		return "pending"
	case ArtifactCompleted:
		return "completed"
	case ArtifactFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// SourceStatus represents the processing status of a source.
type SourceStatus int

const (
	SourceUnknown    SourceStatus = -1 // status block absent or code not recognized
	SourceProcessing SourceStatus = 1
	SourceReady      SourceStatus = 2
	SourceError      SourceStatus = 3
	SourcePreparing  SourceStatus = 5
)

// SourceStatusToStr converts source status code to human-readable string.
func SourceStatusToStr(status SourceStatus) string {
	switch status {
	case SourceProcessing:
		return "processing"
	case SourceReady:
		return "ready"
	case SourceError:
		return "error"
	case SourcePreparing:
		return "preparing"
	case SourceUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// AudioFormat represents audio overview format options.
type AudioFormat int

const (
	AudioDeepDive AudioFormat = 1
	AudioBrief    AudioFormat = 2
	AudioCritique AudioFormat = 3
	AudioDebate   AudioFormat = 4
)

// AudioLength represents audio overview length options.
type AudioLength int

const (
	AudioShort   AudioLength = 1
	AudioDefault AudioLength = 2
	AudioLong    AudioLength = 3
)

// VideoFormat represents video overview format options.
type VideoFormat int

const (
	VideoExplainer VideoFormat = 1
	VideoBrief     VideoFormat = 2
	VideoCinematic VideoFormat = 3
)

// VideoStyle represents video visual style options.
type VideoStyle int

const (
	VideoStyleAutoSelect VideoStyle = 1
	VideoStyleCustom     VideoStyle = 2
	VideoStyleClassic    VideoStyle = 3
	VideoStyleWhiteboard VideoStyle = 4
	VideoStyleKawaii     VideoStyle = 5
	VideoStyleAnime      VideoStyle = 6
	VideoStyleWatercolor VideoStyle = 7
	VideoStyleRetroPrint VideoStyle = 8
	VideoStyleHeritage   VideoStyle = 9
	VideoStylePaperCraft VideoStyle = 10
)

// QuizQuantity represents quiz/flashcards quantity options.
type QuizQuantity int

const (
	QuizFewer    QuizQuantity = 1
	QuizStandard QuizQuantity = 2
	QuizMore     QuizQuantity = 2 // Alias for standard (API limitation)
)

// QuizDifficulty represents quiz/flashcards difficulty options.
type QuizDifficulty int

const (
	QuizEasy   QuizDifficulty = 1
	QuizMedium QuizDifficulty = 2
	QuizHard   QuizDifficulty = 3
)

// InfographicOrientation represents infographic orientation options.
type InfographicOrientation int

const (
	InfographicLandscape InfographicOrientation = 1
	InfographicPortrait  InfographicOrientation = 2
	InfographicSquare    InfographicOrientation = 3
)

// InfographicDetail represents infographic detail level options.
type InfographicDetail int

const (
	InfographicConcise  InfographicDetail = 1
	InfographicStandard InfographicDetail = 2
	InfographicDetailed InfographicDetail = 3
)

// InfographicStyle represents infographic visual style options.
type InfographicStyle int

const (
	InfographicStyleAutoSelect    InfographicStyle = 1
	InfographicStyleSketchNote    InfographicStyle = 2
	InfographicStyleProfessional  InfographicStyle = 3
	InfographicStyleBentoGrid     InfographicStyle = 4
	InfographicStyleEditorial     InfographicStyle = 5
	InfographicStyleInstructional InfographicStyle = 6
	InfographicStyleBricks        InfographicStyle = 7
	InfographicStyleClay          InfographicStyle = 8
	InfographicStyleAnime         InfographicStyle = 9
	InfographicStyleKawaii        InfographicStyle = 10
	InfographicStyleScientific    InfographicStyle = 11
)

// SlideDeckFormat represents slide deck format options.
type SlideDeckFormat int

const (
	SlideDeckDetailed  SlideDeckFormat = 1
	SlideDeckPresenter SlideDeckFormat = 2
)

// SlideDeckLength represents slide deck length options.
type SlideDeckLength int

const (
	SlideDeckDefault SlideDeckLength = 1
	SlideDeckShort   SlideDeckLength = 2
)

// ReportFormat represents report format options.
type ReportFormat string

const (
	ReportBriefingDoc ReportFormat = "briefing_doc"
	ReportStudyGuide  ReportFormat = "study_guide"
	ReportBlogPost    ReportFormat = "blog_post"
	ReportCustom      ReportFormat = "custom"
)

// ChatGoal represents chat persona/goal options for notebook configuration.
type ChatGoal int

const (
	ChatDefault       ChatGoal = 1
	ChatCustom        ChatGoal = 2
	ChatLearningGuide ChatGoal = 3
)

// ChatResponseLength represents chat response length options.
type ChatResponseLength int

const (
	ChatResponseDefault ChatResponseLength = 1
	ChatResponseLonger  ChatResponseLength = 4
	ChatResponseShorter ChatResponseLength = 5
)

// ShareAccess represents notebook access level for public sharing.
type ShareAccess int

const (
	ShareRestricted     ShareAccess = 0
	ShareAnyoneWithLink ShareAccess = 1
)

// ShareViewLevel represents what viewers can access when shared.
type ShareViewLevel int

const (
	ShareFullNotebook ShareViewLevel = 0
	ShareChatOnly     ShareViewLevel = 1
)

// SharePermission represents user permission level for sharing.
type SharePermission int

const (
	ShareOwner  SharePermission = 1
	ShareEditor SharePermission = 2
	ShareViewer SharePermission = 3
	ShareRemove SharePermission = 4 // internal: remove user
)

// ExportType represents export destination types for artifacts.
type ExportType int

const (
	ExportDocs   ExportType = 1
	ExportSheets ExportType = 2
)
