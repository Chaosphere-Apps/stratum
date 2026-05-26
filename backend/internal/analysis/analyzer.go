package analysis

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Report struct {
	Summary  string             `json:"summary"`
	Score    int                `json:"score"`
	Findings []Finding          `json:"findings"`
	Signals  map[string]float64 `json:"signals"`
	Suites   []SuiteReport      `json:"suites,omitempty"`
	Workflow []WorkflowStep     `json:"workflow,omitempty"`
	AIReview *AIReview          `json:"aiReview,omitempty"`
}

type Finding struct {
	Severity       string `json:"severity"`
	Suite          string `json:"suite"`
	Title          string `json:"title"`
	Detail         string `json:"detail"`
	Impact         string `json:"impact,omitempty"`
	Evidence       string `json:"evidence,omitempty"`
	ComponentID    string `json:"componentId,omitempty"`
	ConnectorID    string `json:"connectorId,omitempty"`
	Recommendation string `json:"recommendation,omitempty"`
	Confidence     string `json:"confidence,omitempty"`
}

type WorkflowStep struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type SuiteReport struct {
	Name     string  `json:"name"`
	Score    int     `json:"score"`
	Findings int     `json:"findings"`
	Weight   float64 `json:"weight"`
}

type Analyzer struct{}

func New() Analyzer {
	return Analyzer{}
}

func (Analyzer) Analyze(raw json.RawMessage) (Report, error) {
	document, err := parseDocument(raw)
	if err != nil {
		return Report{}, err
	}

	ctx := newContext(document)
	ctx.runRequirementsSuite()
	ctx.runTopologySuite()
	ctx.runTrafficSuite()
	ctx.runConsistencySuite()
	ctx.runAvailabilitySuite()
	ctx.runSecuritySuite()
	ctx.runDataSuite()

	ctx.finalizeScores()
	return ctx.report(), nil
}

type document struct {
	SchemaVersion    string           `json:"schemaVersion"`
	ID               string           `json:"id"`
	Title            string           `json:"title"`
	RequirementBrief requirementBrief `json:"requirementBrief"`
	Components       []component      `json:"components"`
	Connectors       []connector      `json:"connectors"`
}

type requirementBrief struct {
	UseCase                   string `json:"useCase"`
	FunctionalRequirements    string `json:"functionalRequirements"`
	NonFunctionalRequirements string `json:"nonFunctionalRequirements"`
	TargetRPS                 *int   `json:"targetRps"`
	AvailabilityRequirement   string `json:"availabilityRequirement"`
	SLA                       string `json:"sla"`
	ProblemStatement          string `json:"problemStatement"`
	PrimaryActors             string `json:"primaryActors"`
	TrafficNotes              string `json:"trafficNotes"`
	ConsistencyNotes          string `json:"consistencyNotes"`
	OpenQuestions             string `json:"openQuestions"`
}

type component struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Purpose     string            `json:"purpose"`
	Owner       string            `json:"owner"`
	Criticality string            `json:"criticality"`
	Metadata    componentMetadata `json:"metadata"`
	Notes       []note            `json:"notes"`
}

type componentMetadata struct {
	ExpectedQPS     *int           `json:"expectedQps"`
	LatencyBudgetMS *int           `json:"latencyBudgetMs"`
	ParentFrameID   string         `json:"parentFrameId"`
	Notes           string         `json:"notes"`
	Redis           *redisMetadata `json:"redis"`
}

type redisMetadata struct {
	Usage                  string `json:"usage"`
	ClusterMode            string `json:"clusterMode"`
	Replication            string `json:"replication"`
	Persistence            string `json:"persistence"`
	EvictionPolicy         string `json:"evictionPolicy"`
	ConsistencyExpectation string `json:"consistencyExpectation"`
	FailoverBehavior       string `json:"failoverBehavior"`
	BackupRestore          string `json:"backupRestore"`
	MemoryLimitGB          *int   `json:"memoryLimitGb"`
	ExpectedQPS            *int   `json:"expectedQps"`
	HotKeyRisk             string `json:"hotKeyRisk"`
}

