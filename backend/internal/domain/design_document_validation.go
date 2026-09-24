package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var aiComponentTypes = map[string]bool{
	"client.web": true, "edge.api_gateway": true, "compute.service": true,
	"data.sql_database": true, "data.redis": true, "messaging.queue": true,
	"data.object_store": true, "ai.llm": true, "external.api": true,
	"observability.telemetry": true, "security.control": true,
	"design.link": true, "note.sticky": true, "frame.cloud": true,
}

var aiConnectorTypes = map[string]bool{
	"synchronous": true, "asynchronous_event": true, "batch_transfer": true,
	"cache_read": true, "cache_write": true, "model_call": true,
	"observability_signal": true,
}

type aiCanvasDocument struct {
	SchemaVersion string              `json:"schemaVersion"`
	ID            string              `json:"id"`
	Title         string              `json:"title"`
	UpdatedAt     string              `json:"updatedAt"`
	Components    []aiCanvasComponent `json:"components"`
	Connectors    []aiCanvasConnector `json:"connectors"`
	Journeys      []aiCanvasJourney   `json:"journeys"`
}

type aiCanvasComponent struct {
	ID          string          `json:"id"`
	ShapeID     string          `json:"shapeId"`
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Purpose     string          `json:"purpose"`
	Owner       string          `json:"owner"`
	Criticality string          `json:"criticality"`
	Metadata    json.RawMessage `json:"metadata"`
	Notes       json.RawMessage `json:"notes"`
}

type aiCanvasConnector struct {
	ID                     string `json:"id"`
	FromComponentID        string `json:"fromComponentId"`
	ToComponentID          string `json:"toComponentId"`
	Type                   string `json:"type"`
	Protocol               string `json:"protocol"`
	TimeoutMs              *int   `json:"timeoutMs"`
	ConsistencyExpectation string `json:"consistencyExpectation"`
	Notes                  string `json:"notes"`
	Animated               bool   `json:"animated"`
}

type aiCanvasJourney struct {
	ID               string          `json:"id"`
	Title            string          `json:"title"`
	Description      string          `json:"description"`
	EntryComponentID string          `json:"entryComponentId"`
	CreatedAt        string          `json:"createdAt"`
	UpdatedAt        string          `json:"updatedAt"`
	Steps            json.RawMessage `json:"steps"`
}

// ValidateAIDesignUpdate additionally binds an AI proposal to the working
// design's identity. The canvas shape itself is checked for every API write.
func ValidateAIDesignUpdate(current json.RawMessage, proposed json.RawMessage, maxBytes int64) error {
	if err := ValidateDesignDocumentUpdate(current, proposed, maxBytes); err != nil {
		return err
	}
	var before, after struct {
		SchemaVersion string `json:"schemaVersion"`
		ID            string `json:"id"`
	}
	if json.Unmarshal(current, &before) != nil || json.Unmarshal(proposed, &after) != nil || before.ID != after.ID || before.SchemaVersion != after.SchemaVersion {
		return errors.New("schema version and design identity must be preserved")
	}
	return nil
}

// ValidateDesignDocumentUpdate permits a legacy document with missing identity
// fields to be repaired, but never permits changing identity fields already set.
func ValidateDesignDocumentUpdate(current json.RawMessage, proposed json.RawMessage, maxBytes int64) error {
	if err := ValidateDesignDocument(proposed, maxBytes); err != nil {
		return err
	}
	var before, after struct {
		SchemaVersion string `json:"schemaVersion"`
		ID            string `json:"id"`
	}
	if json.Unmarshal(current, &before) != nil || json.Unmarshal(proposed, &after) != nil {
		return errors.New("design identity could not be read")
	}
	if (before.ID != "" && before.ID != after.ID) || (before.SchemaVersion != "" && before.SchemaVersion != after.SchemaVersion) {
		return errors.New("design identity and schema version cannot change")
	}
	return nil
}

