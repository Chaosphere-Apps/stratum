package domain

import "encoding/json"

func ValidCanvasSnapshot(snapshot json.RawMessage) bool {
	if len(snapshot) == 0 {
		return false
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(snapshot, &parsed); err != nil {
		return false
	}

	return len(parsed) > 0
}
