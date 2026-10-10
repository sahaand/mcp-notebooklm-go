package notebooklm

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/saeedata/notebooklm-go/rpc"
)

// SharingAPI provides notebook sharing management operations.
type SharingAPI struct {
	client *Client
}

// GetStatus returns the current sharing configuration for a notebook.
func (s *SharingAPI) GetStatus(ctx context.Context, notebookID string) (*ShareStatus, error) {
	params := []any{notebookID, []any{2}}
	result, err := s.client.RPCCall(ctx, rpc.GetShareStatus, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("get share status for notebook %s: %w", notebookID, err)
	}
	return parseShareStatus(result, notebookID), nil
}

// SetPublic enables or disables public link sharing.
func (s *SharingAPI) SetPublic(ctx context.Context, notebookID string, public bool) (*ShareStatus, error) {
	access := int(rpc.ShareRestricted)
	if public {
		access = int(rpc.ShareAnyoneWithLink)
	}
	params := []any{[]any{[]any{notebookID, nil, []any{access}, []any{access, ""}}}, 1, nil, []any{2}}
	if _, err := s.client.RPCCall(ctx, rpc.ShareNotebook, params, "/notebook/"+notebookID, true); err != nil {
		return nil, fmt.Errorf("set public sharing: %w", err)
	}
	return s.GetStatus(ctx, notebookID)
}

// SetViewLevel configures what shared viewers can access. The status API does
// not report the view level, so the returned status carries the level just set.
func (s *SharingAPI) SetViewLevel(ctx context.Context, notebookID string, level rpc.ShareViewLevel) (*ShareStatus, error) {
	params := []any{notebookID, []any{[]any{nil, nil, nil, nil, nil, nil, nil, nil, []any{[]any{int(level)}}}}}
	if _, err := s.client.rpcCallPolicy(ctx, rpc.RenameNotebook, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected); err != nil {
		return nil, fmt.Errorf("set view level: %w", err)
	}
	status, err := s.GetStatus(ctx, notebookID)
	if err != nil {
		return nil, err
	}
	status.ViewLevel = level
	return status, nil
}

// AddUser shares a notebook with a user by email, or changes the permission of
// a user who already has access. The grant is confirmed against the
// notebook's share status afterwards.
func (s *SharingAPI) AddUser(ctx context.Context, notebookID, email string, permission rpc.SharePermission, sendNotification bool, message string) (*ShareStatus, error) {
	if permission != rpc.ShareEditor && permission != rpc.ShareViewer {
		return nil, fmt.Errorf("permission must be editor or viewer (use RemoveUser to revoke access)")
	}
	messageBlock := []any{1, ""}
	if message != "" {
		messageBlock = []any{0, message}
	}
	params := shareUserParams(notebookID, email, permission, sendNotification, messageBlock)
	if _, err := s.client.rpcCallPolicy(ctx, rpc.ShareNotebook, params, "/notebook/"+notebookID, rpc.NullAllowedUnlessRejected); err != nil {
		return nil, fmt.Errorf("share with %s: %w", email, err)
	}
	status, err := s.GetStatus(ctx, notebookID)
	if err != nil {
		return nil, err
	}
	if !hasGrant(status, email, permission) {
		return status, fmt.Errorf("share with %s: the share status does not show the requested permission; check sharing before retrying (an invitation may already have been sent)", email)
	}
	return status, nil
}

// UpdateUser changes an existing user's permission level without notifying them.
func (s *SharingAPI) UpdateUser(ctx context.Context, notebookID, email string, permission rpc.SharePermission) (*ShareStatus, error) {
	return s.AddUser(ctx, notebookID, email, permission, false, "")
}

// RemoveUser revokes a user's access to the notebook.
func (s *SharingAPI) RemoveUser(ctx context.Context, notebookID, email string) (*ShareStatus, error) {
	params := shareUserParams(notebookID, email, rpc.ShareRemove, false, []any{0, ""})
	if _, err := s.client.RPCCall(ctx, rpc.ShareNotebook, params, "/notebook/"+notebookID, true); err != nil {
		return nil, fmt.Errorf("remove user %s: %w", email, err)
	}
	return s.GetStatus(ctx, notebookID)
}

// shareUserParams builds the SHARE_NOTEBOOK user grant / removal payload:
// [[[nb, [[email, null, permission]], null, [message_flag, message]]], notify, null, [2]].
func shareUserParams(notebookID, email string, permission rpc.SharePermission, notify bool, messageBlock []any) []any {
	notifyFlag := 0
	if notify {
		notifyFlag = 1
	}
	return []any{
		[]any{[]any{notebookID, []any{[]any{email, nil, int(permission)}}, nil, messageBlock}},
		notifyFlag,
		nil,
		[]any{2},
	}
}

func hasGrant(status *ShareStatus, email string, permission rpc.SharePermission) bool {
	for _, u := range status.SharedUsers {
		if strings.EqualFold(u.Email, email) && u.Permission == permission {
			return true
		}
	}
	return false
}

// parseShareStatus reads GET_SHARE_STATUS:
// [users, [is_public], share_limit, public_sharing_allowed], each user being
// [email, permission, ?, [display_name, avatar_url]].
func parseShareStatus(result any, notebookID string) *ShareStatus {
	status := &ShareStatus{NotebookID: notebookID, Access: rpc.ShareRestricted, ViewLevel: rpc.ShareFullNotebook}
	arr, _ := result.([]any)

	users, _ := listAt(arr, 0)
	for _, u := range users {
		row, _ := u.([]any)
		if len(row) == 0 {
			continue
		}
		user := SharedUser{Permission: rpc.ShareViewer}
		user.Email, _ = stringAt(row, 0)
		if perm, ok := intAt(row, 1); ok {
			user.Permission = rpc.SharePermission(perm)
		}
		if info, ok := listAt(row, 3); ok {
			user.DisplayName, _ = stringAt(info, 0)
			user.AvatarURL, _ = stringAt(info, 1)
		}
		status.SharedUsers = append(status.SharedUsers, user)
	}

	if public, ok := listAt(arr, 1); ok && len(public) > 0 {
		switch v := public[0].(type) {
		case bool:
			status.IsPublic = v
		case float64:
			status.IsPublic = v != 0
		}
	}
	if status.IsPublic {
		status.Access = rpc.ShareAnyoneWithLink
		status.ShareURL = "https://notebooklm.google.com/notebook/" + url.PathEscape(notebookID)
	}
	status.MaxShareLimit, _ = intAt(arr, 2)
	if len(arr) > 3 {
		if allowed, ok := arr[3].(bool); ok {
			status.PublicSharingAllowed = &allowed
		}
	}
	return status
}
