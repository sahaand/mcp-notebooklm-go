package notebooklm

import (
	"reflect"
	"testing"

	"github.com/saeedata/notebooklm-go/rpc"
)

// Expected payloads are transcribed from notebooklm-py _web/sharing.py.
func TestShareUserParams(t *testing.T) {
	cases := []struct {
		name string
		got  any
		want string
	}{
		{"grant with message", shareUserParams("nb", "a@x.com", rpc.ShareEditor, true, []any{0, "hi"}),
			`[[["nb",[["a@x.com",null,2]],null,[0,"hi"]]],1,null,[2]]`},
		{"grant without message", shareUserParams("nb", "a@x.com", rpc.ShareViewer, false, []any{1, ""}),
			`[[["nb",[["a@x.com",null,3]],null,[1,""]]],0,null,[2]]`},
		{"remove", shareUserParams("nb", "a@x.com", rpc.ShareRemove, false, []any{0, ""}),
			`[[["nb",[["a@x.com",null,4]],null,[0,""]]],0,null,[2]]`},
	}
	for _, c := range cases {
		if got := toJSON(t, c.got); got != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.name, got, c.want)
		}
	}
}

func TestParseShareStatus(t *testing.T) {
	status := parseShareStatus(fromJSON(t, `[
		[["owner@x.com",1,null,["Owner","https://lh3/o"]],["v@x.com",3]],
		[true],
		50,
		true
	]`), "nb/1")
	allowed := true
	want := &ShareStatus{
		NotebookID: "nb/1",
		IsPublic:   true,
		Access:     rpc.ShareAnyoneWithLink,
		ViewLevel:  rpc.ShareFullNotebook,
		SharedUsers: []SharedUser{
			{Email: "owner@x.com", Permission: rpc.ShareOwner, DisplayName: "Owner", AvatarURL: "https://lh3/o"},
			{Email: "v@x.com", Permission: rpc.ShareViewer},
		},
		ShareURL:             "https://notebooklm.google.com/notebook/nb%2F1",
		MaxShareLimit:        50,
		PublicSharingAllowed: &allowed,
	}
	if !reflect.DeepEqual(status, want) {
		t.Errorf("got  %+v\nwant %+v", status, want)
	}
	if !hasGrant(status, "V@x.com", rpc.ShareViewer) || hasGrant(status, "v@x.com", rpc.ShareEditor) {
		t.Error("hasGrant mismatch")
	}

	private := parseShareStatus(fromJSON(t, `[[],[false]]`), "nb")
	if private.IsPublic || private.ShareURL != "" || private.PublicSharingAllowed != nil {
		t.Errorf("private: %+v", private)
	}
}

func TestSettingsLanguage(t *testing.T) {
	get := fromJSON(t, `[[null,null,[null,null,null,null,["ja"]]]]`).([]any)
	account, _ := listAt(get, 0)
	if got := settingsLanguage(account); got != "ja" {
		t.Errorf("get: %q", got)
	}
	set := fromJSON(t, `[null,null,[null,null,null,null,["zh_Hans"]]]`).([]any)
	if got := settingsLanguage(set); got != "zh_Hans" {
		t.Errorf("set: %q", got)
	}
	if got := settingsLanguage(nil); got != "" {
		t.Errorf("empty: %q", got)
	}
}
