package bifrost

import (
	"context"
	"testing"
	"time"

	schemas "github.com/maximhq/bifrost/core/schemas"
)

// A primary attempt that already ran past fallbackPrimaryAttemptBudget failed
// at the model, not on the path; a fallback would repeat that wait.
func TestShouldTryFallbacksHonoursPrimaryAttemptBudget(t *testing.T) {
	b := &Bifrost{logger: NewNoOpLogger()}
	req := &schemas.BifrostRequest{ChatRequest: &schemas.BifrostChatRequest{
		Provider:  "bigmodel",
		Model:     "glm",
		Fallbacks: []schemas.Fallback{{Provider: "bigmodel-cn", Model: "glm"}},
	}}
	primaryErr := &schemas.BifrostError{Error: &schemas.ErrorField{Message: "upstream timed out"}}

	quick := schemas.NewBifrostContext(context.Background(), time.Time{})
	quick.SetValue(schemas.BifrostContextKeyRequestStartTime, time.Now())
	if !b.shouldTryFallbacks(quick, req, primaryErr) {
		t.Fatal("a primary failure within the budget must still try the fallbacks")
	}

	slow := schemas.NewBifrostContext(context.Background(), time.Time{})
	slow.SetValue(schemas.BifrostContextKeyRequestStartTime, time.Now().Add(-fallbackPrimaryAttemptBudget-time.Second))
	if b.shouldTryFallbacks(slow, req, primaryErr) {
		t.Fatal("a primary failure past the budget must return the primary error instead of trying fallbacks")
	}
}
