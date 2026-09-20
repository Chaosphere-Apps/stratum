package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/lifecycle"
	"strings"
	"time"
)

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func (r *MemoryRepository) createUserLocked(displayName string, email string, role string, password string) (domain.User, error) {
	displayName = strings.TrimSpace(displayName)
	email = strings.ToLower(strings.TrimSpace(email))
	if displayName == "" {
		return domain.User{}, errors.New("display name is required")
	}
	if email == "" {
		return domain.User{}, errors.New("email is required")
	}
	for _, user := range r.users {
		if strings.EqualFold(user.Email, email) {
			return domain.User{}, errors.New("a user with this email already exists")
		}
	}
	passwordHash := ""
	if strings.TrimSpace(password) != "" {
		var err error
		passwordHash, err = hashPassword(password)
		if err != nil {
			return domain.User{}, err
		}
	}
	now := r.clock().UTC()
	user := domain.User{
		ID:           fmt.Sprintf("user_%d_%d", now.UnixNano(), len(r.users)+1),
		DisplayName:  displayName,
		Email:        email,
		PasswordSet:  passwordHash != "",
		PasswordHash: passwordHash,
		Role:         normalizedUserRole(role),
		Status:       "active",
		LastSeenAt:   now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	r.users[user.ID] = user
	return user, nil
}

func (r *MemoryRepository) emailExistsLocked(email string, exceptUserID string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, user := range r.users {
		if user.ID != exceptUserID && strings.EqualFold(user.Email, email) {
			return true
		}
	}
	return false
}

func (r *MemoryRepository) getUserLocked(userID string) (domain.User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = r.firstUserIDLocked()
	}
	user, ok := r.users[userID]
	if !ok || user.Status == "disabled" {
		return domain.User{}, errors.New("user not found")
	}
	return user, nil
}

func (r *MemoryRepository) firstUserIDLocked() string {
	var first domain.User
	for _, user := range r.users {
		if user.Status == "disabled" {
			continue
		}
		if first.ID == "" || user.CreatedAt.Before(first.CreatedAt) {
			first = user
		}
	}
	return first.ID
}

func (r *MemoryRepository) adminCountLocked() int {
	count := 0
	for _, user := range r.users {
		if user.Role == "admin" && user.Status != "disabled" {
			count++
		}
	}
	return count
}

func (r *MemoryRepository) claimLegacyGuestDataLocked(userID string) {
	if workspace, ok := r.workspaces[domain.GuestWorkspaceID]; ok && workspace.OwnerID == "" {
		workspace.OwnerID = userID
		workspace.UpdatedAt = r.clock().UTC()
		r.workspaces[workspace.ID] = workspace
	}
	for id, design := range r.designs {
		if design.CreatedBy == "" || design.CreatedBy == "guest-user" {
			design.CreatedBy = userID
			r.designs[id] = design
		}
	}
	for designID, versions := range r.versions {
		for index, version := range versions {
			if version.CreatedBy == "" || version.CreatedBy == "guest-user" {
				version.CreatedBy = userID
				versions[index] = version
			}
		}
		r.versions[designID] = versions
	}
	for id, doc := range r.docs {
		if doc.CreatedBy == "" || doc.CreatedBy == "guest-user" {
			doc.CreatedBy = userID
			r.docs[id] = doc
		}
	}
	for id, comment := range r.comments {
		if comment.AuthorID == "" || comment.AuthorID == "guest-user" {
			comment.AuthorID = userID
			r.comments[id] = comment
		}
	}
	for id, notification := range r.notifications {
		if notification.UserID == "" || notification.UserID == "guest-user" {
			notification.UserID = userID
			r.notifications[id] = notification
		}
	}
}

func defaultSignInConfig(now time.Time) domain.SignInConfig {
	return domain.SignInConfig{
		LocalPasswordEnabled: true,
		SSOEnabled:           false,
		Provider:             "okta",
		Scopes:               "openid profile email groups",
		GroupsClaim:          "groups",
		JITProvisioning:      true,
		UpdatedAt:            now,
	}
}

func defaultAIProviderConfig(now time.Time) domain.AIProviderConfig {
	return domain.AIProviderConfig{
		Enabled:   false,
		Provider:  "openai",
		Model:     "gpt-4.1",
		BaseURL:   "",
		UpdatedAt: now,
	}
}

func defaultMCPConfig(now time.Time) domain.MCPConfig {
	return domain.MCPConfig{
		Enabled:             false,
		EndpointPath:        "/mcp",
		ReadCatalog:         true,
		ReadDesigns:         true,
		CreateDraftDesign:   true,
		RunAnalysis:         true,
		FetchImpactReport:   false,
		RequireAdminConsent: true,
		UpdatedAt:           now,
	}
}

