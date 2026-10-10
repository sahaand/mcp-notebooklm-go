package notebooklm

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/saeedata/notebooklm-go/rpc"
)

const tb = `[2,null,null,[1,null,null,null,null,null,null,null,null,null,[1]]]`

func toJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func fromJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("bad test JSON %s: %v", s, err)
	}
	return v
}

// Expected payloads are transcribed from notebooklm-py (_web/sources/add.py,
// _web/params/sources.py, _web/params/notebooks.py).
func TestSourcePayloads(t *testing.T) {
	cases := []struct {
		name string
		got  any
		want string
	}{
		{"get notebook", getNotebookParams("nb"),
			`["nb",null,` + tb + `,null,0]`},
		{"add url", addURLParams("nb", "https://example.com/a"),
			`[[[null,null,["https://example.com/a"],null,null,null,null,null,null,null,1]],"nb",` + tb + `]`},
		{"add youtube", addYouTubeParams("nb", "https://youtu.be/dQw4w9WgXcQ"),
			`[[[null,null,null,null,null,null,null,["https://youtu.be/dQw4w9WgXcQ"],null,null,1]],"nb",` + tb + `]`},
		{"add text", addTextParams("nb", "T", "body"),
			`[[[null,["T","body"],null,2,null,null,null,null,null,null,1]],"nb",` + tb + `]`},
		{"add drive", addDriveParams("nb", "FILE", "application/pdf", "T"),
			`[[[["FILE","application/pdf",1,"T"],null,null,null,null,null,null,null,null,null,1]],"nb",[2],[1,null,null,null,null,null,null,null,null,null,[1]]]`},
		{"register file", registerFileParams("nb", "a.pdf"),
			`[[["a.pdf"]],"nb",` + tb + `]`},
	}
	for _, c := range cases {
		if got := toJSON(t, c.got); got != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.name, got, c.want)
		}
	}
}

