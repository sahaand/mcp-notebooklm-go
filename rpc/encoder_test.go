package rpc

import (
	"encoding/json"
	"net/url"
	"testing"
)

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestTemplateBlock(t *testing.T) {
	want := `[2,null,null,[1,null,null,null,null,null,null,null,null,null,[1]]]`
	if got := mustJSON(t, TemplateBlock()); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	// Each call must return an independent value.
	a := TemplateBlock()
	a[0] = 99
	if TemplateBlock()[0] != 2 {
		t.Fatal("TemplateBlock shares state between calls")
	}
}

func TestNestIDs(t *testing.T) {
	cases := []struct {
		ids   []string
		depth int
		want  string
	}{
		{nil, 1, `[]`},
		{[]string{}, 2, `[]`},
		{[]string{"a", "b"}, 1, `[["a"],["b"]]`},
		{[]string{"a", "b"}, 2, `[[["a"]],[["b"]]]`},
		{[]string{"a"}, 3, `[[[["a"]]]]`},
	}
	for _, c := range cases {
		if got := mustJSON(t, NestIDs(c.ids, c.depth)); got != c.want {
			t.Errorf("NestIDs(%v, %d) = %s, want %s", c.ids, c.depth, got, c.want)
		}
	}
}

func TestEncodeRequestBody(t *testing.T) {
	params := []any{[]any{[]any{nil, nil, []any{"https://example.com"}, nil, nil, nil, nil, nil, nil, nil, 1}}, "nb-1", TemplateBlock()}
	req, err := EncodeRPCRequest(AddSource, params)
	if err != nil {
		t.Fatal(err)
	}
	body, err := BuildRequestBody(req, "csrf-token")
	if err != nil {
		t.Fatal(err)
	}
	form, err := url.ParseQuery(body)
	if err != nil {
		t.Fatal(err)
	}
	if form.Get("at") != "csrf-token" {
		t.Errorf("at = %q", form.Get("at"))
	}
	wantInner := `[[null,null,["https://example.com"],null,null,null,null,null,null,null,1]],"nb-1",[2,null,null,[1,null,null,null,null,null,null,null,null,null,[1]]]]`
	want := mustJSON(t, []any{[]any{[]any{"izAoDd", "[" + wantInner, nil, "generic"}}})
	if got := form.Get("f.req"); got != want {
		t.Errorf("f.req =\n%s\nwant\n%s", got, want)
	}
}

func TestBuildURLParams(t *testing.T) {
	p := BuildURLParams(AddSource, "/notebook/nb-1", "sid-1")
	for k, want := range map[string]string{
		"rpcids": "izAoDd", "source-path": "/notebook/nb-1", "hl": "en", "rt": "c", "f.sid": "sid-1",
	} {
		if got := p.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if got := BuildURLParams(AddSource, "", "").Get("source-path"); got != "/" {
		t.Errorf("default source-path = %q, want /", got)
	}
}
