package rpc

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// chunked builds an rt=c batchexecute response body from JSON chunk lines.
func chunked(lines ...string) string {
	var b strings.Builder
	b.WriteString(")]}'\n\n")
	for _, l := range lines {
		fmt.Fprintf(&b, "%d\n%s\n", len(l)+1, l)
	}
	return b.String()
}

const trailer = `[["di",42],["af.httprm",41,"-1234567890",5]]`

func TestDecodeResponseResult(t *testing.T) {
	raw := chunked(`[["wrb.fr","izAoDd","[[[\"src-1\"],\"Title\"]]",null,null,null,"generic"]]`, trailer)
	got, err := DecodeResponse(raw, "izAoDd", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fmt.Sprint(got) != "[[[src-1] Title]]" {
		t.Fatalf("got %v", got)
	}
}

func TestDecodeResponseLastNonNullFrameWins(t *testing.T) {
	raw := chunked(
		`[["wrb.fr","rLM1Ne",null,null,null,null,"generic"]]`,
		`[["wrb.fr","rLM1Ne","[\"final\"]",null,null,null,"generic"]]`,
		trailer,
	)
	got, err := DecodeResponse(raw, "rLM1Ne", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fmt.Sprint(got) != "[final]" {
		t.Fatalf("got %v, want [final]", got)
	}
}

func TestDecodeResponseNullWithStatus(t *testing.T) {
	// The shape returned when izAoDd gets an outdated payload.
	raw := chunked(`[["wrb.fr","izAoDd",null,null,null,[3],"generic"]]`, trailer)

	_, err := DecodeResponse(raw, "izAoDd", false)
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("want *RPCError, got %v", err)
	}
	if rpcErr.GRPCCode != GRPCInvalidArgument {
		t.Errorf("GRPCCode = %d, want %d", rpcErr.GRPCCode, GRPCInvalidArgument)
	}
	if !strings.Contains(err.Error(), "invalid argument") {
		t.Errorf("error %q does not name the status", err)
	}

	// allowNull callers still get a nil result, matching the reference.
	got, err := DecodeResponse(raw, "izAoDd", true)
	if err != nil || got != nil {
		t.Fatalf("allowNull: got (%v, %v), want (nil, nil)", got, err)
	}
}

func TestDecodeResponseNotFoundHint(t *testing.T) {
	raw := chunked(`[["wrb.fr","rLM1Ne",null,null,null,[5],"generic"]]`, trailer)
	_, err := DecodeResponse(raw, "rLM1Ne", false)
	if err == nil || !strings.Contains(err.Error(), "not found") || !strings.Contains(err.Error(), "account") {
		t.Fatalf("want not-found error with account hint, got %v", err)
	}
}

func TestDecodeResponseServerStatusMessage(t *testing.T) {
	raw := chunked(`[["wrb.fr","R7cb6c",null,null,null,[9,"  Notebook   has no sources "],"generic"]]`, trailer)
	_, err := DecodeResponse(raw, "R7cb6c", false)
	if err == nil || !strings.Contains(err.Error(), "(failed precondition): Notebook has no sources") {
		t.Fatalf("got %v", err)
	}
}

func TestDecodeResponseNullWithoutStatus(t *testing.T) {
	raw := chunked(`[["wrb.fr","izAoDd",null,null,null,null,"generic"]]`, trailer)
	_, err := DecodeResponse(raw, "izAoDd", false)
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) || rpcErr.GRPCCode != 0 || !strings.Contains(err.Error(), "empty result") {
		t.Fatalf("got %v", err)
	}
}

func TestDecodeResponseUnrecognizedStatusIgnored(t *testing.T) {
	for _, status := range []string{`[99]`, `[true]`, `["3"]`, `[[3]]`, `[3.5]`} {
		raw := chunked(`[["wrb.fr","izAoDd",null,null,null,`+status+`,"generic"]]`, trailer)
		_, err := DecodeResponse(raw, "izAoDd", false)
		var rpcErr *RPCError
		if !errors.As(err, &rpcErr) || rpcErr.GRPCCode != 0 {
			t.Errorf("status %s: got %v", status, err)
		}
	}
}

func TestDecodeResponseUserDisplayableError(t *testing.T) {
	raw := chunked(`[["wrb.fr","R7cb6c",null,null,null,[8,null,[["type.googleapis.com/google.internal.labs.tailwind.orchestration.v1.UserDisplayableError",[null,[[1]]]]]],"generic"]]`, trailer)
	for _, allowNull := range []bool{false, true} {
		_, err := DecodeResponse(raw, "R7cb6c", allowNull)
		var rpcErr *RPCError
		if !errors.As(err, &rpcErr) || rpcErr.RPCCode != "USER_DISPLAYABLE_ERROR" {
			t.Fatalf("allowNull=%v: got %v", allowNull, err)
		}
		if rpcErr.GRPCCode != 8 || !strings.Contains(err.Error(), "resource exhausted") {
			t.Errorf("allowNull=%v: got %v (GRPCCode %d)", allowNull, err, rpcErr.GRPCCode)
		}
	}
}

func TestDecodeResponseErFrame(t *testing.T) {
	raw := chunked(`[["er","izAoDd",400]]`, trailer)
	_, err := DecodeResponse(raw, "izAoDd", true)
	if err == nil || !strings.Contains(err.Error(), "invalid request parameters") {
		t.Fatalf("got %v", err)
	}
}

func TestDecodeResponseMethodIDMissing(t *testing.T) {
	raw := chunked(`[["wrb.fr","other1","[1]",null,null,null,"generic"]]`, trailer)
	for _, allowNull := range []bool{false, true} {
		_, err := DecodeResponse(raw, "izAoDd", allowNull)
		if err == nil || !strings.Contains(err.Error(), "method ID may have changed") {
			t.Errorf("allowNull=%v: got %v", allowNull, err)
		}
	}
}

func TestDecodeResponseNoRPCData(t *testing.T) {
	for _, allowNull := range []bool{false, true} {
		_, err := DecodeResponse(chunked(trailer), "izAoDd", allowNull)
		if err == nil || !strings.Contains(err.Error(), "no RPC data") {
			t.Errorf("allowNull=%v: got %v", allowNull, err)
		}
	}
}

func TestDecodeResponseFlatItemChunk(t *testing.T) {
	// A chunk can be a single item rather than a list of items.
	raw := chunked(`["wrb.fr","wXbhsf","[[]]",null,null,null,"generic"]`)
	got, err := DecodeResponse(raw, "wXbhsf", false)
	if err != nil || fmt.Sprint(got) != "[[]]" {
		t.Fatalf("got (%v, %v)", got, err)
	}
}
