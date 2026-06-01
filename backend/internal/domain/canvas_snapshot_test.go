package domain

import (
	"encoding/json"
	"testing"
)

func TestValidCanvasSnapshot(t *testing.T) {
	for _, test := range []struct {
		name     string
		snapshot json.RawMessage
		want     bool
	}{
		{"empty", nil, false},
		{"invalid json", json.RawMessage(`{`), false},
		{"null", json.RawMessage(`null`), false},
		{"array", json.RawMessage(`[]`), false},
		{"empty object", json.RawMessage(`{}`), false},
		{"object", json.RawMessage(`{"viewport":{"x":0,"y":0,"zoom":1}}`), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ValidCanvasSnapshot(test.snapshot); got != test.want {
				t.Fatalf("ValidCanvasSnapshot(%s) = %v, want %v", string(test.snapshot), got, test.want)
			}
		})
	}
}
