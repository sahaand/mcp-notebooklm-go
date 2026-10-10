package notebooklm

import (
	"reflect"
	"testing"
)

func TestParseResearchTasks(t *testing.T) {
	result := fromJSON(t, `[[
		["task-fast",[null,["golang generics",1],1,[[
			["https://go.dev/blog","Go blog","hint",1],
			["https://example.com/g","Example",null,1]
		],"Two sources found"],2]],
		["task-deep",[null,["history of rpc",1],5,[[
			[null,"RPC report",null,5,null,null,["# Report",3]],
			["https://rpc.example","RPC",null,1]
		],"Deep summary"],6]],
		["task-running",[null,["q",1],1,null,1]],
		["task-empty",[null,["q",1],1,[[],""],3]]
	]]`)
	tasks := parseResearchTasks(result)
	if len(tasks) != 4 {
		t.Fatalf("got %d tasks", len(tasks))
	}
	want := &ResearchResult{
		TaskID: "task-fast", Status: "completed", Query: "golang generics", Summary: "Two sources found",
		Sources: []ResearchSource{{URL: "https://go.dev/blog", Title: "Go blog", ResultType: 1}, {URL: "https://example.com/g", Title: "Example", ResultType: 1}},
	}
	if !reflect.DeepEqual(tasks[0], want) {
		t.Errorf("fast:\n got  %+v\n want %+v", tasks[0], want)
	}
	deep := tasks[1]
	if deep.Status != "completed" || deep.Report != "# Report" || len(deep.Sources) != 2 || deep.Sources[0].ReportMarkdown != "# Report" {
		t.Errorf("deep: %+v", deep)
	}
	if tasks[2].Status != "in_progress" || tasks[3].Status != "failed" {
		t.Errorf("statuses: %s, %s", tasks[2].Status, tasks[3].Status)
	}

	// Flat (unwrapped) task list.
	flat := parseResearchTasks(fromJSON(t, `[["t1",[null,["q",1],1,null,2]]]`))
	if len(flat) != 1 || flat[0].TaskID != "t1" || flat[0].Status != "completed" {
		t.Errorf("flat: %+v", flat)
	}
}

func TestParseImportedSources(t *testing.T) {
	for _, raw := range []string{
		`[[["src-1"],"Go blog"],[["src-2"],"Example"]]`,
		`[[[["src-1"],"Go blog"],[["src-2"],"Example"]]]`,
	} {
		got := parseImportedSources(fromJSON(t, raw), "nb")
		if len(got) != 2 || got[0].ID != "src-1" || got[1].Title != "Example" {
			t.Errorf("%s: got %+v", raw, got)
		}
	}
	// A single flat row must not be mistaken for a wrapper.
	if got := parseImportedSources(fromJSON(t, `[[["src-1"],"Only"]]`), "nb"); len(got) != 1 || got[0].ID != "src-1" {
		t.Errorf("single row: %+v", got)
	}
}
