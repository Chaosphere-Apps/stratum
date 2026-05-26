package analysis

import (
	"encoding/json"
	"strings"
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

func TestAIReviewContractIsStructured(t *testing.T) {
	report, err := New().Analyze(json.RawMessage(`{
		"schemaVersion": "sde-ui/v0.1",
		"id": "design_1",
		"title": "Notification system",
		"requirementBrief": {"useCase": "Send transactional notifications"},
		"components": [],
		"connectors": []
	}`))
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	prompt := BuildSystemPrompt()
	if !strings.Contains(prompt, `"executiveReview"`) || !strings.Contains(prompt, `"recommendations"`) {
		t.Fatalf("system prompt does not include required JSON shape: %s", prompt)
	}
	updated := AttachAIReview(report, AIReview{
		Status:          "completed",
		Provider:        "openai",
		Model:           "gpt-4.1",
		ExecutiveReview: "The design needs durability and delivery semantics before production review.",
		Recommendations: []AIReviewPoint{{Severity: "medium", Suite: "data", Title: "Define message retention", Detail: "Retention is not modeled.", Impact: "Retries may be unsafe.", Recommendation: "Add queue retention and dead-letter policy."}},
	})
	if updated.Summary != "The design needs durability and delivery semantics before production review." {
		t.Fatalf("summary = %q, want AI executive review", updated.Summary)
	}
	if updated.AIReview == nil || updated.AIReview.PromptVersion != PromptVersion {
		t.Fatalf("AI review prompt version was not attached")
	}
	assertWorkflowStatus(t, updated, "ai", "completed")
}

func assertWorkflowStatus(t *testing.T, report Report, id string, status string) {
	t.Helper()
	for _, step := range report.Workflow {
		if step.ID == id {
			if step.Status != status {
				t.Fatalf("workflow %q status = %q, want %q", id, step.Status, status)
			}
			return
		}
	}
	t.Fatalf("workflow step %q missing in %#v", id, report.Workflow)
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