// ValidateDesignDocument rejects JSON that cannot be interpreted as Stratum's
// supported React Flow canvas document. Unknown extra fields are preserved for
// forward compatibility, while all fields consumed by the UI are type-checked.
func ValidateDesignDocument(proposed json.RawMessage, maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = 4 << 20
	}
	if int64(len(proposed)) > maxBytes || len(proposed) == 0 || !json.Valid(proposed) {
		return errors.New("design document is invalid or exceeds the configured size limit")
	}
	if !jsonObject(proposed) {
		return errors.New("design document must be a JSON object")
	}
	var after aiCanvasDocument
	if json.Unmarshal(proposed, &after) != nil {
		return errors.New("design document does not match the structured schema")
	}
	if strings.TrimSpace(after.ID) == "" {
		return errors.New("design identity is required")
	}
	if after.SchemaVersion != "sde-ui/v0.1" {
		return errors.New("unsupported design document schema version")
	}
	if strings.TrimSpace(after.Title) == "" || utf8.RuneCountInString(after.Title) > MaxDesignNameLength {
		return errors.New("design title is required and must be at most 120 characters")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(proposed, &fields); err != nil || !jsonObject(fields["requirementBrief"]) || !jsonArray(fields["components"]) || !jsonArray(fields["connectors"]) || !jsonArray(fields["journeys"]) {
		return errors.New("design document is missing required structured fields")
	}
	if err := validateAIRequirementBrief(fields["requirementBrief"]); err != nil {
		return err
	}
	if len(after.Components) > 500 || len(after.Connectors) > 1000 || len(after.Journeys) > 100 {
		return errors.New("design update exceeds graph limits")
	}
	componentTypes := make(map[string]string, len(after.Components))
	shapeIDs := make(map[string]bool, len(after.Components))
	for _, component := range after.Components {
		if strings.TrimSpace(component.ID) == "" || len(component.ID) > 128 || componentTypes[component.ID] != "" || strings.TrimSpace(component.ShapeID) == "" || shapeIDs[component.ShapeID] {
			return errors.New("components must have unique IDs and shape IDs")
		}
		if !aiComponentTypes[component.Type] || strings.TrimSpace(component.Name) == "" || len(component.Name) > 200 || len(component.Purpose) > 2000 || len(component.Owner) > 200 {
			return fmt.Errorf("component %q has an invalid type or display fields", component.ID)
		}
		if !map[string]bool{"low": true, "medium": true, "high": true, "critical": true}[component.Criticality] {
			return fmt.Errorf("component %q has an invalid criticality", component.ID)
		}
		if !jsonObject(component.Metadata) || !jsonArray(component.Notes) {
			return fmt.Errorf("component %q has invalid metadata or notes", component.ID)
		}
		if err := validateAIMetadata(component.Metadata); err != nil {
			return fmt.Errorf("component %q: %w", component.ID, err)
		}
		var notes []struct {
			ID   string `json:"id"`
			Body string `json:"body"`
			Tone string `json:"tone"`
		}
		if err := json.Unmarshal(component.Notes, &notes); err != nil || len(notes) > 100 {
			return fmt.Errorf("component %q has invalid notes", component.ID)
		}
		for _, note := range notes {
			if note.ID == "" || len(note.Body) > 8000 || !map[string]bool{"neutral": true, "risk": true, "decision": true, "question": true}[note.Tone] {
				return fmt.Errorf("component %q has an invalid note", component.ID)
			}
		}
		componentTypes[component.ID] = component.Type
		shapeIDs[component.ShapeID] = true
	}
	for _, component := range after.Components {
		var metadata struct {
			ParentFrameID string `json:"parentFrameId"`
		}
		_ = json.Unmarshal(component.Metadata, &metadata)
		if metadata.ParentFrameID != "" && (metadata.ParentFrameID == component.ID || componentTypes[metadata.ParentFrameID] != "frame.cloud") {
			return fmt.Errorf("component %q references an invalid parent frame", component.ID)
		}
	}
	connectorIDs := map[string]bool{}
	for _, connector := range after.Connectors {
		if strings.TrimSpace(connector.ID) == "" || len(connector.ID) > 128 || connectorIDs[connector.ID] || componentTypes[connector.FromComponentID] == "" || componentTypes[connector.ToComponentID] == "" {
			return errors.New("connectors must have unique IDs and reference existing components")
		}
		if !aiConnectorTypes[connector.Type] || len(connector.Protocol) > 200 || len(connector.ConsistencyExpectation) > 500 || len(connector.Notes) > 2000 || (connector.TimeoutMs != nil && (*connector.TimeoutMs < 0 || *connector.TimeoutMs > 86400000)) {
			return fmt.Errorf("connector %q has invalid fields", connector.ID)
		}
		connectorIDs[connector.ID] = true
	}
	journeyIDs := map[string]bool{}
	for _, journey := range after.Journeys {
		if journey.ID == "" || journeyIDs[journey.ID] || strings.TrimSpace(journey.Title) == "" || !jsonArray(journey.Steps) {
			return errors.New("journeys must have unique IDs, titles, and steps")
		}
		if journey.EntryComponentID != "" && componentTypes[journey.EntryComponentID] == "" {
			return fmt.Errorf("journey %q references a missing entry component", journey.ID)
		}
		var steps []struct {
			ID              string `json:"id"`
			Title           string `json:"title"`
			Description     string `json:"description"`
			Kind            string `json:"kind"`
			ComponentID     string `json:"componentId"`
			ConnectorID     string `json:"connectorId"`
			FromComponentID string `json:"fromComponentId"`
			ToComponentID   string `json:"toComponentId"`
		}
		if err := json.Unmarshal(journey.Steps, &steps); err != nil || len(steps) > 200 {
			return fmt.Errorf("journey %q has invalid steps", journey.ID)
		}
		stepIDs := map[string]bool{}
		for _, step := range steps {
			if step.ID == "" || stepIDs[step.ID] || strings.TrimSpace(step.Title) == "" ||
				(step.Kind != "" && !map[string]bool{"component": true, "request": true, "async": true, "callback": true, "batch": true, "signal": true, "decision": true}[step.Kind]) ||
				(step.ComponentID != "" && componentTypes[step.ComponentID] == "") ||
				(step.ConnectorID != "" && !connectorIDs[step.ConnectorID]) ||
				(step.FromComponentID != "" && componentTypes[step.FromComponentID] == "") ||
				(step.ToComponentID != "" && componentTypes[step.ToComponentID] == "") {
				return fmt.Errorf("journey %q has an invalid step", journey.ID)
			}
			stepIDs[step.ID] = true
		}
		journeyIDs[journey.ID] = true
	}
	return nil
}

