package datasheet

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
)

const (
	bigmodel = schemas.ModelProvider("bigmodel")
	// Second route to the same upstream, reached over the fleet's CN-egress
	// SOCKS5 rather than the relay. Pricing is keyed by (provider, model), so
	// this provider needs rows of its own or half the GLM traffic logs at zero
	// cost while looking healthy.
	bigmodelCN = schemas.ModelProvider("bigmodel-cn")
)

// TestFleetSheetsRideEveryLoad: a sheet that knows nothing about bigmodel
// still yields the fleet's bigmodel rows, priced from fleet/bigmodel.json.
func TestFleetSheetsRideEveryLoad(t *testing.T) {
	store := New(nil, bifrost.NewDefaultLogger(schemas.LogLevelWarn), Config{
		URL: fileURL(t, "testdata/pricing.json"),
	})
	if err := store.LoadFromURLIntoMemory(context.Background()); err != nil {
		t.Fatalf("LoadFromURLIntoMemory: %v", err)
	}

	// The upstream rows are untouched.
	if store.GetPricingEntryForModel("gpt-4o", schemas.OpenAI) == nil {
		t.Fatal("expected gpt-4o from the loaded sheet")
	}

	// Both routes to the upstream carry the same prices. bigmodel-cn is listed
	// too because a provider absent from the sheet costs zero silently.
	for _, provider := range []schemas.ModelProvider{bigmodel, bigmodelCN} {
		for model, want := range map[string]float64{
			"glm-5.3":       1.4e-06,
			"glm-5.3-flash": 7.5e-08,
		} {
			p := store.GetPricingEntryForModel(model, provider)
			if p == nil {
				t.Fatalf("expected fleet pricing for %s/%s, got nil", provider, model)
			}
			if p.InputCostPerToken == nil || *p.InputCostPerToken != want {
				t.Fatalf("%s/%s input_cost_per_token: want %g, got %#v", provider, model, want, p.InputCostPerToken)
			}
			if p.CacheReadInputTokenCost == nil {
				t.Fatalf("%s/%s: expected cache_read_input_token_cost", provider, model)
			}
		}
	}
}

// TestFleetSheetsWinOnCollision: when the URL sheet carries the same
// top-level key, the fleet row is the one that lands.
func TestFleetSheetsWinOnCollision(t *testing.T) {
	sheet := filepath.Join(t.TempDir(), "pricing.json")
	if err := os.WriteFile(sheet, []byte(`{
		"bigmodel/glm-5.3": {"provider": "bigmodel", "mode": "chat", "input_cost_per_token": 9.9, "output_cost_per_token": 9.9}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := New(nil, bifrost.NewDefaultLogger(schemas.LogLevelWarn), Config{URL: "file://" + sheet})
	if err := store.LoadFromURLIntoMemory(context.Background()); err != nil {
		t.Fatalf("LoadFromURLIntoMemory: %v", err)
	}
	p := store.GetPricingEntryForModel("glm-5.3", bigmodel)
	if p == nil || p.InputCostPerToken == nil || *p.InputCostPerToken != 1.4e-06 {
		t.Fatalf("expected the fleet row (1.4e-06) to win over the URL row, got %#v", p)
	}
}
