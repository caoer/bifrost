package datasheet

import (
	"context"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A custom provider backed by the built-in anthropic type, as core reports it.
const customAnthropic = schemas.ModelProvider("dewiest_gulp")

func claudePricing() configstoreTables.TableModelPricing {
	return configstoreTables.TableModelPricing{
		Model:                   "claude-opus-4-7",
		Provider:                "anthropic",
		Mode:                    "chat",
		InputCostPerToken:       bifrost.Ptr(5e-06),
		OutputCostPerToken:      bifrost.Ptr(2.5e-05),
		CacheReadInputTokenCost: bifrost.Ptr(5e-07),
	}
}

// TestCustomProviderPricedAsBaseProvider: a custom provider with no rows of its
// own costs exactly what provider anthropic costs, on the response path
// (RoutingInfo.BaseProvider) and on the bare-usage path (scopes from ctx), and
// a provider-scoped override keyed on the custom name still applies.
func TestCustomProviderPricedAsBaseProvider(t *testing.T) {
	s := testStoreWithPricing(map[string]configstoreTables.TableModelPricing{
		makeKey("claude-opus-4-7", "anthropic", "chat"): claudePricing(),
	})
	usage := &schemas.BifrostLLMUsage{
		PromptTokens:        1200,
		CompletionTokens:    300,
		TotalTokens:         1500,
		PromptTokensDetails: &schemas.ChatPromptTokensDetails{CachedReadTokens: 800},
	}

	want := s.CalculateCost(makeChatResponse(schemas.Anthropic, "claude-opus-4-7", usage), nil)
	require.Greater(t, want, 0.0)

	resp := makeChatResponse(customAnthropic, "claude-opus-4-7", usage)
	resp.ChatResponse.ExtraFields.RoutingInfo.BaseProvider = schemas.Anthropic
	assert.Equal(t, want, s.CalculateCost(resp, nil), "response path")

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	ctx.SetValue(schemas.BifrostContextKeyBaseProviderType, schemas.Anthropic)
	scopes := LookupScopesFromContext(ctx, string(customAnthropic))
	assert.Equal(t, "anthropic", scopes.BaseProvider)
	assert.Equal(t, want, s.CalculateCostForUsage(usage, customAnthropic, "claude-opus-4-7", schemas.ChatCompletionStreamRequest, scopes), "bare-usage path")

	providerID := string(customAnthropic)
	require.NoError(t, s.SetOverrides([]configstoreTables.TablePricingOverride{{
		ID:               "custom-override",
		ScopeKind:        string(ScopeKindProvider),
		ProviderID:       &providerID,
		MatchType:        string(MatchTypeExact),
		Pattern:          "claude-opus-4-7",
		RequestTypes:     []schemas.RequestType{schemas.ChatCompletionRequest},
		PricingPatchJSON: `{"input_cost_per_token":1}`,
	}}))
	pricing := s.resolvePricing(resp.ChatResponse.ExtraFields.RoutingInfo, schemas.ChatCompletionRequest, LookupScopes{})
	require.NotNil(t, pricing)
	assert.Equal(t, 1.0, *pricing.InputCostPerToken, "override on the custom name")
	assert.Equal(t, 2.5e-05, *pricing.OutputCostPerToken, "base rows under the override")
}

// TestCustomProviderOwnRowsWin: a custom provider with datasheet rows of its
// own (the fleet sheet's bigmodel/*) keeps them over its base type's rows.
func TestCustomProviderOwnRowsWin(t *testing.T) {
	own := chatPricing(1.4e-06, 4.4e-06)
	s := testStoreWithPricing(map[string]configstoreTables.TableModelPricing{
		makeKey("glm-5.3", "bigmodel", "chat"): own,
		makeKey("glm-5.3", "openai", "chat"):   chatPricing(9.9, 9.9),
	})
	pricing := s.resolvePricing(schemas.RoutingInfo{Provider: bigmodel, BaseProvider: schemas.OpenAI, Model: "glm-5.3"}, schemas.ChatCompletionRequest, LookupScopes{})
	require.NotNil(t, pricing)
	assert.Equal(t, 1.4e-06, *pricing.InputCostPerToken)
}

// TestBuiltInProviderNeverFallsBack: a built-in provider prices from its own
// rows only, whatever base type the routing info or scopes carry.
func TestBuiltInProviderNeverFallsBack(t *testing.T) {
	s := testStoreWithPricing(map[string]configstoreTables.TableModelPricing{
		makeKey("claude-opus-4-7", "anthropic", "chat"): claudePricing(),
	})
	assert.Nil(t, s.resolvePricing(schemas.RoutingInfo{Provider: schemas.OpenAI, BaseProvider: schemas.Anthropic, Model: "claude-opus-4-7"}, schemas.ChatCompletionRequest, LookupScopes{}))
	assert.Nil(t, s.resolvePricing(schemas.RoutingInfo{Provider: schemas.OpenAI, Model: "claude-opus-4-7"}, schemas.ChatCompletionRequest, LookupScopes{Provider: "openai", BaseProvider: "anthropic"}))
}
