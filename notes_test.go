package notebooklm

import (
	"reflect"
	"testing"
	"time"
)

func TestNoteRowsAndDecoding(t *testing.T) {
	mindMap := `{\"name\":\"Topics\",\"children\":[]}`
	cases := map[string]string{
		// Current shape: rows wrapped as [null, [id, content, meta, ?, title]].
		"wrapped container": `[[
			[null,["n1","Body one",[null,null,[1767225600,0]],null,"Title one"]],
			["n2",null,2],
			[null,["m1","` + mindMap + `",null,null,"Map"]]
		],[1767225600,0]]`,
		// Flat row list.
		"flat container": `[
			[null,["n1","Body one",[null,null,[1767225600,0]],null,"Title one"]],
			["n2",null,2],
			[null,["m1","` + mindMap + `",null,null,"Map"]]
		]`,
	}
	for name, raw := range cases {
		rows, err := noteRows(fromJSON(t, raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var notes, maps []Note
		for _, row := range rows {
			note, ok := noteFromRow(row, "nb")
			if !ok {
				continue
			}
			if isMindMapContent(note.Content) {
				maps = append(maps, note)
			} else {
				notes = append(notes, note)
			}
		}
		wantNotes := []Note{{ID: "n1", NotebookID: "nb", Title: "Title one", Content: "Body one", CreatedAt: time.Unix(1767225600, 0).UTC()}}
		if !reflect.DeepEqual(notes, wantNotes) {
			t.Errorf("%s notes: got %+v", name, notes)
		}
		if len(maps) != 1 || maps[0].ID != "m1" || maps[0].Title != "Map" {
			t.Errorf("%s mind maps: got %+v", name, maps)
		}
	}
}

func TestNoteRowsLegacyAndEmpty(t *testing.T) {
	rows, err := noteRows(fromJSON(t, `[[["n1","Legacy body"]]]`))
	if err != nil || len(rows) != 1 {
		t.Fatalf("got (%v, %v)", rows, err)
	}
	if note, ok := noteFromRow(rows[0], "nb"); !ok || note.Content != "Legacy body" || note.Title != "" {
		t.Errorf("legacy: got %+v ok=%v", note, ok)
	}
	for _, raw := range []string{`null`, `[]`, `[null,[1767225600,0]]`} {
		if rows, err := noteRows(fromJSON(t, raw)); err != nil || len(rows) != 0 {
			t.Errorf("%s: got (%v, %v)", raw, rows, err)
		}
	}
	if _, err := noteRows("not a list"); err == nil {
		t.Error("want error for non-list payload")
	}
}

func TestIsMindMapContent(t *testing.T) {
	for content, want := range map[string]bool{
		`{"name":"x","children":[]}`: true,
		`{"nodes":[]}`:               true,
		`My "children": Alice, Bob`:  false,
		``:                           false,
	} {
		if got := isMindMapContent(content); got != want {
			t.Errorf("%q: got %v", content, got)
		}
	}
}

func TestCreatedNoteID(t *testing.T) {
	for raw, want := range map[string]string{
		`[["note-1",null,[null,null,[1,0]]]]`: "note-1",
		`["note-2"]`:                          "note-2",
		`[]`:                                  "",
		`null`:                                "",
	} {
		if got := createdNoteID(fromJSON(t, raw)); got != want {
			t.Errorf("%s: got %q, want %q", raw, got, want)
		}
	}
}
