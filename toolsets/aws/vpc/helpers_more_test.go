package awsvpc

import (
	"encoding/json"
	"testing"
)

func TestVPCTypeHelpers(t *testing.T) {
	if got := toInt(json.Number("9"), 1); got != 9 {
		t.Fatalf("unexpected toInt: %d", got)
	}
	filters := tagFiltersFromArgs(map[string]any{
		"":      []string{"skip"},
		"env":   "dev",
		"empty": []string{""},
	})
	if len(filters) != 2 {
		t.Fatalf("expected two tag filters, got %#v", filters)
	}
}
