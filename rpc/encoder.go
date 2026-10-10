package rpc

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// EncodeRPCRequest encodes an RPC request into batchexecute triple-nested array format:
// [[[rpc_id, json_params, null, "generic"]]]
func EncodeRPCRequest(method RPCMethod, params any) ([]any, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	inner := []any{string(method), string(paramsJSON), nil, "generic"}
	return []any{[]any{inner}}, nil
}

// BuildRequestBody creates form-encoded request body for batchexecute.
// Format: f.req=<url-encoded-json>[&at=<csrf>]&
func BuildRequestBody(rpcRequest []any, csrfToken string) (string, error) {
	reqJSON, err := json.Marshal(rpcRequest)
	if err != nil {
		return "", fmt.Errorf("marshal rpc request: %w", err)
	}
	body := "f.req=" + url.QueryEscape(string(reqJSON))
	if csrfToken != "" {
		body += "&at=" + url.QueryEscape(csrfToken)
	}
	body += "&"
	return body, nil
}

// BuildURLParams constructs query parameters for batchexecute request.
func BuildURLParams(method RPCMethod, sourcePath, sessionID string) url.Values {
	if sourcePath == "" {
		sourcePath = "/"
	}
	params := url.Values{
		"rpcids":      {string(method)},
		"source-path": {sourcePath},
		"hl":          {"en"},
		"rt":          {"c"},
	}
	if sessionID != "" {
		params.Set("f.sid", sessionID)
	}
	return params
}

// TemplateBlock returns the nested request-context block
// [2, null, null, [1, null x9, [1]]] that create-notebook, every add-source
// variant and get-notebook now require (older payloads sent a flat [2] and are
// rejected with INVALID_ARGUMENT). A fresh value is returned on each call.
func TemplateBlock() []any {
	return []any{2, nil, nil, []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}}}
}

// NestIDs wraps each ID in depth inner lists and collects them:
// depth 1 -> [[id1], [id2]], depth 2 -> [[[id1]], [[id2]]].
// It returns an empty (non-nil) list for no IDs, which encodes as [].
func NestIDs(ids []string, depth int) []any {
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		var v any = id
		for i := 0; i < depth; i++ {
			v = []any{v}
		}
		out = append(out, v)
	}
	return out
}
