package notebooklm

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/saeedata/notebooklm-go/rpc"
)

const clientOpts = `[2,null,null,[1,null,null,null,null,null,null,null,null,null,[1]],[[1,4,8,2,3,6]]]`

// descriptor renders the expected CREATE_ARTIFACT descriptor for source "s1":
// [null, null, type, [[["s1"]]], null..., config] with config at index slot.
func descriptor(typeCode, slot int, config string) string {
	parts := []string{"null", "null", strconv.Itoa(typeCode), `[[["s1"]]]`}
	for len(parts) < slot {
		parts = append(parts, "null")
	}
	parts = append(parts, config)
	return `[` + clientOpts + `,"nb",[` + strings.Join(parts, ",") + `]]`
}

// Expected configs are transcribed from notebooklm-py _web/params/creation.py.
func TestArtifactPayloads(t *testing.T) {
	ids := []string{"s1"}
	report, err := reportParams("nb", ids, GenerateReportOptions{ExtraInstructions: "Be brief."})
	if err != nil {
		t.Fatal(err)
	}
	custom, _ := reportParams("nb", ids, GenerateReportOptions{Format: rpc.ReportCustom})

	cases := []struct {
		name string
		got  any
		want string
	}{
		{"audio defaults", audioParams("nb", ids, GenerateAudioOptions{}),
			descriptor(1, 6, `[null,[null,2,null,[["s1"]],"en",null,1]]`)},
		{"audio options", audioParams("nb", ids, GenerateAudioOptions{Language: "de", Instructions: "x", Format: rpc.AudioDebate, Length: rpc.AudioLong}),
			descriptor(1, 6, `[null,["x",3,null,[["s1"]],"de",null,4]]`)},
		{"infographic defaults", infographicParams("nb", ids, GenerateInfographicOptions{}),
			descriptor(7, 14, `[[null,"en",null,1,2,1]]`)},
		{"slide deck defaults", slideDeckParams("nb", ids, GenerateSlideDeckOptions{}),
			descriptor(8, 16, `[[null,"en",1,1]]`)},
		{"data table", dataTableParams("nb", ids, GenerateDataTableOptions{Instructions: "compare"}),
			descriptor(9, 18, `[null,["compare","en"]]`)},
		{"interactive mind map", interactiveMindMapParams("nb", ids, ""),
			descriptor(4, 9, `[null,[4]]`)},
		{"video defaults", videoParams("nb", ids, GenerateVideoOptions{}),
			descriptor(3, 8, `[null,null,[[["s1"]],"en",null,null,1,1]]`)},
		{"video custom style", videoParams("nb", ids, GenerateVideoOptions{Instructions: "focus", StylePrompt: "neon"}),
			descriptor(3, 8, `[null,null,[[["s1"]],"en","focus",null,1,null,"neon"]]`)},
		{"video cinematic", videoParams("nb", ids, GenerateVideoOptions{Format: rpc.VideoCinematic}),
			descriptor(3, 8, `[null,null,[[["s1"]],"en",null,null,3]]`)},
		{"report briefing", report,
			descriptor(2, 7, `[null,["Briefing Doc","Key insights and important quotes",null,[["s1"]],"en","Create a comprehensive briefing document that includes an Executive Summary, detailed analysis of key themes, important quotes with context, and actionable insights.\n\nBe brief.",null,true]]`)},
		{"report custom", custom,
			descriptor(2, 7, `[null,["Custom Report","Custom format",null,[["s1"]],"en","Create a report based on the provided sources.",null,true]]`)},
		{"quiz", quizParams("nb", ids, GenerateQuizOptions{}),
			descriptor(4, 9, `[null,[2,null,null,null,null,null,null,[2,2]]]`)},
		{"flashcards", quizParams("nb", ids, GenerateQuizOptions{Flashcards: true, Quantity: rpc.QuizMore, Difficulty: rpc.QuizHard}),
			descriptor(4, 9, `[null,[1,null,null,null,null,null,[3,3]]]`)},
		{"mind map", mindMapParams(ids, "en", ""),
			`[[[["s1"]]],null,null,null,null,["interactive_mindmap",[["[CONTEXT]",""]],"en"],null,[2,null,[1]]]`},
	}
	for _, c := range cases {
		if got := toJSON(t, c.got); got != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.name, got, c.want)
		}
	}
	if _, err := reportParams("nb", ids, GenerateReportOptions{Format: "nope"}); err == nil {
		t.Error("unknown report format accepted")
	}
}

func TestParseGenerationResult(t *testing.T) {
	got, err := parseGenerationResult(fromJSON(t, `[["art-1","Title",1,null,2]]`), "x")
	if err != nil || got.TaskID != "art-1" || got.Status != "in_progress" {
		t.Fatalf("got (%+v, %v)", got, err)
	}
	for _, raw := range []string{`null`, `[[null]]`} {
		if _, err := parseGenerationResult(fromJSON(t, raw), "x"); err == nil || !strings.Contains(err.Error(), "unavailable") {
			t.Errorf("%s: got %v", raw, err)
		}
	}
}

