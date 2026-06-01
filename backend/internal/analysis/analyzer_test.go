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

func TestAnalyzeFindsConsistencyAvailabilityAndDataRisks(t *testing.T) {
	report, err := New().Analyze(json.RawMessage(`{
		"schemaVersion": "sde-ui/v0.1",
		"id": "design_1",
		"title": "Checkout cache",
		"requirementBrief": {
			"useCase": "Serve checkout carts",
			"functionalRequirements": "Create carts and reserve inventory",
			"targetRps": 700,
			"availabilityRequirement": "99.95%",
			"sla": "300ms",
			"consistencyNotes": "strong read-after-write consistency",
			"openQuestions": "What is the inventory ownership boundary?"
		},
		"components": [
			{"id": "client", "type": "client.web", "name": "Web", "metadata": {}},
			{"id": "svc1", "type": "compute.service", "name": "Checkout", "criticality": "critical", "metadata": {}},
			{"id": "svc2", "type": "compute.service", "name": "Inventory", "metadata": {}},
			{"id": "redis", "type": "data.redis", "name": "Cart Cache", "metadata": {
				"redis": {
					"usage": "cache",
					"clusterMode": "unknown",
					"persistence": "unknown",
					"backupRestore": "unknown",
					"hotKeyRisk": "high"
				}
			}}
		],
		"connectors": [
			{"id": "c1", "fromComponentId": "client", "toComponentId": "svc1", "type": "synchronous", "protocol": "http"},
			{"id": "c2", "fromComponentId": "svc1", "toComponentId": "svc2", "type": "synchronous"},
			{"id": "c3", "fromComponentId": "svc2", "toComponentId": "redis", "type": "synchronous"},
			{"id": "c4", "fromComponentId": "redis", "toComponentId": "svc1", "type": "asynchronous_event"},
			{"id": "c5", "fromComponentId": "svc1", "toComponentId": "redis", "type": "cache_write"}
		]
	}`))
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	assertFinding(t, report, "consistency", "Strong consistency goal crosses async connector")
	assertFinding(t, report, "consistency", "Cache write consistency is unspecified")
	assertFinding(t, report, "consistency", "Redis consistency mode needs review")
	assertFinding(t, report, "consistency", "Redis cluster mode is unknown")
	assertFinding(t, report, "availability", "Tight SLA with long synchronous chain")
	assertFinding(t, report, "availability", "Critical component has no owner")
	assertFinding(t, report, "security", "Client traffic has no ingress component")
	assertFinding(t, report, "security", "Connector protocol may need transport security")
	assertFinding(t, report, "data", "Redis persistence is unknown")
	assertFinding(t, report, "data", "Redis backup/restore is unknown")
	assertFinding(t, report, "data", "Redis hot-key risk is high")
	assertFinding(t, report, "data", "Data store ownership is unclear")
	if report.Signals["openQuestions"] != 1 {
		t.Fatalf("openQuestions signal = %v, want 1", report.Signals["openQuestions"])
	}
}

func TestAnalyzeRejectsInvalidDocumentJSON(t *testing.T) {
	if _, err := New().Analyze(json.RawMessage(`{`)); err == nil {
		t.Fatal("expected invalid JSON to be rejected")
	}
	if _, err := New().Analyze(nil); err == nil {
		t.Fatal("expected empty JSON to be rejected")
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

func TestBuildUserPromptAndFailedAIReview(t *testing.T) {
	report, err := New().Analyze(json.RawMessage(`{
		"schemaVersion": "sde-ui/v0.1",
		"id": "design_1",
		"title": "Promptable",
		"requirementBrief": {"useCase": "Review prompt construction"},
		"components": [],
		"connectors": []
	}`))
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	prompt, err := BuildUserPrompt(json.RawMessage(`{"id":"design_1","title":"Promptable"}`), report)
	if err != nil {
		t.Fatalf("BuildUserPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Deterministic report:") || !strings.Contains(prompt, "Structured design JSON:") {
		t.Fatalf("prompt missing expected sections: %s", prompt)
	}
	if _, err := BuildUserPrompt(json.RawMessage(`{`), report); err == nil {
		t.Fatal("expected invalid design JSON to fail prompt construction")
	}

	originalSummary := report.Summary
	updated := AttachAIReview(report, AIReview{
		Status: "failed",
		Error:  "provider timed out",
	})
	if updated.Summary != originalSummary {
		t.Fatalf("failed AI review changed summary to %q, want %q", updated.Summary, originalSummary)
	}
	assertWorkflowStatus(t, updated, "ai", "failed")
	if updated.AIReview == nil || updated.AIReview.PromptVersion != PromptVersion {
		t.Fatal("failed AI review did not attach prompt version")
	}
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