func defaultTelemetryIntegrationConfig(now time.Time) domain.TelemetryIntegrationConfig {
	return domain.TelemetryIntegrationConfig{
		Enabled:             false,
		Provider:            "prometheus",
		DisplayName:         "Prometheus Service Graph",
		AuthMode:            "none",
		QueryWindow:         "5m",
		RequestTotalMetric:  "traces_service_graph_request_total",
		RequestFailedMetric: "traces_service_graph_request_failed_total",
		ServerLatencyMetric: "traces_service_graph_request_server_seconds_bucket",
		ClientLatencyMetric: "traces_service_graph_request_client_seconds_bucket",
		Filters: domain.TelemetryIntegrationFilter{
			IncludeExternal:        true,
			IncludeDatabaseClients: true,
		},
		UpdatedAt: now,
	}
}

func sanitizeAIProviderConfig(config domain.AIProviderConfig) domain.AIProviderConfig {
	config.APIKey = ""
	return config
}

func sanitizeTelemetryIntegrationConfig(config domain.TelemetryIntegrationConfig) domain.TelemetryIntegrationConfig {
	config.Secret = ""
	return config
}

func normalizedTelemetryIntegrationConfig(current domain.TelemetryIntegrationConfig, input domain.TelemetryIntegrationConfig, secret string, now time.Time) (domain.TelemetryIntegrationConfig, error) {
	next := current
	next.Enabled = input.Enabled
	next.Provider = normalizedTelemetryProvider(input.Provider)
	next.DisplayName = strings.TrimSpace(input.DisplayName)
	if next.DisplayName == "" {
		next.DisplayName = "Prometheus Service Graph"
	}
	next.BaseURL = strings.TrimSpace(input.BaseURL)
	next.AuthMode = normalizedTelemetryAuthMode(input.AuthMode)
	next.CustomHeaderName = strings.TrimSpace(input.CustomHeaderName)
	next.QueryWindow = strings.TrimSpace(input.QueryWindow)
	if next.QueryWindow == "" {
		next.QueryWindow = "5m"
	}
	next.RequestTotalMetric = strings.TrimSpace(input.RequestTotalMetric)
	if next.RequestTotalMetric == "" {
		next.RequestTotalMetric = "traces_service_graph_request_total"
	}
	next.RequestFailedMetric = strings.TrimSpace(input.RequestFailedMetric)
	if next.RequestFailedMetric == "" {
		next.RequestFailedMetric = "traces_service_graph_request_failed_total"
	}
	next.ServerLatencyMetric = strings.TrimSpace(input.ServerLatencyMetric)
	if next.ServerLatencyMetric == "" {
		next.ServerLatencyMetric = "traces_service_graph_request_server_seconds_bucket"
	}
	next.ClientLatencyMetric = strings.TrimSpace(input.ClientLatencyMetric)
	if next.ClientLatencyMetric == "" {
		next.ClientLatencyMetric = "traces_service_graph_request_client_seconds_bucket"
	}
	next.Filters = normalizedTelemetryFilters(input.Filters)
	if strings.TrimSpace(secret) != "" {
		next.Secret = strings.TrimSpace(secret)
		next.SecretSet = true
	} else {
		next.SecretSet = current.SecretSet && strings.TrimSpace(current.Secret) != ""
	}
	if next.Enabled && next.BaseURL == "" {
		return domain.TelemetryIntegrationConfig{}, errors.New("prometheus URL is required when telemetry integration is enabled")
	}
	if next.AuthMode == "custom_header" && next.CustomHeaderName == "" {
		return domain.TelemetryIntegrationConfig{}, errors.New("custom header name is required for custom header authentication")
	}
	next.UpdatedAt = now
	return next, nil
}

func normalizedTelemetryFilters(filters domain.TelemetryIntegrationFilter) domain.TelemetryIntegrationFilter {
	return domain.TelemetryIntegrationFilter{
		Namespaces:             normalizedStringList(filters.Namespaces),
		Services:               normalizedStringList(filters.Services),
		ExcludeServices:        normalizedStringList(filters.ExcludeServices),
		ExcludeEndpoints:       normalizedStringList(filters.ExcludeEndpoints),
		RequiredLabels:         normalizedStringMap(filters.RequiredLabels),
		MinimumRequestsPerSec:  filters.MinimumRequestsPerSec,
		IncludeExternal:        filters.IncludeExternal,
		IncludeDatabaseClients: filters.IncludeDatabaseClients,
	}
}

func normalizedStringList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, value)
	}
	return cleaned
}

func normalizedStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cleaned := map[string]string{}
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			cleaned[key] = value
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func normalizedTelemetryProvider(provider string) string {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "prometheus":
		return "prometheus"
	default:
		return "prometheus"
	}
}

func normalizedTelemetryAuthMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "bearer":
		return "bearer"
	case "basic":
		return "basic"
	case "custom_header":
		return "custom_header"
	default:
		return "none"
	}
}

func normalizedMCPPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/mcp"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func accessKey(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		cleaned = append(cleaned, strings.TrimSpace(part))
	}
	return strings.Join(cleaned, "\x00")
}

func (r *MemoryRepository) accessGroupMemberCountLocked(groupID string) int {
	count := 0
	for _, member := range r.groupMembers {
		if member.GroupID == groupID {
			count++
		}
	}
	return count
}

