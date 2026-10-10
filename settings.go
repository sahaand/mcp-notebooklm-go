package notebooklm

import (
	"context"
	"fmt"

	"github.com/saeedata/notebooklm-go/rpc"
)

// SettingsAPI provides user settings operations.
type SettingsAPI struct {
	client *Client
}

// UserSettings holds user-level settings.
type UserSettings struct {
	OutputLanguage string `json:"output_language"`
}

// Get returns the current user settings.
func (s *SettingsAPI) Get(ctx context.Context) (*UserSettings, error) {
	params := []any{nil, []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}}}
	result, err := s.client.RPCCall(ctx, rpc.GetUserSettings, params, "/", false)
	if err != nil {
		return nil, fmt.Errorf("get user settings: %w", err)
	}
	arr, _ := result.([]any)
	account, _ := listAt(arr, 0)
	return &UserSettings{OutputLanguage: settingsLanguage(account)}, nil
}

// SetOutputLanguage updates the output language for generated content.
// Use language codes such as "en", "es", "ja" or "zh_Hans".
func (s *SettingsAPI) SetOutputLanguage(ctx context.Context, languageCode string) (*UserSettings, error) {
	if languageCode == "" {
		return nil, fmt.Errorf("set output language: language code must not be empty")
	}
	params := []any{[]any{[]any{nil, []any{[]any{nil, nil, nil, nil, []any{languageCode}}}}}}
	result, err := s.client.RPCCall(ctx, rpc.SetUserSettings, params, "/", false)
	if err != nil {
		return nil, fmt.Errorf("set output language to %s: %w", languageCode, err)
	}
	arr, _ := result.([]any)
	return &UserSettings{OutputLanguage: settingsLanguage(arr)}, nil
}

// settingsLanguage reads the output language at account[2][4][0]; "" means
// the account default.
func settingsLanguage(account []any) string {
	flags, _ := listAt(account, 2)
	lang, _ := listAt(flags, 4)
	s, _ := stringAt(lang, 0)
	return s
}
