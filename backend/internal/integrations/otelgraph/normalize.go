package otelgraph

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/system-design-evaluator/backend/internal/integrations"
)

var nonKeyCharacter = regexp.MustCompile(`[^a-z0-9._:-]+`)

func Normalize(graph ObservedServiceGraph, filters integrations.FilterConfig) NormalizedTopology {
	nodesByKey := make(map[string]NormalizedNode)
	warnings := append([]string{}, graph.Warnings...)
	for _, service := range graph.Services {
		node := normalizeService(service)
		if node.ExternalKey == "" {
			warnings = append(warnings, "ignored observed service without a name or key")
			continue
		}
		if filteredNode(node, filters) {
			continue
		}
		nodesByKey[node.ExternalKey] = node
	}

	edgesByKey := make(map[string]NormalizedEdge)
	for _, call := range graph.Calls {
		fromKey := NormalizeKey(call.From)
		toKey := NormalizeKey(call.To)
		if fromKey == "" || toKey == "" || fromKey == toKey {
			continue
		}
		if !matchesAllowBlock(fromKey, filters.ServiceAllowlist, filters.ServiceBlocklist) ||
			!matchesAllowBlock(toKey, filters.ServiceAllowlist, filters.ServiceBlocklist) {
			continue
		}
		edge := NormalizedEdge{
			ExternalKey:     edgeKey(fromKey, toKey, call.ConnectionType),
			FromExternalKey: fromKey,
			ToExternalKey:   toKey,
			ConnectionType:  strings.ToLower(strings.TrimSpace(call.ConnectionType)),
			RequestRate:     call.RequestRate,
			ErrorRate:       call.ErrorRate,
			P95LatencyMs:    call.P95LatencyMs,
			Labels:          filterLabels(copyLabels(call.Labels), filters),
		}
		if filteredEdge(edge, filters) {
			continue
		}
		if _, ok := nodesByKey[fromKey]; !ok {
			nodesByKey[fromKey] = NormalizedNode{ExternalKey: fromKey, Name: displayNameFromKey(fromKey)}
		}
		if _, ok := nodesByKey[toKey]; !ok {
			nodesByKey[toKey] = NormalizedNode{ExternalKey: toKey, Name: displayNameFromKey(toKey)}
		}
		edgesByKey[edge.ExternalKey] = edge
	}

	nodes := make([]NormalizedNode, 0, len(nodesByKey))
	for _, node := range nodesByKey {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ExternalKey < nodes[j].ExternalKey })

	edges := make([]NormalizedEdge, 0, len(edgesByKey))
	for _, edge := range edgesByKey {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].ExternalKey < edges[j].ExternalKey })

	return NormalizedTopology{
		Source:      graph.Source,
		WindowStart: graph.WindowStart,
		WindowEnd:   graph.WindowEnd,
		Nodes:       nodes,
		Edges:       edges,
		Warnings:    warnings,
	}
}

func NormalizeKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonKeyCharacter.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-_.:")
	return value
}

func normalizeService(service ObservedService) NormalizedNode {
	name := strings.TrimSpace(service.Name)
	if name == "" {
		name = strings.TrimSpace(service.Key)
	}
	key := NormalizeKey(service.Key)
	if key == "" {
		key = NormalizeKey(name)
	}
	return NormalizedNode{
		ExternalKey: key,
		Name:        firstNonEmpty(name, displayNameFromKey(key)),
		Namespace:   strings.TrimSpace(service.Namespace),
		Labels:      copyLabels(service.Labels),
	}
}

func edgeKey(fromKey string, toKey string, connectionType string) string {
	key := fmt.Sprintf("%s->%s:%s", fromKey, toKey, NormalizeKey(connectionType))
	if len(key) <= 160 {
		return key
	}
	sum := sha1.Sum([]byte(key))
	return fmt.Sprintf("%s->%s:%s", fromKey, toKey, hex.EncodeToString(sum[:])[:12])
}

func displayNameFromKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	parts := strings.FieldsFunc(key, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == ':'
	})
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func filteredNode(node NormalizedNode, filters integrations.FilterConfig) bool {
	if !matchesAllowBlock(node.ExternalKey, filters.ServiceAllowlist, filters.ServiceBlocklist) {
		return true
	}
	if !matchesAllowBlock(node.Namespace, filters.NamespaceAllowlist, filters.NamespaceBlocklist) {
		return true
	}
	if filters.Environment != "" && !labelHasValue(node.Labels, filters.Environment) {
		return true
	}
	return false
}

func filteredEdge(edge NormalizedEdge, filters integrations.FilterConfig) bool {
	if filters.MinimumRequests > 0 && (edge.RequestRate == nil || *edge.RequestRate < filters.MinimumRequests) {
		return true
	}
	if len(filters.ConnectionTypes) > 0 && !containsFold(filters.ConnectionTypes, edge.ConnectionType) {
		return true
	}
	for _, blocked := range filters.EndpointBlocklist {
		if blocked == "" {
			continue
		}
		for _, value := range edge.Labels {
			if strings.Contains(strings.ToLower(value), strings.ToLower(blocked)) {
				return true
			}
		}
	}
	return false
}

func matchesAllowBlock(value string, allowlist []string, blocklist []string) bool {
	value = NormalizeKey(value)
	if value == "" {
		return len(allowlist) == 0
	}
	for _, blocked := range blocklist {
		if patternMatches(value, blocked) {
			return false
		}
	}
	if len(allowlist) == 0 {
		return true
	}
	for _, allowed := range allowlist {
		if patternMatches(value, allowed) {
			return true
		}
	}
	return false
}

func patternMatches(value string, pattern string) bool {
	pattern = NormalizeKey(pattern)
	if pattern == "" {
		return false
	}
	return value == pattern || strings.Contains(value, pattern)
}

func filterLabels(labels map[string]string, filters integrations.FilterConfig) map[string]string {
	if len(labels) == 0 {
		return labels
	}
	filtered := make(map[string]string, len(labels))
	for key, value := range labels {
		if len(filters.LabelAllowlist) > 0 && !containsFold(filters.LabelAllowlist, key) {
			continue
		}
		if containsFold(filters.LabelBlocklist, key) {
			continue
		}
		filtered[key] = value
	}
	return filtered
}

func copyLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	copied := make(map[string]string, len(labels))
	for key, value := range labels {
		copied[key] = value
	}
	return copied
}

func containsFold(values []string, candidate string) bool {
	return slices.ContainsFunc(values, func(value string) bool {
		return strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(candidate))
	})
}

func labelHasValue(labels map[string]string, expected string) bool {
	for _, value := range labels {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(expected)) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