func normalizedAIProvider(provider string) string {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "anthropic":
		return "anthropic"
	case "openrouter":
		return "openrouter"
	case "custom":
		return "custom"
	default:
		return "openai"
	}
}

func defaultAIModel(provider string) string {
	switch normalizedAIProvider(provider) {
	case "anthropic":
		return "claude-3-5-sonnet-latest"
	case "openrouter":
		return "openai/gpt-4.1"
	default:
		return "gpt-4.1"
	}
}

func normalizedDocTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Untitled doc"
	}
	return title
}

func normalizedUserRole(role string) string {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "admin":
		return "admin"
	case "architect":
		return "architect"
	case "reviewer":
		return "reviewer"
	default:
		return "member"
	}
}

func normalizedUserStatus(status string) string {
	if strings.TrimSpace(strings.ToLower(status)) == "disabled" {
		return "disabled"
	}
	return "active"
}

func normalizedScopes(scopes string) string {
	scopes = strings.Join(strings.Fields(scopes), " ")
	if scopes == "" {
		return "openid profile email groups"
	}
	return scopes
}

func normalizedDocFormat(format string) string {
	format = strings.TrimSpace(strings.ToLower(format))
	if format == "" {
		return "html"
	}
	return format
}

func normalizedReviewStatus(status string, fallback string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "requested", "pending":
		return "requested"
	case "approved":
		return "approved"
	case "changes_requested":
		return "changes_requested"
	default:
		if fallback == "" {
			return "requested"
		}
		return fallback
	}
}

func normalizeReviewerIDs(reviewerIDs []string) []string {
	seen := make(map[string]struct{}, len(reviewerIDs))
	normalized := make([]string, 0, len(reviewerIDs))
	for _, reviewerID := range reviewerIDs {
		reviewerID = strings.TrimSpace(reviewerID)
		if reviewerID == "" {
			continue
		}
		if _, ok := seen[reviewerID]; ok {
			continue
		}
		seen[reviewerID] = struct{}{}
		normalized = append(normalized, reviewerID)
	}
	return normalized
}

func (r *MemoryRepository) resolveDesignVersionLocked(workspaceID string, designID string, versionID string, fallbackVersionNumber int) (domain.DesignVersion, error) {
	versionID = strings.TrimSpace(versionID)
	for _, version := range r.versions[designID] {
		if version.WorkspaceID != workspaceID {
			continue
		}
		if versionID != "" && version.ID == versionID {
			return version, nil
		}
		if versionID == "" && fallbackVersionNumber > 0 && version.VersionNumber == fallbackVersionNumber {
			return version, nil
		}
	}
	if versionID == "" {
		return domain.DesignVersion{}, errors.New("design version is required before requesting review")
	}
	return domain.DesignVersion{}, errors.New("version not found")
}

func (r *MemoryRepository) findReviewForVersionReviewerLocked(workspaceID string, designID string, versionID string, reviewerID string) (domain.DesignReviewRequest, bool) {
	for _, review := range r.reviews {
		if review.WorkspaceID == workspaceID && review.DesignID == designID && review.VersionID == versionID && review.ReviewerID == reviewerID {
			return review, true
		}
	}
	return domain.DesignReviewRequest{}, false
}

func (r *MemoryRepository) markVersionReviewedIfApprovedLocked(workspaceID string, designID string, versionID string, now time.Time) {
	reviewCount := 0
	for _, review := range r.reviews {
		if review.WorkspaceID != workspaceID || review.DesignID != designID || review.VersionID != versionID {
			continue
		}
		reviewCount++
		if review.Status != "approved" {
			return
		}
	}
	if reviewCount == 0 {
		return
	}
	nextStatus := lifecycle.StatusAfterAllReviewsApproved(lifecycle.VersionPendingReview, reviewCount, 0)
	for index := range r.versions[designID] {
		version := &r.versions[designID][index]
		if version.ID == versionID && version.WorkspaceID == workspaceID && version.Status == lifecycle.VersionPendingReview {
			version.Status = nextStatus
			version.UpdatedAt = now
			return
		}
	}
}

func initialDesignDocument(document []byte, designID string, name string, now time.Time) (json.RawMessage, error) {
	if len(document) > 0 {
		if !json.Valid(document) {
			return nil, errors.New("design document must be valid JSON")
		}
		if strings.TrimSpace(string(document)) == "null" {
			document = nil
		} else {
			return append([]byte(nil), document...), nil
		}
	}

	parsed := map[string]any{
		"schemaVersion": "sde-ui/v0.1",
		"requirementBrief": map[string]any{
			"useCase":                   "",
			"functionalRequirements":    "",
			"nonFunctionalRequirements": "",
			"targetRps":                 nil,
			"availabilityRequirement":   "",
			"sla":                       "",
			"problemStatement":          "",
			"primaryActors":             "",
			"trafficNotes":              "",
			"consistencyNotes":          "",
			"openQuestions":             "",
		},
		"components": []any{},
		"connectors": []any{},
		"journeys":   []any{},
	}
	parsed["id"] = designID
	parsed["title"] = name
	parsed["updatedAt"] = now.Format(time.RFC3339Nano)
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}