type connector struct {
	ID                     string `json:"id"`
	FromComponentID        string `json:"fromComponentId"`
	ToComponentID          string `json:"toComponentId"`
	Type                   string `json:"type"`
	Protocol               string `json:"protocol"`
	TimeoutMS              *int   `json:"timeoutMs"`
	ConsistencyExpectation string `json:"consistencyExpectation"`
	Notes                  string `json:"notes"`
	Animated               bool   `json:"animated"`
}

type note struct {
	ID   string `json:"id"`
	Body string `json:"body"`
	Tone string `json:"tone"`
}

func parseDocument(raw json.RawMessage) (document, error) {
	if len(raw) == 0 || !json.Valid(raw) {
		return document{}, fmt.Errorf("design document must be valid JSON")
	}
	var parsed document
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return document{}, fmt.Errorf("design document is invalid: %w", err)
	}
	return parsed, nil
}

type analysisContext struct {
	document       document
	componentsByID map[string]component
	incoming       map[string][]connector
	outgoing       map[string][]connector
	findings       []Finding
	signals        map[string]float64
	suites         map[string]*suiteScore
}

type suiteScore struct {
	name     string
	weight   float64
	penalty  int
	findings int
}

func newContext(document document) *analysisContext {
	ctx := &analysisContext{
		document:       document,
		componentsByID: map[string]component{},
		incoming:       map[string][]connector{},
		outgoing:       map[string][]connector{},
		signals:        map[string]float64{},
		suites: map[string]*suiteScore{
			"requirements": {name: "requirements", weight: 1.2},
			"topology":     {name: "topology", weight: 1.3},
			"traffic":      {name: "traffic", weight: 1.1},
			"consistency":  {name: "consistency", weight: 1.1},
			"availability": {name: "availability", weight: 1.1},
			"security":     {name: "security", weight: 1.0},
			"data":         {name: "data", weight: 1.0},
			"integrity":    {name: "integrity", weight: 1.2},
		},
	}
	for _, component := range document.Components {
		ctx.componentsByID[component.ID] = component
	}
	for _, connector := range document.Connectors {
		ctx.outgoing[connector.FromComponentID] = append(ctx.outgoing[connector.FromComponentID], connector)
		ctx.incoming[connector.ToComponentID] = append(ctx.incoming[connector.ToComponentID], connector)
	}
	return ctx
}

func (ctx *analysisContext) runRequirementsSuite() {
	brief := ctx.document.RequirementBrief
	if blank(brief.UseCase) && blank(brief.ProblemStatement) {
		ctx.add("medium", "requirements", "Use case is missing", "Capture the user journey or product flow before evaluating architecture choices.", "", "", "Add a concise use case and the main actor.")
	}
	if blank(brief.FunctionalRequirements) {
		ctx.add("medium", "requirements", "Functional requirements are missing", "The analyzer cannot distinguish must-have behavior from implementation details.", "", "", "List the primary read/write flows and expected outcomes.")
	}
	if blank(brief.NonFunctionalRequirements) && blank(brief.AvailabilityRequirement) && blank(brief.SLA) {
		ctx.add("medium", "requirements", "Non-functional requirements are thin", "Availability, latency, consistency, and scale goals should be explicit before design scoring.", "", "", "Add latency, availability, durability, and operating constraints.")
	}
	if !blank(brief.OpenQuestions) {
		ctx.signals["openQuestions"] = 1
	}
}

