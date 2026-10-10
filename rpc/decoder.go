package rpc

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// StripAntiXSSI removes the )]}' prefix Google adds to prevent XSSI attacks.
func StripAntiXSSI(response string) string {
	if strings.HasPrefix(response, ")]}'\n") {
		return response[5:]
	}
	if strings.HasPrefix(response, ")]}'\r\n") {
		return response[6:]
	}
	return response
}

// ParseChunkedResponse parses the alternating byte-count/JSON chunked format (rt=c mode).
func ParseChunkedResponse(response string) ([]any, error) {
	if strings.TrimSpace(response) == "" {
		return nil, nil
	}

	var chunks []any
	skipped := 0
	lines := strings.Split(strings.TrimSpace(response), "\n")

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		// Try to parse as byte count
		if _, err := strconv.Atoi(line); err == nil {
			i++
			if i < len(lines) {
				var chunk any
				if err := json.Unmarshal([]byte(lines[i]), &chunk); err != nil {
					skipped++
				} else {
					chunks = append(chunks, chunk)
				}
			}
			i++
			continue
		}

		// Not a byte count — try parsing as JSON directly
		var chunk any
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			skipped++
		} else {
			chunks = append(chunks, chunk)
		}
		i++
	}

	if skipped > 0 && len(lines) > 0 {
		rate := float64(skipped) / float64(len(lines))
		if rate > 0.1 {
			return nil, fmt.Errorf("response parsing failed: %d of %d chunks malformed (%.0f%%)", skipped, len(lines), rate*100)
		}
	}

	return chunks, nil
}

// gRPC canonical status codes (google.rpc.Code) that batchexecute embeds at
// index 5 of a wrb.fr frame when an RPC returns a null result.
const (
	GRPCInvalidArgument  = 3
	GRPCNotFound         = 5
	GRPCPermissionDenied = 7
)

var grpcStatusLabels = map[int]string{
	0:  "ok",
	1:  "cancelled",
	2:  "unknown",
	3:  "invalid argument",
	4:  "deadline exceeded",
	5:  "not found",
	6:  "already exists",
	7:  "permission denied",
	8:  "resource exhausted",
	9:  "failed precondition",
	10: "aborted",
	11: "out of range",
	12: "not implemented",
	13: "internal",
	14: "unavailable",
	15: "data loss",
	16: "unauthenticated",
}

// Appended to NOT_FOUND / PERMISSION_DENIED rejections, which are commonly an
// account-routing mismatch when several Google accounts are signed in.
const accountMismatchHint = " If several Google accounts are signed in, this is often an account-routing mismatch (requests default to account index 0)."

const maxStatusMessageChars = 300

// rpcItems returns every [tag, id, ...] item in the chunks, in order.
// A chunk is either a list of items or a single item.
func rpcItems(chunks []any) [][]any {
	var items [][]any
	for _, chunk := range chunks {
		list, ok := toSlice(chunk)
		if !ok || len(list) == 0 {
			continue
		}
		if _, nested := list[0].([]any); !nested {
			list = []any{list}
		}
		for _, v := range list {
			if item, ok := toSlice(v); ok && len(item) >= 3 {
				items = append(items, item)
			}
		}
	}
	return items
}

// collectRPCIDs returns the RPC IDs of every wrb.fr / er frame in the chunks.
func collectRPCIDs(chunks []any) []string {
	var ids []string
	for _, item := range rpcItems(chunks) {
		tag, _ := item[0].(string)
		id, ok := item[1].(string)
		if ok && (tag == "wrb.fr" || tag == "er") {
			ids = append(ids, id)
		}
	}
	return ids
}

// ExtractRPCResult finds and returns result data for a specific RPC method ID from chunks.
// In rt=c mode the server may send several wrb.fr frames for one ID (e.g. a null
// placeholder followed by the real payload), so the last non-null one wins.
// "er" frames and UserDisplayableError markers are terminal and returned as errors.
func ExtractRPCResult(chunks []any, rpcID string) (any, error) {
	var result any
	for _, item := range rpcItems(chunks) {
		tag, _ := item[0].(string)
		id, _ := item[1].(string)
		if id != rpcID {
			continue
		}

		if tag == "er" {
			code := item[2]
			return nil, &RPCError{
				Message:  errorMessageForCode(code),
				MethodID: rpcID,
				RPCCode:  code,
			}
		}
		if tag != "wrb.fr" {
			continue
		}

		resultData := item[2]

		// Check for UserDisplayableError (rate limit / quota) when result is nil
		if resultData == nil && len(item) > 5 && containsUserDisplayableError(item[5]) {
			return nil, userDisplayableError(rpcID, item[5])
		}

		// If resultData is a string, try JSON-parsing it
		if s, ok := resultData.(string); ok {
			var parsed any
			if err := json.Unmarshal([]byte(s), &parsed); err == nil {
				resultData = parsed
			}
		}
		if resultData != nil {
			result = resultData
		}
	}
	return result, nil
}

// NullPolicy controls how DecodeResponse treats a result frame that is present but null.
type NullPolicy int

const (
	// NullIsError: a null result is an error naming the server's status, if any.
	NullIsError NullPolicy = iota
	// NullAllowed: a null result decodes to nil, even when the server attached
	// a non-OK status (some RPCs do that on success, e.g. remove-from-recent).
	NullAllowed
	// NullAllowedUnlessRejected: a null result decodes to nil unless the server
	// attached a non-OK status, which is then an error. For RPCs whose success
	// value is null but which answer a rejection the same way (rename, refresh).
	NullAllowedUnlessRejected
)

