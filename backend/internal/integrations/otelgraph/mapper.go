package otelgraph

import (
	"sort"
	"strings"
)

func Project(topology NormalizedTopology, policy ProjectionPolicy) StratumTopologyProjection {
	if policy.DefaultComponentType == "" {
		policy.DefaultComponentType = ComponentService
	}
	if policy.DefaultConnectorType == "" {
		policy.DefaultConnectorType = ConnectorSynchronous
	}
	if policy.Source == "" {
		policy.Source = topology.Source
	}

	projection := StratumTopologyProjection{
		Components: make([]ProjectedComponent, 0, len(topology.Nodes)),
		Connectors: make([]ProjectedConnector, 0, len(topology.Edges)),
		Warnings:   append([]string{}, topology.Warnings...),
	}
	for _, node := range topology.Nodes {
		componentType, confidence := inferComponentType(node, policy.DefaultComponentType)
		component := ProjectedComponent{
			ExternalKey: node.ExternalKey,
			Name:        node.Name,
			Type:        componentType,
			Confidence:  confidence,
			Metadata: map[string]any{
				"observed": observedMetadata(policy, topology, node.ExternalKey, node.Labels, confidence),
			},
		}
		if match, ok := matchCatalog(node, policy.CatalogCandidates); ok {
			component.CatalogAssetID = &match.CatalogAssetID
			component.Type = matchTypeOrDefault(match, component.Type, policy.CatalogCandidates)
			projection.Matches = append(projection.Matches, match)
		}
		projection.Components = append(projection.Components, component)
	}
	for _, edge := range topology.Edges {
		connectorType, protocol := inferConnector(edge, policy.DefaultConnectorType)
		metrics := make(map[string]float64)
		if edge.RequestRate != nil {
			metrics["requestRate"] = *edge.RequestRate
		}
		if edge.ErrorRate != nil {
			metrics["errorRate"] = *edge.ErrorRate
		}
		if edge.P95LatencyMs != nil {
			metrics["p95LatencyMs"] = *edge.P95LatencyMs
		}
		projection.Connectors = append(projection.Connectors, ProjectedConnector{
			ExternalKey:     edge.ExternalKey,
			FromExternalKey: edge.FromExternalKey,
			ToExternalKey:   edge.ToExternalKey,
			Type:            connectorType,
			Protocol:        protocol,
			Metrics:         metrics,
			Metadata: map[string]any{
				"observed": observedMetadata(policy, topology, edge.ExternalKey, edge.Labels, 0.8),
			},
		})
	}
	sort.Slice(projection.Matches, func(i, j int) bool {
		return projection.Matches[i].ExternalKey < projection.Matches[j].ExternalKey
	})
	return projection
}

func inferComponentType(node NormalizedNode, defaultType string) (string, float64) {
	labels := lowerLabels(node.Labels)
	key := strings.ToLower(node.ExternalKey + " " + node.Name)
	if containsAny(labels["db.system"], "redis") || containsAny(key, "redis", "cache") {
		return ComponentRedis, 0.82
	}
	if labels["db.system"] != "" || labels["db.name"] != "" || containsAny(key, "postgres", "mysql", "mariadb", "oracle", "sqlserver") {
		return ComponentSQLDatabase, 0.86
	}
	if labels["messaging.system"] != "" || labels["messaging.destination"] != "" || containsAny(key, "queue", "kafka", "rabbit", "sqs", "pubsub") {
		return ComponentQueue, 0.84
	}
	if containsAny(labels["http.route"], "gateway") || containsAny(key, "gateway", "ingress", "edge") {
		return ComponentAPIGateway, 0.74
	}
	if containsAny(labels["component.scope"], "external") || containsAny(key, "external", "partner", "third-party") {
		return ComponentExternalAPI, 0.72
	}
	return defaultType, 0.65
}

func inferConnector(edge NormalizedEdge, defaultType string) (string, string) {
	connection := strings.ToLower(edge.ConnectionType)
	labels := lowerLabels(edge.Labels)
	if containsAny(connection, "messaging", "queue", "event", "async") || labels["messaging.system"] != "" {
		return ConnectorAsyncEvent, firstNonEmpty(labels["messaging.system"], "event")
	}
	if protocol := firstNonEmpty(labels["rpc.system"], labels["http.scheme"], labels["network.protocol.name"]); protocol != "" {
		return defaultType, protocol
	}
	return defaultType, firstNonEmpty(connection, "request")
}

func observedMetadata(policy ProjectionPolicy, topology NormalizedTopology, externalKey string, labels map[string]string, confidence float64) map[string]any {
	return map[string]any{
		"source":        policy.Source,
		"integrationId": policy.IntegrationID,
		"externalKey":   externalKey,
		"windowStart":   topology.WindowStart,
		"windowEnd":     topology.WindowEnd,
		"labels":        labels,
		"confidence":    confidence,
	}
}

func matchCatalog(node NormalizedNode, candidates []CatalogCandidate) (ProjectionMatch, bool) {
	for _, candidate := range candidates {
		if candidate.NormalizedName != "" && NormalizeKey(candidate.NormalizedName) == node.ExternalKey {
			return ProjectionMatch{
				ExternalKey:    node.ExternalKey,
				CatalogAssetID: candidate.ID,
				MatchType:      "normalized_name",
				Confidence:     0.96,
				Reason:         "Observed service normalized name matches catalog asset.",
			}, true
		}
		if NormalizeKey(candidate.Name) == node.ExternalKey {
			return ProjectionMatch{
				ExternalKey:    node.ExternalKey,
				CatalogAssetID: candidate.ID,
				MatchType:      "name",
				Confidence:     0.92,
				Reason:         "Observed service name matches catalog asset.",
			}, true
		}
		for _, alias := range candidate.Aliases {
			if NormalizeKey(alias) == node.ExternalKey {
				return ProjectionMatch{
					ExternalKey:    node.ExternalKey,
					CatalogAssetID: candidate.ID,
					MatchType:      "alias",
					Confidence:     0.88,
					Reason:         "Observed service name matches catalog alias.",
				}, true
			}
		}
	}
	return ProjectionMatch{}, false
}

func matchTypeOrDefault(match ProjectionMatch, fallback string, candidates []CatalogCandidate) string {
	for _, candidate := range candidates {
		if candidate.ID == match.CatalogAssetID && candidate.Type != "" {
			return candidate.Type
		}
	}
	return fallback
}

func lowerLabels(labels map[string]string) map[string]string {
	lowered := make(map[string]string, len(labels))
	for key, value := range labels {
		lowered[strings.ToLower(strings.TrimSpace(key))] = strings.ToLower(strings.TrimSpace(value))
	}
	return lowered
}

func containsAny(value string, needles ...string) bool {
	value = strings.ToLower(value)
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