func (ctx *analysisContext) runTopologySuite() {
	realComponents := ctx.realComponents()
	ctx.signals["components"] = float64(len(realComponents))
	ctx.signals["connectors"] = float64(len(ctx.document.Connectors))
	ctx.signals["cloudFrames"] = float64(ctx.countComponentsByPrefix("frame."))
	ctx.signals["notes"] = float64(ctx.countComponentsByPrefix("note."))

	if len(realComponents) == 0 {
		ctx.add("high", "topology", "No architecture components modeled", "Add compute, data, network, or external components to make the design analyzable.", "", "", "Start with client, ingress, service, data store, and external dependencies.")
		return
	}
	if len(realComponents) > 1 && len(ctx.document.Connectors) == 0 {
		ctx.add("high", "topology", "No data flow modeled", "Connect components so dependency, latency, and consistency paths can be evaluated.", "", "", "Draw at least the critical request path.")
	}

	for _, component := range realComponents {
		if len(ctx.incoming[component.ID]) == 0 && len(ctx.outgoing[component.ID]) == 0 {
			ctx.add("medium", "topology", "Component is isolated", fmt.Sprintf("%s is not connected to the modeled flow.", displayName(component)), component.ID, "", "Connect it or convert it to a note if it is just context.")
		}
		if blank(component.Purpose) && !isFrameOrNote(component) {
			ctx.add("low", "topology", "Component purpose is missing", fmt.Sprintf("%s has no purpose captured.", displayName(component)), component.ID, "", "Add a one-line responsibility so later AI review has intent, not just shape.")
		}
	}

	for _, connector := range ctx.document.Connectors {
		if _, ok := ctx.componentsByID[connector.FromComponentID]; !ok {
			ctx.add("high", "integrity", "Connector source is missing", "A connector points from a component that is no longer present.", "", connector.ID, "Delete or reconnect the connector.")
		}
		if _, ok := ctx.componentsByID[connector.ToComponentID]; !ok {
			ctx.add("high", "integrity", "Connector target is missing", "A connector points to a component that is no longer present.", "", connector.ID, "Delete or reconnect the connector.")
		}
	}

	if ctx.hasSynchronousCycle() {
		ctx.add("high", "topology", "Synchronous cycle detected", "A blocking call cycle can amplify latency, retries, and partial outages.", "", "", "Break the cycle with ownership boundaries, async handoff, or a clear source of truth.")
	}
}

func (ctx *analysisContext) runTrafficSuite() {
	brief := ctx.document.RequirementBrief
	if brief.TargetRPS == nil && blank(brief.TrafficNotes) {
		ctx.add("medium", "traffic", "Traffic target is missing", "Capacity checks need a target RPS or traffic note to be grounded.", "", "", "Capture peak RPS, burst behavior, payload size, and read/write split.")
		return
	}
	targetRPS := 0
	if brief.TargetRPS != nil {
		targetRPS = *brief.TargetRPS
		ctx.signals["targetRps"] = float64(targetRPS)
	}
	if targetRPS >= 1000 && !ctx.hasTypePrefix("data.redis") && !ctx.hasTypePrefix("messaging.") {
		ctx.add("medium", "traffic", "High traffic path lacks buffering or cache", "The target RPS is high, but the design does not model a cache, queue, or buffering layer.", "", "", "Add an explicit cache, async queue, rate limiter, or explain why direct serving is enough.")
	}
	for _, component := range ctx.realComponents() {
		if component.Metadata.ExpectedQPS == nil && targetRPS >= 500 {
			ctx.add("low", "traffic", "Component-level QPS is missing", fmt.Sprintf("%s has no expected QPS captured.", displayName(component)), component.ID, "", "Add expected QPS for hot components to support bottleneck analysis.")
		}
	}
	for _, connector := range ctx.document.Connectors {
		if connector.Type == "synchronous" && connector.TimeoutMS == nil {
			ctx.add("medium", "traffic", "Synchronous connector has no timeout", "Blocking calls need timeout budgets for latency and retry analysis.", "", connector.ID, "Set a timeout and retry policy aligned to the SLA.")
		}
	}
}

func (ctx *analysisContext) runConsistencySuite() {
	brief := strings.ToLower(ctx.document.RequirementBrief.ConsistencyNotes)
	if blank(brief) {
		ctx.add("medium", "consistency", "Consistency expectation is missing", "State which paths require strong, read-after-write, or eventual consistency.", "", "", "Capture consistency per critical write/read path.")
	}
	requiresStrong := containsAny(brief, "strong", "read-after-write", "read after write", "linearizable", "transaction")
	for _, connector := range ctx.document.Connectors {
		expectation := strings.ToLower(connector.ConsistencyExpectation)
		if requiresStrong && connector.Type == "asynchronous_event" {
			ctx.add("medium", "consistency", "Strong consistency goal crosses async connector", "The requirements mention strong consistency, but an async connector is in the design.", "", connector.ID, "Mark this path eventual, add reconciliation, or use a synchronous transactional boundary.")
		}
		if connector.Type == "cache_write" && blank(expectation) {
			ctx.add("low", "consistency", "Cache write consistency is unspecified", "Cache writes need invalidation, TTL, or write-through/write-around semantics.", "", connector.ID, "Specify cache consistency behavior.")
		}
	}
	for _, component := range ctx.document.Components {
		if component.Metadata.Redis == nil {
			continue
		}
		redis := component.Metadata.Redis
		if requiresStrong && redis.ConsistencyExpectation != "strong_for_locking" && redis.Usage != "lock_manager" {
			ctx.add("medium", "consistency", "Redis consistency mode needs review", fmt.Sprintf("%s is present while requirements suggest strong consistency.", displayName(component)), component.ID, "", "Clarify whether Redis is cache-only, lock manager, or source of truth.")
		}
		if redis.ClusterMode == "unknown" {
			ctx.add("low", "consistency", "Redis cluster mode is unknown", fmt.Sprintf("%s does not specify cluster mode.", displayName(component)), component.ID, "", "Set cluster mode because it affects sharding, failover, and multi-key guarantees.")
		}
	}
}