func TestParseSourceList(t *testing.T) {
	result := fromJSON(t, `[["Notebook",[
		[["src-1"],"Web page",[null,120,[1767225600,0],null,5,null,null,["https://example.com"]],[null,2]],
		[[null,true,["src-2"]],"Drive doc",[null,null,null,null,1],[null,1,null,2]],
		[["src-3"],"Video",[null,null,null,null,9,["https://youtu.be/x"]],[null,99]]
	],"nb-id"]]`)
	got, err := parseSourceList(result, "nb-id")
	if err != nil {
		t.Fatal(err)
	}
	want := []Source{
		{ID: "src-1", NotebookID: "nb-id", Title: "Web page", URL: "https://example.com", TypeCode: 5,
			CreatedAt: time.Unix(1767225600, 0).UTC(), Status: rpc.SourceReady},
		{ID: "src-2", NotebookID: "nb-id", Title: "Drive doc", TypeCode: 1, Status: rpc.SourceProcessing},
		{ID: "src-3", NotebookID: "nb-id", Title: "Video", URL: "https://youtu.be/x", TypeCode: 9, Status: rpc.SourceUnknown},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if got[0].Kind() != SourceTypeWebPage || got[2].Kind() != SourceTypeYouTube {
		t.Errorf("kinds: %s, %s", got[0].Kind(), got[2].Kind())
	}
}

func TestParseSourceListEmptyNotebook(t *testing.T) {
	got, err := parseSourceList(fromJSON(t, `[["Empty",null,"nb-id"]]`), "nb-id")
	if err != nil || len(got) != 0 {
		t.Fatalf("got (%v, %v)", got, err)
	}
	if _, err := parseSourceList(fromJSON(t, `[]`), "nb-id"); err == nil {
		t.Fatal("want error for malformed response")
	}
}

func TestParseSourceResultShapes(t *testing.T) {
	cases := map[string]string{
		"medium": `[[["new-id"],"Title",[null,null,null,null,5,null,null,["https://e.com"]],[null,1]]]`,
		"deep":   `[[[["new-id"],"Title",[null,null,null,null,5,null,null,["https://e.com"]],[null,1]]]]`,
		"flat":   `["new-id","Title",[null,null,null,null,5,null,null,["https://e.com"]],[null,1]]`,
	}
	for name, raw := range cases {
		src, ok := parseSourceResult(fromJSON(t, raw), "nb")
		if !ok || src.ID != "new-id" || src.Title != "Title" || src.URL != "https://e.com" || src.Status != rpc.SourceProcessing {
			t.Errorf("%s: got %+v ok=%v", name, src, ok)
		}
	}
	// Only the deep shape honors a bare URL at metadata[0].
	deep, _ := parseSourceResult(fromJSON(t, `[[[["id"],"T",["https://bare.example"]]]]`), "nb")
	medium, _ := parseSourceResult(fromJSON(t, `[[["id"],"T",["https://bare.example"]]]`), "nb")
	if deep.URL != "https://bare.example" || medium.URL != "" {
		t.Errorf("bare URL: deep %q, medium %q", deep.URL, medium.URL)
	}
	if _, ok := parseSourceResult(nil, "nb"); ok {
		t.Error("nil result decoded as a source")
	}
}

func TestRegisteredFileSourceID(t *testing.T) {
	const uuid = "0f8fad5b-d9cb-469f-a165-70867728950e"
	cases := []struct {
		raw  string
		want string
	}{
		{`[["` + uuid + `"]]`, uuid},
		{`[[[["` + uuid + `"]]]]`, uuid},
		{`[null,[["` + uuid + `"]]]`, uuid},
		{`[[["` + uuid + `"],"report.pdf"]]`, uuid},
		{`[["report.pdf"]]`, ""},
		{`[["has space 1"]]`, ""},
		{`null`, ""},
	}
	for _, c := range cases {
		if got := registeredFileSourceID(fromJSON(t, c.raw), "report.pdf"); got != c.want {
			t.Errorf("%s: got %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestParseSourceGuide(t *testing.T) {
	g := parseSourceGuide(fromJSON(t, `[[[null,["A summary."],[["go","rpc"]]]]]`))
	if g.Summary != "A summary." || !reflect.DeepEqual(g.Keywords, []string{"go", "rpc"}) {
		t.Errorf("got %+v", g)
	}
	if g := parseSourceGuide(nil); g.Summary != "" || g.Keywords == nil {
		t.Errorf("empty guide: %+v", g)
	}
}

func TestParseSourceFulltext(t *testing.T) {
	raw := `[[["s1"],"Doc",[null,null,null,null,5,null,null,["https://e.com"]]],null,null,[[["Para one",["Para two",[""]]],["Para three"]]]]`
	ft, ok := parseSourceFulltext(fromJSON(t, raw), "s1")
	if !ok {
		t.Fatal("not decoded")
	}
	want := &SourceFulltext{SourceID: "s1", Title: "Doc", URL: "https://e.com", TypeCode: 5, Content: "Para one\nPara two\nPara three"}
	if !reflect.DeepEqual(ft, want) {
		t.Errorf("got %+v", ft)
	}
	if _, ok := parseSourceFulltext(nil, "s1"); ok {
		t.Error("nil result decoded")
	}
}

func TestInterpretFreshness(t *testing.T) {
	cases := []struct {
		raw       string
		fresh, ok bool
	}{
		{`true`, true, true},
		{`false`, false, true},
		{`[]`, true, true},
		{`[[null,true,["id"]]]`, true, true},
		{`[[null,false,["id"]]]`, false, true},
		{`null`, false, false},
		{`["x"]`, false, false},
		{`[[null,null]]`, false, false},
	}
	for _, c := range cases {
		fresh, ok := interpretFreshness(fromJSON(t, c.raw))
		if fresh != c.fresh || ok != c.ok {
			t.Errorf("%s: got (%v, %v), want (%v, %v)", c.raw, fresh, ok, c.fresh, c.ok)
		}
	}
}

func TestParseDiscoverResult(t *testing.T) {
	res, ok := parseDiscoverResult(fromJSON(t, `[[["https://a.com","A"],["https://b.com","B",null,1]],"Overview",["job-1"]]`))
	want := &DiscoverResult{TaskID: "job-1", Summary: "Overview", Sources: []DiscoveredSource{{"https://a.com", "A"}, {"https://b.com", "B"}}}
	if !ok || !reflect.DeepEqual(res, want) {
		t.Errorf("got %+v ok=%v", res, ok)
	}
	failed, ok := parseDiscoverResult(fromJSON(t, `[null,null,["job-2"],null,1]`))
	if !ok || !failed.Failed || len(failed.Sources) != 0 {
		t.Errorf("failed run: got %+v ok=%v", failed, ok)
	}
	if _, ok := parseDiscoverResult(fromJSON(t, `[[],"x"]`)); ok {
		t.Error("decoded a response without a job ID")
	}
}

func TestDriveFileID(t *testing.T) {
	cases := map[string]string{
		"1AbC_d-9":                                         "1AbC_d-9",
		"https://docs.google.com/document/d/1AbC_d-9/edit": "1AbC_d-9",
		"https://drive.google.com/file/d/1AbC_d-9/view":    "1AbC_d-9",
		"https://drive.google.com/open?id=1AbC_d-9":        "1AbC_d-9",
		"https://drive.google.com/drive/my-drive":          "",
	}
	for in, want := range cases {
		if got := driveFileID(in); got != want {
			t.Errorf("driveFileID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestYouTubeVideoID(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":          "dQw4w9WgXcQ",
		"https://www.youtube.com/watch?list=x&v=dQw4w9WgXcQ":   "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ?t=3":                     "dQw4w9WgXcQ",
		"https://youtube.com/shorts/dQw4w9WgXcQ":               "dQw4w9WgXcQ",
		"https://www.youtube.com/@channel":                     "",
		"https://example.com/watch?v=dQw4w9WgXcQ-not-youtube/": "",
	}
	for in, want := range cases {
		if got := youtubeVideoID(in); got != want {
			t.Errorf("youtubeVideoID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateUploadURL(t *testing.T) {
	for _, ok := range []string{
		"https://notebooklm.google.com/upload/_/?upload_id=abc",
		"https://notebook.google.com/upload/_/?upload_id=abc",
	} {
		if err := validateUploadURL(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "http://notebooklm.google.com/upload", "https://evil.example/upload", "https://notgoogle.com/x"} {
		if err := validateUploadURL(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