// DecodeResponse is the full pipeline: strip prefix -> parse chunks -> extract result.
//
// A response that does not contain rpcID at all is always an error (the method ID
// changed, or the server sent a login / anti-bot page instead of RPC data). A frame
// that is present but null is handled according to policy.
func DecodeResponse(raw, rpcID string, policy NullPolicy) (any, error) {
	cleaned := StripAntiXSSI(raw)
	chunks, err := ParseChunkedResponse(cleaned)
	if err != nil {
		return nil, fmt.Errorf("parse chunked response: %w", err)
	}
	result, err := ExtractRPCResult(chunks, rpcID)
	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}

	foundIDs := collectRPCIDs(chunks)
	if len(foundIDs) == 0 {
		return nil, &RPCError{
			Message:  fmt.Sprintf("response contained no RPC data (%d chunks parsed) — possibly a login or anti-bot page", len(chunks)),
			MethodID: rpcID,
		}
	}
	if !containsString(foundIDs, rpcID) {
		return nil, &RPCError{
			Message:  fmt.Sprintf("no result for this RPC; response contains IDs %v — the method ID may have changed", foundIDs),
			MethodID: rpcID,
		}
	}
	code, label, payload, ok := findNullResultStatus(chunks, rpcID)
	rejected := ok && code != 0
	if policy == NullAllowed || (policy == NullAllowedUnlessRejected && !rejected) {
		return nil, nil
	}

	if rejected {
		msg := "server rejected the request (" + label + ")"
		if serverMsg := statusMessage(payload); serverMsg != "" {
			msg += ": " + serverMsg
		}
		if code == GRPCNotFound || code == GRPCPermissionDenied {
			msg += "." + accountMismatchHint
		}
		return nil, &RPCError{Message: msg, MethodID: rpcID, RPCCode: code, GRPCCode: code}
	}
	return nil, &RPCError{
		Message:  "server returned an empty result (possible parameter mismatch)",
		MethodID: rpcID,
	}
}

// findNullResultStatus returns the google.rpc.Status code at index 5 of the first
// null-result wrb.fr frame for rpcID that carries a recognized one.
func findNullResultStatus(chunks []any, rpcID string) (code int, label string, payload []any, ok bool) {
	for _, item := range rpcItems(chunks) {
		if len(item) < 6 || item[0] != "wrb.fr" || item[1] != rpcID || item[2] != nil {
			continue
		}
		if c, l, p, found := grpcStatus(item[5]); found {
			return c, l, p, true
		}
	}
	return 0, "", nil, false
}

// grpcStatus reads a JSON-array google.rpc.Status: [code, message?, details?].
func grpcStatus(v any) (code int, label string, payload []any, ok bool) {
	payload, isList := toSlice(v)
	if !isList || len(payload) == 0 {
		return 0, "", nil, false
	}
	f, isNum := payload[0].(float64)
	if !isNum || f != float64(int(f)) {
		return 0, "", nil, false
	}
	label, ok = grpcStatusLabels[int(f)]
	if !ok {
		return 0, "", nil, false
	}
	return int(f), label, payload, true
}

// statusMessage returns the server-authored message at index 1 of a
// google.rpc.Status, whitespace-collapsed and capped, or "".
func statusMessage(payload []any) string {
	if len(payload) < 2 {
		return ""
	}
	s, _ := payload[1].(string)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > maxStatusMessageChars {
		s = strings.TrimSpace(string(r[:maxStatusMessageChars])) + "…"
	}
	return s
}

func userDisplayableError(rpcID string, errorInfo any) *RPCError {
	err := &RPCError{
		Message:  "API rate limit or quota exceeded. Please wait before retrying.",
		MethodID: rpcID,
		RPCCode:  "USER_DISPLAYABLE_ERROR",
	}
	if code, label, payload, ok := grpcStatus(errorInfo); ok {
		if serverMsg := statusMessage(payload); serverMsg != "" {
			err.Message = serverMsg
		}
		err.Message += " (upstream: " + label + ")"
		err.GRPCCode = code
	}
	return err
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func toSlice(v any) ([]any, bool) {
	s, ok := v.([]any)
	return s, ok
}

func containsUserDisplayableError(v any) bool {
	return containsUserDisplayableErrorDepth(v, 20)
}

// The payload is server-controlled, so recursion is bounded.
func containsUserDisplayableErrorDepth(v any, depth int) bool {
	if depth <= 0 {
		return false
	}
	switch val := v.(type) {
	case string:
		return strings.Contains(val, "UserDisplayableError")
	case []any:
		for _, item := range val {
			if containsUserDisplayableErrorDepth(item, depth-1) {
				return true
			}
		}
	case map[string]any:
		for _, item := range val {
			if containsUserDisplayableErrorDepth(item, depth-1) {
				return true
			}
		}
	}
	return false
}

func errorMessageForCode(code any) string {
	if code == nil {
		return "unknown error"
	}
	if n, ok := code.(float64); ok {
		switch int(n) {
		case 400:
			return "invalid request parameters"
		case 401:
			return "authentication required — run 'notebooklm login'"
		case 403:
			return "insufficient permissions"
		case 404:
			return "resource not found"
		case 429:
			return "API rate limit exceeded — please wait before retrying"
		case 500:
			return "server error — usually temporary, try again later"
		}
	}
	return fmt.Sprintf("error code: %v", code)
}

// RPCError represents an error returned by the RPC API.
type RPCError struct {
	Message  string
	MethodID string
	RPCCode  any
	// GRPCCode is the google.rpc.Code the server attached to a rejected call
	// (e.g. GRPCInvalidArgument for a malformed payload); 0 when none was sent.
	GRPCCode int
}

func (e *RPCError) Error() string {
	if e.MethodID != "" {
		return fmt.Sprintf("RPC %s error: %s", e.MethodID, e.Message)
	}
	return e.Message
}