func (ctx *analysisContext) runAvailabilitySuite() {
	availability := parseAvailability(ctx.document.RequirementBrief.AvailabilityRequirement)
	if availability > 0 {
		ctx.signals["availabilityTarget"] = availability
	}
	if availability >= 99.9 && !ctx.hasRedundancySignal() {
		ctx.add("medium", "availability", "Availability target lacks redundancy detail", "The target suggests production-grade availability, but replicas, multi-AZ, failover, or recovery behavior are not modeled.", "", "", "Add HA details to critical components and data stores.")
	}
	if ctx.document.RequirementBrief.SLA != "" {
		if latencyMS := parseLatencyMS(ctx.document.RequirementBrief.SLA); latencyMS > 0 {
			ctx.signals["slaLatencyMs"] = float64(latencyMS)
			if latencyMS <= 500 && ctx.countConnectors("synchronous") >= 3 {
				ctx.add("medium", "availability", "Tight SLA with long synchronous chain", "Multiple synchronous hops make a tight latency SLA harder to meet and operate.", "", "", "Consider collapsing hops, adding async handoff, or assigning latency budgets per hop.")
			}
		}
	}
	for _, component := range ctx.realComponents() {
		if isCritical(component) && blank(component.Owner) {
			ctx.add("low", "availability", "Critical component has no owner", fmt.Sprintf("%s is marked critical but has no owner.", displayName(component)), component.ID, "", "Add an owning team or service boundary.")
		}
	}
}

func (ctx *analysisContext) runSecuritySuite() {
	hasExternal := ctx.hasTypePrefix("external.")
	hasClient := ctx.hasTypePrefix("client.")
	hasSecurity := ctx.hasTypePrefix("security.")
	hasEdge := ctx.hasTypePrefix("edge.")
	if (hasExternal || hasClient) && !hasSecurity {
		ctx.add("medium", "security", "Trust boundary lacks security control", "The design has clients or external APIs but no explicit auth, policy, WAF, KMS, or security control.", "", "", "Add security controls at ingress and sensitive data boundaries.")
	}
	if hasClient && !hasEdge {
		ctx.add("low", "security", "Client traffic has no ingress component", "Client-facing flows should usually model an API gateway, load balancer, or edge policy layer.", "", "", "Add the entry point that owns auth, routing, throttling, and observability.")
	}
	for _, connector := range ctx.document.Connectors {
		if connector.Protocol != "" && !containsAny(strings.ToLower(connector.Protocol), "https", "tls", "grpc") {
			ctx.add("low", "security", "Connector protocol may need transport security", fmt.Sprintf("Protocol %q does not indicate TLS.", connector.Protocol), "", connector.ID, "Use TLS-capable protocol details for production paths.")
		}
	}
}

