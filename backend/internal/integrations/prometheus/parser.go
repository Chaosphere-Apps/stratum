package prometheus

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/system-design-evaluator/backend/internal/integrations"
	"github.com/system-design-evaluator/backend/internal/integrations/otelgraph"
)

type QueryBundle struct {
	RequestTotal  []byte
	RequestFailed []byte
	ServerP95     []byte
	ClientP95     []byte
}

type prometheusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		ResultType string             `json:"resultType"`
		Result     []prometheusVector `json:"result"`
	} `json:"data"`
}

type prometheusVector struct {
	Metric map[string]string `json:"metric"`
	Value  []json.RawMessage `json:"value"`
}

type callAccumulator struct {
	labels          map[string]string
	requestCount    *float64
	failureCount    *float64
	serverP95Millis *float64
	clientP95Millis *float64
}

func ParseServiceGraph(bundle QueryBundle, source string, windowStart time.Time, windowEnd time.Time, filters integrations.FilterConfig) (otelgraph.ObservedServiceGraph, error) {
	calls := make(map[string]*callAccumulator)
	warnings := make([]string, 0)
	if len(bundle.RequestTotal) > 0 {
		parsedWarnings, err := mergeVector(calls, bundle.RequestTotal, func(acc *callAccumulator, value float64, _ time.Duration) {
			acc.requestCount = floatPtr(value)
		})
		if err != nil {
			return otelgraph.ObservedServiceGraph{}, fmt.Errorf("parse request total: %w", err)
		}
		warnings = append(warnings, parsedWarnings...)
	}
	if len(bundle.RequestFailed) > 0 {
		parsedWarnings, err := mergeVector(calls, bundle.RequestFailed, func(acc *callAccumulator, value float64, _ time.Duration) {
			acc.failureCount = floatPtr(value)
		})
		if err != nil {
			return otelgraph.ObservedServiceGraph{}, fmt.Errorf("parse request failed: %w", err)
		}
		warnings = append(warnings, parsedWarnings...)
	}
	if len(bundle.ServerP95) > 0 {
		parsedWarnings, err := mergeVector(calls, bundle.ServerP95, func(acc *callAccumulator, value float64, _ time.Duration) {
			acc.serverP95Millis = floatPtr(value * 1000)
		})
		if err != nil {
			return otelgraph.ObservedServiceGraph{}, fmt.Errorf("parse server p95: %w", err)
		}
		warnings = append(warnings, parsedWarnings...)
	}
	if len(bundle.ClientP95) > 0 {
		parsedWarnings, err := mergeVector(calls, bundle.ClientP95, func(acc *callAccumulator, value float64, _ time.Duration) {
			acc.clientP95Millis = floatPtr(value * 1000)
		})
		if err != nil {
			return otelgraph.ObservedServiceGraph{}, fmt.Errorf("parse client p95: %w", err)
		}
		warnings = append(warnings, parsedWarnings...)
	}

	window := windowEnd.Sub(windowStart)
	services := make(map[string]otelgraph.ObservedService)
	observedCalls := make([]otelgraph.ObservedCall, 0, len(calls))
	for key, acc := range calls {
		client, server, connectionType := splitCallKey(key)
		if client == "" || server == "" {
			continue
		}
		services[otelgraph.NormalizeKey(client)] = observedService(client, acc.labels)
		services[otelgraph.NormalizeKey(server)] = observedService(server, acc.labels)
		requestRate := ratePtr(acc.requestCount, window)
		errorRate := ratePtr(acc.failureCount, window)
		latency := acc.serverP95Millis
		if latency == nil {
			latency = acc.clientP95Millis
		}
		call := otelgraph.ObservedCall{
			From:           client,
			To:             server,
			ConnectionType: connectionType,
			RequestRate:    requestRate,
			ErrorRate:      errorRate,
			P95LatencyMs:   latency,
			Labels:         acc.labels,
		}
		if shouldSkipByEndpoint(call, filters) {
			continue
		}
		observedCalls = append(observedCalls, call)
	}

	observedServices := make([]otelgraph.ObservedService, 0, len(services))
	for _, service := range services {
		observedServices = append(observedServices, service)
	}
	return otelgraph.ObservedServiceGraph{
		Source:      source,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Services:    observedServices,
		Calls:       observedCalls,
		Warnings:    warnings,
	}, nil
}

func mergeVector(calls map[string]*callAccumulator, payload []byte, apply func(*callAccumulator, float64, time.Duration)) ([]string, error) {
	var response prometheusResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	if response.Status != "" && response.Status != "success" {
		return nil, fmt.Errorf("prometheus response status %q: %s", response.Status, response.Error)
	}
	warnings := make([]string, 0)
	for _, result := range response.Data.Result {
		client := strings.TrimSpace(result.Metric["client"])
		server := strings.TrimSpace(result.Metric["server"])
		connectionType := strings.TrimSpace(result.Metric["connection_type"])
		if client == "" || server == "" {
			warnings = append(warnings, "ignored service graph metric without client or server label")
			continue
		}
		value, err := vectorValue(result)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("ignored service graph metric for %s -> %s with invalid value", client, server))
			continue
		}
		key := callKey(client, server, connectionType)
		acc := calls[key]
		if acc == nil {
			acc = &callAccumulator{labels: copyLabels(result.Metric)}
			calls[key] = acc
		}
		for label, labelValue := range result.Metric {
			acc.labels[label] = labelValue
		}
		apply(acc, value, 0)
	}
	return warnings, nil
}

func vectorValue(vector prometheusVector) (float64, error) {
	if len(vector.Value) < 2 {
		return 0, fmt.Errorf("missing vector value")
	}
	var encoded string
	if err := json.Unmarshal(vector.Value[1], &encoded); err != nil {
		return 0, err
	}
	return strconv.ParseFloat(encoded, 64)
}

func callKey(client string, server string, connectionType string) string {
	return strings.Join([]string{client, server, connectionType}, "\x00")
}

func splitCallKey(key string) (string, string, string) {
	parts := strings.SplitN(key, "\x00", 3)
	if len(parts) != 3 {
		return "", "", ""
	}
	return parts[0], parts[1], parts[2]
}

func observedService(name string, labels map[string]string) otelgraph.ObservedService {
	namespace := firstNonEmpty(labels["client_namespace"], labels["server_namespace"], labels["namespace"], labels["k8s.namespace.name"])
	return otelgraph.ObservedService{
		Key:             name,
		Name:            name,
		Namespace:       namespace,
		Labels:          copyLabels(labels),
		FirstSeenSource: Kind,
	}
}

func ratePtr(count *float64, window time.Duration) *float64 {
	if count == nil || window <= 0 {
		return nil
	}
	value := *count / window.Seconds()
	return &value
}

func shouldSkipByEndpoint(call otelgraph.ObservedCall, filters integrations.FilterConfig) bool {
	for _, blocked := range filters.EndpointBlocklist {
		blocked = strings.ToLower(strings.TrimSpace(blocked))
		if blocked == "" {
			continue
		}
		for _, value := range call.Labels {
			if strings.Contains(strings.ToLower(value), blocked) {
				return true
			}
		}
	}
	return false
}

func floatPtr(value float64) *float64 {
	return &value
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
