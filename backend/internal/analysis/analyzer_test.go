package analysis

import (
	"encoding/json"
	"testing"
)

func TestAnalyzeFindsMissingSignals(t *testing.T) {
	report, err := New().Analyze(json.RawMessage(`{
		"schemaVersion": "sde-ui/v0.1",
		"id": "design_1",
		"title": "Empty",
		"requirementBrief": {},
		"components": [],
		"connectors": []
	}`))
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if report.Score >= 80 {
		t.Fatalf("score = %d, want below 80 for empty design", report.Score)
	}
	assertFinding(t, report, "requirements", "Use case is missing")
	assertFinding(t, report, "topology", "No architecture components modeled")
}

func TestAnalyzeIgnoresFramesForComponentCount(t *testing.T) {
	report, err := New().Analyze(json.RawMessage(`{
		"schemaVersion": "sde-ui/v0.1",
		"id": "design_1",
		"title": "Frame only",
		"requirementBrief": {
			"useCase": "Model a cloud boundary",
			"functionalRequirements": "Sketch system boundaries",
			"targetRps": 50,
			"availabilityRequirement": "99.9%",
			"sla": "1s",
			"consistencyNotes": "eventual"
		},
		"components": [
			{"id": "frame_1", "type": "frame.cloud", "name": "Cloud", "metadata": {}}
		],
		"connectors": []
	}`))
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if report.Signals["components"] != 0 {
		t.Fatalf("real component signal = %v, want 0", report.Signals["components"])
	}
	assertFinding(t, report, "topology", "No architecture components modeled")
}

func TestAnalyzeFindsSyncCycleAndSecurityBoundary(t *testing.T) {
	report, err := New().Analyze(json.RawMessage(`{
		"schemaVersion": "sde-ui/v0.1",
		"id": "design_1",
		"title": "Cycle",
		"requirementBrief": {
			"useCase": "Users submit payment requests",
			"functionalRequirements": "Create and approve requests",
			"nonFunctionalRequirements": "Multi-AZ replicas",
			"targetRps": 1200,
			"availabilityRequirement": "99.99%",
			"sla": "400ms",
			"consistencyNotes": "strong read-after-write consistency"
		},
		"components": [
			{"id": "client", "type": "client.web", "name": "Web", "criticality": "medium", "metadata": {}},
			{"id": "svc", "type": "compute.service", "name": "Service", "criticality": "critical", "metadata": {}},
			{"id": "db", "type": "data.sql_database", "name": "DB", "criticality": "critical", "metadata": {}}
		],
		"connectors": [
			{"id": "c1", "fromComponentId": "client", "toComponentId": "svc", "type": "synchronous", "protocol": "http"},
			{"id": "c2", "fromComponentId": "svc", "toComponentId": "db", "type": "synchronous"},
			{"id": "c3", "fromComponentId": "db", "toComponentId": "svc", "type": "synchronous"}
		]
	}`))
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	assertFinding(t, report, "topology", "Synchronous cycle detected")
	assertFinding(t, report, "security", "Trust boundary lacks security control")
	assertFinding(t, report, "traffic", "High traffic path lacks buffering or cache")
	if report.Signals["synchronousConnectors"] != 3 {
		t.Fatalf("synchronousConnectors = %v, want 3", report.Signals["synchronousConnectors"])
	}
}

func assertFinding(t *testing.T, report Report, suite string, title string) {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.Suite == suite && finding.Title == title {
			return
		}
	}
	t.Fatalf("finding %q/%q missing in %#v", suite, title, report.Findings)
}