func (ctx *analysisContext) runDataSuite() {
	hasDataStore := ctx.hasTypePrefix("data.")
	if len(ctx.realComponents()) >= 2 && !hasDataStore {
		ctx.add("low", "data", "No data store identified", "If the use case persists state, model the source of truth explicitly.", "", "", "Add SQL, object store, Redis usage, or document why the flow is stateless.")
	}
	for _, component := range ctx.document.Components {
		if !strings.HasPrefix(component.Type, "data.") {
			continue
		}
		if component.Metadata.Redis != nil {
			redis := component.Metadata.Redis
			if redis.Persistence == "unknown" {
				ctx.add("low", "data", "Redis persistence is unknown", fmt.Sprintf("%s does not specify persistence.", displayName(component)), component.ID, "", "Choose none, RDB, AOF, managed default, or explain why persistence is unnecessary.")
			}
			if redis.BackupRestore == "unknown" {
				ctx.add("low", "data", "Redis backup/restore is unknown", fmt.Sprintf("%s does not specify backup or restore behavior.", displayName(component)), component.ID, "", "Capture whether backups are configured or not required.")
			}
			if redis.HotKeyRisk == "high" {
				ctx.add("medium", "data", "Redis hot-key risk is high", fmt.Sprintf("%s is marked with high hot-key risk.", displayName(component)), component.ID, "", "Add sharding, request coalescing, local caching, or key design notes.")
			}
		}
		if blank(component.Purpose) {
			ctx.add("low", "data", "Data store ownership is unclear", fmt.Sprintf("%s does not describe what data it owns.", displayName(component)), component.ID, "", "State the entities, durability expectation, and write owner.")
		}
	}
}

func (ctx *analysisContext) add(severity, suite, title, detail, componentID, connectorID, recommendation string) {
	ctx.findings = append(ctx.findings, Finding{
		Severity:       severity,
		Suite:          suite,
		Title:          title,
		Detail:         detail,
		Impact:         impactForSeverity(severity),
		Evidence:       evidenceLabel(componentID, connectorID),
		ComponentID:    componentID,
		ConnectorID:    connectorID,
		Recommendation: recommendation,
		Confidence:     "deterministic",
	})
	score, ok := ctx.suites[suite]
	if !ok {
		score = &suiteScore{name: suite, weight: 1}
		ctx.suites[suite] = score
	}
	score.findings++
	score.penalty += severityPenalty(severity)
}

func (ctx *analysisContext) finalizeScores() {
	ctx.signals["synchronousConnectors"] = float64(ctx.countConnectors("synchronous"))
	ctx.signals["asyncConnectors"] = float64(ctx.countConnectors("asynchronous_event"))
	ctx.signals["dataStores"] = float64(ctx.countComponentsByPrefix("data."))
	ctx.signals["securityControls"] = float64(ctx.countComponentsByPrefix("security."))
}

func (ctx *analysisContext) report() Report {
	totalPenalty := 0.0
	suites := make([]SuiteReport, 0, len(ctx.suites))
	for _, suite := range ctx.suites {
		score := clampInt(100-suite.penalty, 0, 100)
		totalPenalty += float64(suite.penalty) * suite.weight
		suites = append(suites, SuiteReport{Name: suite.name, Score: score, Findings: suite.findings, Weight: suite.weight})
	}
	sort.Slice(suites, func(i, j int) bool { return suites[i].Name < suites[j].Name })

	score := clampInt(100-int(math.Round(totalPenalty/1.8)), 0, 100)
	summary := "Structured analysis completed across requirements, topology, traffic, consistency, availability, security, and data."
	if len(ctx.findings) == 0 {
		summary = "Structured analysis found no base issues. The design is ready for deeper mathematical and AI review."
	}
	sort.SliceStable(ctx.findings, func(i, j int) bool {
		return severityPenalty(ctx.findings[i].Severity) > severityPenalty(ctx.findings[j].Severity)
	})
	return Report{Summary: summary, Score: score, Findings: ctx.findings, Signals: ctx.signals, Suites: suites, Workflow: deterministicWorkflow()}
}

func (ctx *analysisContext) realComponents() []component {
	components := []component{}
	for _, component := range ctx.document.Components {
		if isFrameOrNote(component) {
			continue
		}
		components = append(components, component)
	}
	return components
}

func (ctx *analysisContext) hasTypePrefix(prefix string) bool {
	for _, component := range ctx.document.Components {
		if strings.HasPrefix(component.Type, prefix) {
			return true
		}
	}
	return false
}

func (ctx *analysisContext) countComponentsByPrefix(prefix string) int {
	count := 0
	for _, component := range ctx.document.Components {
		if strings.HasPrefix(component.Type, prefix) {
			count++
		}
	}
	return count
}

func (ctx *analysisContext) countConnectors(connectorType string) int {
	count := 0
	for _, connector := range ctx.document.Connectors {
		if connector.Type == connectorType {
			count++
		}
	}
	return count
}

