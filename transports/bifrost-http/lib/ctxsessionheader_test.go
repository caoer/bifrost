package lib

import (
	"testing"

	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/valyala/fasthttp"
)

// outboundSessionHeader runs the real wiring end to end: it builds a Bifrost
// context from an inbound request the way the transport does, then applies that
// context to a fresh outbound request through the same helper every provider
// uses. It returns what a provider would actually put on the wire.
func outboundSessionHeader(t *testing.T, inbound map[string]string) string {
	t.Helper()

	ctx := &fasthttp.RequestCtx{}
	for k, v := range inbound {
		ctx.Request.Header.Set(k, v)
	}

	bifrostCtx, cancel := ConvertToBifrostContext(ctx, testHandlerStore{})
	if bifrostCtx == nil {
		t.Fatal("expected non-nil bifrost context")
	}
	defer cancel()

	outbound := &fasthttp.Request{}
	providerUtils.SetExtraHeaders(bifrostCtx, outbound, nil, nil)
	return string(outbound.Header.Peek(UpstreamSessionIDHeader))
}

// TestOutboundSessionHeader_FromClaudeCodeHarnessHeader is the case that matters
// in production: Claude Code sends its own session header, and the provider
// request must carry it as `session_id` so a Codex-protocol gateway keys its
// conversation cache on one stable identity instead of minting a fresh random
// session for every turn.
func TestOutboundSessionHeader_FromClaudeCodeHarnessHeader(t *testing.T) {
	got := outboundSessionHeader(t, map[string]string{
		"x-claude-code-session-id": "e33324d2-65dc-468a-866c-d6ea764a39e9",
	})
	if got != "e33324d2-65dc-468a-866c-d6ea764a39e9" {
		t.Fatalf("expected the harness session id on the outbound request, got %q", got)
	}
}

// TestOutboundSessionHeader_FromExplicitBifrostHeader covers the documented
// precedence: x-bf-session-id decides the session whenever the caller sends one.
func TestOutboundSessionHeader_FromExplicitBifrostHeader(t *testing.T) {
	got := outboundSessionHeader(t, map[string]string{
		"x-bf-session-id":          "explicit-wins",
		"x-claude-code-session-id": "harness-loses",
	})
	if got != "explicit-wins" {
		t.Fatalf("expected x-bf-session-id to win, got %q", got)
	}
}

// TestOutboundSessionHeader_AbsentWhenNoSessionID is the negative case: a request
// carrying no session identifier at all must send no session_id header, rather
// than an empty one that an upstream could bind a cache to.
func TestOutboundSessionHeader_AbsentWhenNoSessionID(t *testing.T) {
	if got := outboundSessionHeader(t, map[string]string{
		"content-type": "application/json",
	}); got != "" {
		t.Fatalf("expected no session_id header without a session id, got %q", got)
	}
}

// TestOutboundSessionHeader_ExplicitExtraHeaderOverrideWins verifies the escape
// hatch: a caller that sets x-bf-eh-session_id is deliberately overriding this
// exact header, so the resolved session id must not clobber it.
func TestOutboundSessionHeader_ExplicitExtraHeaderOverrideWins(t *testing.T) {
	got := outboundSessionHeader(t, map[string]string{
		"x-claude-code-session-id": "resolved-session",
		"x-bf-eh-session_id":       "caller-override",
	})
	if got != "caller-override" {
		t.Fatalf("expected the explicit x-bf-eh-session_id override to win, got %q", got)
	}
}

// TestOutboundSessionHeader_OversizedSessionIDSendsNothing pins the interaction
// with NormalizeSessionID: an oversized identifier is rejected rather than
// truncated, and a rejected id must not reach the provider.
func TestOutboundSessionHeader_OversizedSessionIDSendsNothing(t *testing.T) {
	oversized := make([]byte, schemas.MaxSessionIDLength+1)
	for i := range oversized {
		oversized[i] = 'a'
	}
	if got := outboundSessionHeader(t, map[string]string{
		"x-bf-session-id": string(oversized),
	}); got != "" {
		t.Fatalf("expected no session_id header for a rejected oversized id, got %q", got)
	}
}

// TestConvertToBifrostContext_SessionIDReachesExtraHeaders asserts the wiring
// point itself, so a regression in the context plumbing is distinguishable from
// a regression in SetExtraHeaders.
func TestConvertToBifrostContext_SessionIDReachesExtraHeaders(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("session-id", "codex-cli-session")

	bifrostCtx, cancel := ConvertToBifrostContext(ctx, testHandlerStore{})
	if bifrostCtx == nil {
		t.Fatal("expected non-nil bifrost context")
	}
	defer cancel()

	if got, _ := bifrostCtx.Value(schemas.BifrostContextKeySessionID).(string); got != "codex-cli-session" {
		t.Fatalf("expected session id on the context, got %q", got)
	}

	extra, ok := bifrostCtx.Value(schemas.BifrostContextKeyExtraHeaders).(map[string][]string)
	if !ok {
		t.Fatal("expected extra headers map on the context")
	}
	values := extra[UpstreamSessionIDHeader]
	if len(values) != 1 || values[0] != "codex-cli-session" {
		t.Fatalf("expected extra headers to carry the session id once, got %v", values)
	}
}
