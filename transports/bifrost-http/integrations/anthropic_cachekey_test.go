package integrations

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

func cacheKeyContext(sessionIDs ...string) *schemas.BifrostContext {
	ctx := &schemas.BifrostContext{}
	if len(sessionIDs) > 0 && sessionIDs[0] != "" {
		ctx.SetValue(schemas.BifrostContextKeyExtraHeaders, map[string][]string{
			lib.UpstreamSessionIDHeader: sessionIDs,
		})
	}
	return ctx
}

func cacheKeyRequest(existing ...string) *schemas.BifrostResponsesRequest {
	req := &schemas.BifrostResponsesRequest{Params: &schemas.ResponsesParameters{}}
	if len(existing) > 0 && existing[0] != "" {
		key := existing[0]
		req.Params.PromptCacheKey = &key
	}
	return req
}

// A resolved harness session id replaces the user_id-derived cache key, so
// concurrent conversations of one identity stop sharing one cache bucket.
func TestApplyPerConversationCacheKey_SessionIDWins(t *testing.T) {
	req := cacheKeyRequest("user-derived-key")
	applyPerConversationCacheKey(cacheKeyContext("sess-123"), req)
	if req.Params.PromptCacheKey == nil || *req.Params.PromptCacheKey != "sess-123" {
		t.Fatalf("expected session-derived key, got %v", req.Params.PromptCacheKey)
	}
}

// Without a session id the converter's user_id-derived key must stand.
func TestApplyPerConversationCacheKey_NoSessionKeepsUserKey(t *testing.T) {
	req := cacheKeyRequest("user-derived-key")
	applyPerConversationCacheKey(cacheKeyContext(""), req)
	if req.Params.PromptCacheKey == nil || *req.Params.PromptCacheKey != "user-derived-key" {
		t.Fatalf("expected user-derived key preserved, got %v", req.Params.PromptCacheKey)
	}
}

// Keys past 64 chars are hashed, matching the converter's own rule.
func TestApplyPerConversationCacheKey_LongKeyIsHashed(t *testing.T) {
	long := make([]byte, 65)
	for i := range long {
		long[i] = 'a'
	}
	req := cacheKeyRequest("user-derived-key")
	applyPerConversationCacheKey(cacheKeyContext(string(long)), req)
	want := fmt.Sprintf("%x", sha256.Sum256(long))
	if req.Params.PromptCacheKey == nil || *req.Params.PromptCacheKey != want {
		t.Fatalf("expected hashed key %s, got %v", want, req.Params.PromptCacheKey)
	}
	if len(*req.Params.PromptCacheKey) != 64 {
		t.Fatalf("expected 64-char hash, got %d", len(*req.Params.PromptCacheKey))
	}
}

// A nil Params request (no parameters at all) must not panic.
func TestApplyPerConversationCacheKey_NilParamsNoPanic(t *testing.T) {
	applyPerConversationCacheKey(cacheKeyContext("sess-123"), &schemas.BifrostResponsesRequest{})
	applyPerConversationCacheKey(nil, nil)
}
