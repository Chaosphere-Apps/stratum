package analysis

import (
	"encoding/json"
	"fmt"
	"strings"
)

const PromptVersion = "stratum-analysis-v2"

type ReviewOptions struct {
	Focus         string
	VersionID     string
	VersionNumber int
}

type AIReview struct {
	Status          string          `json:"status"`
	Provider        string          `json:"provider,omitempty"`
	Model           string          `json:"model,omitempty"`
	PromptVersion   string          `json:"promptVersion"`
	ExecutiveReview string          `json:"executiveReview,omitempty"`
	Strengths       []string        `json:"strengths,omitempty"`
	Risks           []AIReviewPoint `json:"risks,omitempty"`
	Recommendations []AIReviewPoint `json:"recommendations,omitempty"`
	OpenQuestions   []string        `json:"openQuestions,omitempty"`
	Error           string          `json:"error,omitempty"`
}

type AIReviewPoint struct {
	Severity       string `json:"severity"`
	Suite          string `json:"suite"`
	Title          string `json:"title"`
	Detail         string `json:"detail"`
	Impact         string `json:"impact"`
	Recommendation string `json:"recommendation"`
	ComponentID    string `json:"componentId,omitempty"`
	ConnectorID    string `json:"connectorId,omitempty"`
}

type AIReviewEnvelope struct {
	ExecutiveReview string          `json:"executiveReview"`
	Strengths       []string        `json:"strengths"`
	Risks           []AIReviewPoint `json:"risks"`
	Recommendations []AIReviewPoint `json:"recommendations"`
	OpenQuestions   []string        `json:"openQuestions"`
}

func BuildSystemPrompt() string {
	return BuildReviewSystemPrompt(ReviewOptions{})
}

func BuildReviewSystemPrompt(options ReviewOptions) string {
	focus := strings.TrimSpace(options.Focus)
	if focus == "" || focus == "full" {
		focus = "a balanced full architecture review"
	} else {
		focus = "a deep review focused on " + focus
	}
	return strings.TrimSpace(`
You are Stratum's enterprise system-design reviewer.
You review structured architecture diagrams using deterministic evidence plus senior architecture judgement.

Rules:
- Treat the deterministic report as evidence. Do not contradict it unless you explicitly say the evidence is insufficient.
- Be concrete. Tie every risk or recommendation to a componentId, connectorId, suite, requirement, or missing field when possible.
- Do not invent infrastructure that is not in the JSON. Phrase unknowns as open questions.
- Prefer production concerns: correctness, consistency, availability, scalability, operability, security, data ownership, blast radius, and rollout risk.
- Keep output actionable for an enterprise UI. No markdown tables. No prose outside JSON.
- Severity must be one of: high, medium, low.
- Suite must be one of: requirements, topology, traffic, consistency, availability, security, data, operability, cost, ai, integrity.

Return only valid JSON with this exact shape:
{
  "executiveReview": "short but detailed paragraph",
  "strengths": ["specific strength"],
  "risks": [
    {
      "severity": "high|medium|low",
      "suite": "traffic",
      "title": "short risk title",
      "detail": "what is wrong or uncertain",
      "impact": "why it matters",
      "recommendation": "specific next action",
      "componentId": "optional",
      "connectorId": "optional"
    }
  ],
  "recommendations": [
    {
      "severity": "high|medium|low",
      "suite": "availability",
      "title": "short recommendation title",
      "detail": "what to change",
      "impact": "expected benefit",
      "recommendation": "implementation guidance",
      "componentId": "optional",
      "connectorId": "optional"
    }
  ],
  "openQuestions": ["question that blocks a confident review"]
}
`) + "\n\nReview objective: " + focus + "."
}

func BuildUserPrompt(raw json.RawMessage, report Report) (string, error) {
	return BuildReviewUserPrompt(raw, report, ReviewOptions{})
}

func BuildReviewUserPrompt(raw json.RawMessage, report Report, options ReviewOptions) (string, error) {
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	var compactDesign any
	if err := json.Unmarshal(raw, &compactDesign); err != nil {
		return "", err
	}
	designJSON, err := json.MarshalIndent(compactDesign, "", "  ")
	if err != nil {
		return "", err
	}
	versionContext := "current working design"
	if options.VersionNumber > 0 {
		versionContext = fmt.Sprintf("saved version v%d (%s)", options.VersionNumber, options.VersionID)
	}
	return fmt.Sprintf("Review target: %s\nDeterministic report:\n%s\n\nStructured design JSON:\n%s", versionContext, reportJSON, designJSON), nil
}

func AttachAIReview(report Report, review AIReview) Report {
	review.PromptVersion = PromptVersion
	report.AIReview = &review
	for index := range report.Workflow {
		if report.Workflow[index].ID == "ai" {
			report.Workflow[index].Status = review.Status
			if review.Status == "completed" {
				report.Workflow[index].Detail = "Model-assisted tradeoff synthesis completed using enterprise AI configuration."
			} else if review.Error != "" {
				report.Workflow[index].Detail = review.Error
			}
		}
	}
	if review.ExecutiveReview != "" {
		report.Summary = review.ExecutiveReview
	}
	return report
}