func validateAIRequirementBrief(raw json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return errors.New("requirements must be an object")
	}
	// The journey editor calls useCase.trim() directly; accepting a missing or
	// null value here would persist a document that can crash that view.
	if _, exists := fields["useCase"]; !exists {
		return errors.New("requirement useCase is required")
	}
	for _, key := range []string{"useCase", "functionalRequirements", "nonFunctionalRequirements", "availabilityRequirement", "sla", "problemStatement", "primaryActors", "trafficNotes", "consistencyNotes", "openQuestions"} {
		if value, exists := fields[key]; exists {
			var text string
			if err := json.Unmarshal(value, &text); err != nil || string(value) == "null" || len(text) > 20000 {
				return fmt.Errorf("requirement %q must be text within the size limit", key)
			}
		}
	}
	if value, exists := fields["targetRps"]; exists && string(value) != "null" {
		var number float64
		if err := json.Unmarshal(value, &number); err != nil || number < 0 || number > 1000000000000 {
			return errors.New("target RPS must be a nonnegative number")
		}
	}
	return nil
}

func validateAIMetadata(raw json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return errors.New("metadata must be an object")
	}
	for _, key := range []string{"position", "size"} {
		value, exists := fields[key]
		if !exists {
			continue
		}
		if !jsonObject(value) {
			return fmt.Errorf("%s must be an object", key)
		}
		var coordinates map[string]float64
		if err := json.Unmarshal(value, &coordinates); err != nil {
			return fmt.Errorf("%s has invalid dimensions", key)
		}
		first, second := "x", "y"
		if key == "size" {
			first, second = "width", "height"
		}
		if _, ok := coordinates[first]; !ok {
			return fmt.Errorf("%s is incomplete", key)
		}
		if _, ok := coordinates[second]; !ok {
			return fmt.Errorf("%s is incomplete", key)
		}
		for _, number := range []float64{coordinates[first], coordinates[second]} {
			if number < -100000 || number > 100000 {
				return fmt.Errorf("%s is outside canvas bounds", key)
			}
		}
		if key == "size" && (coordinates[first] <= 0 || coordinates[second] <= 0) {
			return errors.New("size must be positive")
		}
	}
	for _, key := range []string{"redis", "linkedDesign", "enterpriseAsset"} {
		if value, exists := fields[key]; exists && !jsonObject(value) {
			return fmt.Errorf("%s must be an object", key)
		}
	}
	if value, exists := fields["linkedDesign"]; exists {
		var linked struct {
			WorkspaceID string `json:"workspaceId"`
			DesignID    string `json:"designId"`
			Title       string `json:"title"`
			Access      string `json:"access"`
			UpdatedAt   string `json:"updatedAt"`
		}
		if err := json.Unmarshal(value, &linked); err != nil || (linked.Access != "" && !map[string]bool{"private": true, "workspace": true, "public": true}[linked.Access]) {
			return errors.New("linked design metadata is invalid")
		}
	}
	if value, exists := fields["enterpriseAsset"]; exists {
		var asset struct {
			AssetID            string `json:"assetId"`
			Name               string `json:"name"`
			Kind               string `json:"kind"`
			Type               string `json:"type"`
			Owner              string `json:"owner"`
			Criticality        string `json:"criticality"`
			Status             string `json:"status"`
			ReplacementAssetID string `json:"replacementAssetId"`
			UpdateMessage      string `json:"updateMessage"`
			LinkedAt           string `json:"linkedAt"`
		}
		if err := json.Unmarshal(value, &asset); err != nil {
			return errors.New("enterprise asset metadata is invalid")
		}
	}
	if value, exists := fields["redis"]; exists {
		var redis struct {
			Usage                  string   `json:"usage"`
			ClusterMode            string   `json:"clusterMode"`
			Replication            string   `json:"replication"`
			Persistence            string   `json:"persistence"`
			EvictionPolicy         string   `json:"evictionPolicy"`
			ConsistencyExpectation string   `json:"consistencyExpectation"`
			FailoverBehavior       string   `json:"failoverBehavior"`
			BackupRestore          string   `json:"backupRestore"`
			MemoryLimitGb          *float64 `json:"memoryLimitGb"`
			ExpectedQps            *float64 `json:"expectedQps"`
			HotKeyRisk             string   `json:"hotKeyRisk"`
		}
		if err := json.Unmarshal(value, &redis); err != nil || (redis.MemoryLimitGb != nil && *redis.MemoryLimitGb < 0) || (redis.ExpectedQps != nil && *redis.ExpectedQps < 0) {
			return errors.New("Redis metadata is invalid")
		}
	}
	for _, key := range []string{"parentFrameId", "notes"} {
		if value, exists := fields[key]; exists {
			var text string
			if err := json.Unmarshal(value, &text); err != nil || string(value) == "null" {
				return fmt.Errorf("%s must be text", key)
			}
		}
	}
	for _, key := range []string{"expectedQps", "latencyBudgetMs"} {
		if value, exists := fields[key]; exists && string(value) != "null" {
			var number float64
			if err := json.Unmarshal(value, &number); err != nil || number < 0 || number > 1000000000000 {
				return fmt.Errorf("%s must be a nonnegative number", key)
			}
		}
	}
	return nil
}

func jsonObject(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) > 0 && trimmed[0] == '{'
}
func jsonArray(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) > 0 && trimmed[0] == '['
}
