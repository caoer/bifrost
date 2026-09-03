package integrations

import (
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
)

func roleMessage(role schemas.ResponsesMessageRoleType) schemas.ResponsesMessage {
	return schemas.ResponsesMessage{
		Type: schemas.Ptr(schemas.ResponsesMessageTypeMessage),
		Role: schemas.Ptr(role),
	}
}

func roles(req *schemas.BifrostResponsesRequest) []string {
	out := make([]string, 0, len(req.Input))
	for _, item := range req.Input {
		if item.Role == nil {
			out = append(out, "-")
			continue
		}
		out = append(out, string(*item.Role))
	}
	return out
}

func assertRoles(t *testing.T, req *schemas.BifrostResponsesRequest, want ...string) {
	t.Helper()
	got := roles(req)
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

// A system message after the first non-system item becomes developer; the
// leading system message — the conversation's instructions — stays system.
func TestRelabelMidConversationSystemMessages_MidConversationBecomesDeveloper(t *testing.T) {
	req := &schemas.BifrostResponsesRequest{Input: []schemas.ResponsesMessage{
		roleMessage(schemas.ResponsesInputMessageRoleSystem),
		roleMessage(schemas.ResponsesInputMessageRoleUser),
		roleMessage(schemas.ResponsesInputMessageRoleSystem),
		{Type: schemas.Ptr(schemas.ResponsesMessageTypeReasoning)},
		roleMessage(schemas.ResponsesInputMessageRoleAssistant),
		roleMessage(schemas.ResponsesInputMessageRoleSystem),
	}}
	relabelMidConversationSystemMessages(req)
	assertRoles(t, req, "system", "user", "developer", "-", "assistant", "developer")
}

// Several leading system messages are all instructions.
func TestRelabelMidConversationSystemMessages_LeadingRunStaysSystem(t *testing.T) {
	req := &schemas.BifrostResponsesRequest{Input: []schemas.ResponsesMessage{
		roleMessage(schemas.ResponsesInputMessageRoleSystem),
		roleMessage(schemas.ResponsesInputMessageRoleSystem),
		roleMessage(schemas.ResponsesInputMessageRoleUser),
		roleMessage(schemas.ResponsesInputMessageRoleSystem),
	}}
	relabelMidConversationSystemMessages(req)
	assertRoles(t, req, "system", "system", "user", "developer")
}

// A conversation with no mid-conversation system message is untouched.
func TestRelabelMidConversationSystemMessages_NoopWithoutMidSystem(t *testing.T) {
	req := &schemas.BifrostResponsesRequest{Input: []schemas.ResponsesMessage{
		roleMessage(schemas.ResponsesInputMessageRoleSystem),
		roleMessage(schemas.ResponsesInputMessageRoleUser),
		roleMessage(schemas.ResponsesInputMessageRoleAssistant),
		roleMessage(schemas.ResponsesInputMessageRoleDeveloper),
	}}
	relabelMidConversationSystemMessages(req)
	assertRoles(t, req, "system", "user", "assistant", "developer")
}

// Nil request and empty input do not panic.
func TestRelabelMidConversationSystemMessages_NilAndEmpty(t *testing.T) {
	relabelMidConversationSystemMessages(nil)
	req := &schemas.BifrostResponsesRequest{}
	relabelMidConversationSystemMessages(req)
	if len(req.Input) != 0 {
		t.Fatalf("expected empty input, got %d items", len(req.Input))
	}
}