func (ctx *analysisContext) hasRedundancySignal() bool {
	haystack := strings.ToLower(strings.Join([]string{
		ctx.document.RequirementBrief.NonFunctionalRequirements,
		ctx.document.RequirementBrief.AvailabilityRequirement,
		ctx.document.RequirementBrief.SLA,
	}, " "))
	if containsAny(haystack, "multi-az", "multi az", "replica", "active-active", "active active", "failover", "redundan") {
		return true
	}
	for _, component := range ctx.document.Components {
		text := strings.ToLower(component.Metadata.Notes + " " + component.Purpose)
		if containsAny(text, "multi-az", "multi az", "replica", "active-active", "failover", "redundan") {
			return true
		}
		if component.Metadata.Redis != nil {
			redis := component.Metadata.Redis
			if redis.Replication != "unknown" && redis.Replication != "none" {
				return true
			}
			if redis.FailoverBehavior == "automatic" {
				return true
			}
		}
	}
	return false
}

func (ctx *analysisContext) hasSynchronousCycle() bool {
	adjacency := map[string][]string{}
	for _, connector := range ctx.document.Connectors {
		if connector.Type != "synchronous" {
			continue
		}
		adjacency[connector.FromComponentID] = append(adjacency[connector.FromComponentID], connector.ToComponentID)
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(node string) bool {
		if visiting[node] {
			return true
		}
		if visited[node] {
			return false
		}
		visiting[node] = true
		for _, next := range adjacency[node] {
			if visit(next) {
				return true
			}
		}
		visiting[node] = false
		visited[node] = true
		return false
	}
	for node := range adjacency {
		if visit(node) {
			return true
		}
	}
	return false
}

func severityPenalty(severity string) int {
	switch severity {
	case "high":
		return 18
	case "medium":
		return 10
	case "low":
		return 4
	default:
		return 6
	}
}

func impactForSeverity(severity string) string {
	switch severity {
	case "high":
		return "Likely to cause reliability, security, or correctness issues unless addressed before production."
	case "medium":
		return "May become a production risk as traffic, team ownership, or failure modes grow."
	case "low":
		return "Improves clarity and evaluation confidence."
	default:
		return "Review recommended."
	}
}

func evidenceLabel(componentID string, connectorID string) string {
	if componentID != "" {
		return "component:" + componentID
	}
	if connectorID != "" {
		return "connector:" + connectorID
	}
	return "design"
}

func deterministicWorkflow() []WorkflowStep {
	return []WorkflowStep{
		{ID: "parse", Label: "Parse structured design", Status: "completed", Detail: "Validated design JSON and extracted requirements, components, connectors, metadata, and notes."},
		{ID: "math", Label: "Run deterministic suites", Status: "completed", Detail: "Evaluated topology, traffic, consistency, availability, security, data, and integrity signals."},
		{ID: "ai", Label: "AI synthesis", Status: "skipped", Detail: "Configure an enterprise AI provider in Admin to add model-assisted tradeoff review."},
	}
}

func blank(value string) bool {
	return strings.TrimSpace(value) == ""
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func isFrameOrNote(component component) bool {
	return strings.HasPrefix(component.Type, "frame.") || strings.HasPrefix(component.Type, "note.")
}

func isCritical(component component) bool {
	return component.Criticality == "high" || component.Criticality == "critical"
}

func displayName(component component) string {
	if !blank(component.Name) {
		return component.Name
	}
	if !blank(component.Type) {
		return component.Type
	}
	return component.ID
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func parseAvailability(value string) float64 {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	if value == "" {
		return 0
	}
	re := regexp.MustCompile(`\d+(?:\.\d+)?`)
	match := re.FindString(value)
	if match == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseLatencyMS(value string) int {
	value = strings.ToLower(value)
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(ms|millisecond|milliseconds|s|sec|secs|second|seconds)`)
	matches := re.FindStringSubmatch(value)
	if len(matches) != 3 {
		return 0
	}
	number, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}
	unit := matches[2]
	if unit == "s" || strings.HasPrefix(unit, "sec") || strings.HasPrefix(unit, "second") {
		number *= 1000
	}
	return int(math.Round(number))
}