func TestArtifactFromAPIResponse(t *testing.T) {
	audio := ArtifactFromAPIResponse(fromJSON(t, `["a1","Podcast",1,null,3,null,
		[null,null,null,null,null,[["https://x.googleusercontent.com/hls","1","application/x-mpegURL"],["https://x.googleusercontent.com/mp4","2","audio/mp4"]]],
		null,null,null,null,null,null,null,null,[1767225600,0]]`).([]any))
	if audio.URL != "https://x.googleusercontent.com/mp4" || audio.Status != rpc.ArtifactCompleted ||
		!audio.CreatedAt.Equal(time.Unix(1767225600, 0)) || audio.Kind() != ArtifactKindAudio {
		t.Errorf("audio: %+v", audio)
	}

	video := ArtifactFromAPIResponse(fromJSON(t, `["v1","Video",3,null,3,null,null,null,
		[null,[["https://v.googleusercontent.com/a",1,"video/webm"],["https://v.googleusercontent.com/b",2,"video/mp4"],["https://v.googleusercontent.com/c",4,"video/mp4"]]]]`).([]any))
	if video.URL != "https://v.googleusercontent.com/c" {
		t.Errorf("video URL %q", video.URL)
	}

	deck := ArtifactFromAPIResponse(fromJSON(t, `["d1","Deck",8,null,3,null,null,null,null,null,null,null,null,null,null,null,
		[null,null,[],"https://d.googleusercontent.com/deck.pdf","https://d.googleusercontent.com/deck.pptx"]]`).([]any))
	if deck.URL != "https://d.googleusercontent.com/deck.pdf" || deck.PPTXURL != "https://d.googleusercontent.com/deck.pptx" {
		t.Errorf("deck: %+v", deck)
	}

	pendingMedia := ArtifactFromAPIResponse(fromJSON(t, `["a2","Podcast",1,null,3]`).([]any))
	if pendingMedia.IsMediaReady() {
		t.Error("audio without URL reported media-ready")
	}

	for variant, kind := range map[int]ArtifactKind{1: ArtifactKindFlashcards, 2: ArtifactKindQuiz, 4: ArtifactKindMindMap} {
		row := fromJSON(t, `["q","Q",4,null,3,null,null,null,null,[null,[`+strconv.Itoa(variant)+`]]]`).([]any)
		if got := ArtifactFromAPIResponse(row).Kind(); got != kind {
			t.Errorf("variant %d: kind %s, want %s", variant, got, kind)
		}
	}
}

func TestUnwrapRows(t *testing.T) {
	cases := map[string]int{
		`[[["a1"],["a2"]]]`: 2, // wrapped
		`[["a1"],["a2"]]`:   2, // flat
		`[["a1","T",1]]`:    1, // flat, single row
		`[[]]`:              0,
		`[]`:                0,
	}
	for raw, want := range cases {
		arr, _ := fromJSON(t, raw).([]any)
		if got := len(unwrapRows(arr)); got != want {
			t.Errorf("%s: %d rows, want %d", raw, got, want)
		}
	}
}

func TestReportMarkdownAndDataTable(t *testing.T) {
	report := fromJSON(t, `["r1","R",2,null,3,null,null,["# Title\n\nBody"]]`).([]any)
	if got := reportMarkdown(report); got != "# Title\n\nBody" {
		t.Errorf("markdown %q", got)
	}

	cell := func(s string) string { return `[0,1,[[0,1,[[0,1,[["` + s + `"]]]]]]]` }
	table := `[[[[[null,null,null,null,[null,null,[` +
		`[0,10,[` + cell("Name") + `,` + cell("Age") + `]],` +
		`[10,20,[` + cell("Ada") + `,` + cell("36") + `]]` +
		`]]]]]]]`
	row := make([]any, 19)
	row[18] = fromJSON(t, table)
	got, err := dataTableCSV(row)
	if err != nil || string(got) != "Name,Age\nAda,36\n" {
		t.Errorf("csv (%q, %v)", got, err)
	}
	if _, err := dataTableCSV(make([]any, 19)); err == nil {
		t.Error("empty table accepted")
	}
}

func TestParseReportSuggestions(t *testing.T) {
	got := parseReportSuggestions(fromJSON(t, `[[["Timeline","Key dates",null,null,"Create a timeline",1]]]`))
	want := []ReportSuggestion{{Title: "Timeline", Description: "Key dates", Prompt: "Create a timeline", AudienceLevel: "1"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
}

func TestMindMapJSON(t *testing.T) {
	if got, ok := mindMapJSON(fromJSON(t, `[["{\"name\":\"Topics\",\"children\":[]}"]]`)); !ok || got != `{"name":"Topics","children":[]}` {
		t.Errorf("string leaf: (%q, %v)", got, ok)
	}
	if got, ok := mindMapJSON(fromJSON(t, `[[{"name":"T"}]]`)); !ok || got != `{"name":"T"}` {
		t.Errorf("object leaf: (%q, %v)", got, ok)
	}
	if _, ok := mindMapJSON(fromJSON(t, `[]`)); ok {
		t.Error("empty response decoded")
	}
}

func TestIsGoogleMediaURL(t *testing.T) {
	for u, want := range map[string]bool{
		"https://lh3.googleusercontent.com/x":    true,
		"https://notebooklm.google.com/x":        true,
		"https://storage.googleapis.com/x":       true,
		"http://lh3.googleusercontent.com/x":     false,
		"https://evil.example/x":                 false,
		"https://googleusercontent.com.evil.io/": false,
	} {
		if got := isGoogleMediaURL(u); got != want {
			t.Errorf("%s: got %v", u, got)
		}
	}
}
