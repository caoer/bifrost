package datasheet

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
)

// fleetSheets is the fleet's own model database: one JSON file per provider
// under fleet/, in the same shape as the upstream datasheet. Every payload
// loadPricingFromURL parses gets these rows merged over it, so a provider the
// upstream sheet does not carry (bigmodel — open.bigmodel.cn, an
// Anthropic-compatible surface) is priced and listed in the catalog instead of
// costing zero. On a shared top-level key the fleet row wins.
//
//go:embed fleet/*.json
var fleetSheets embed.FS

// mergeFleetSheets overlays every embedded fleet sheet onto pricingData and
// returns the number of rows merged.
func mergeFleetSheets(pricingData map[string]Entry) (int, error) {
	files, err := fs.ReadDir(fleetSheets, "fleet")
	if err != nil {
		return 0, fmt.Errorf("failed to list fleet datasheets: %w", err)
	}
	merged := 0
	for _, f := range files {
		name := path.Join("fleet", f.Name())
		data, err := fleetSheets.ReadFile(name)
		if err != nil {
			return merged, fmt.Errorf("failed to read fleet datasheet %s: %w", name, err)
		}
		var rows map[string]Entry
		if err := json.Unmarshal(data, &rows); err != nil {
			return merged, fmt.Errorf("failed to unmarshal fleet datasheet %s: %w", name, err)
		}
		for key, entry := range rows {
			pricingData[key] = entry
			merged++
		}
	}
	return merged, nil
}
