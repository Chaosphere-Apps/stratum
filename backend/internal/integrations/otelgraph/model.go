package otelgraph

import "time"

const (
	ComponentWebClient   = "client.web"
	ComponentAPIGateway  = "edge.api_gateway"
	ComponentService     = "compute.service"
	ComponentSQLDatabase = "data.sql_database"
	ComponentRedis       = "data.redis"
	ComponentQueue       = "messaging.queue"
	ComponentExternalAPI = "external.api"

	ConnectorSynchronous = "synchronous"
	ConnectorAsyncEvent  = "asynchronous_event"
)

type ObservedServiceGraph struct {
	Source      string
	WindowStart time.Time
	WindowEnd   time.Time
	Services    []ObservedService
	Calls       []ObservedCall
	Warnings    []string
}

type ObservedService struct {
	Key             string
	Name            string
	Namespace       string
	Labels          map[string]string
	FirstSeenSource string
}

type ObservedCall struct {
	From           string
	To             string
	ConnectionType string
	RequestRate    *float64
	ErrorRate      *float64
	P95LatencyMs   *float64
	Labels         map[string]string
}

type NormalizedTopology struct {
	Source      string
	WindowStart time.Time
	WindowEnd   time.Time
	Nodes       []NormalizedNode
	Edges       []NormalizedEdge
	Warnings    []string
}

type NormalizedNode struct {
	ExternalKey string
	Name        string
	Namespace   string
	Labels      map[string]string
}

type NormalizedEdge struct {
	ExternalKey     string
	FromExternalKey string
	ToExternalKey   string
	ConnectionType  string
	RequestRate     *float64
	ErrorRate       *float64
	P95LatencyMs    *float64
	Labels          map[string]string
}

type ProjectionPolicy struct {
	DefaultComponentType string
	DefaultConnectorType string
	CatalogCandidates    []CatalogCandidate
	IntegrationID        string
	Source               string
}

type CatalogCandidate struct {
	ID             string
	Name           string
	NormalizedName string
	Type           string
	Aliases        []string
	Labels         map[string]string
}

type StratumTopologyProjection struct {
	Components []ProjectedComponent
	Connectors []ProjectedConnector
	Matches    []ProjectionMatch
	Warnings   []string
}

type ProjectedComponent struct {
	ExternalKey    string
	Name           string
	Type           string
	Confidence     float64
	CatalogAssetID *string
	Metadata       map[string]any
}

type ProjectedConnector struct {
	ExternalKey     string
	FromExternalKey string
	ToExternalKey   string
	Type            string
	Protocol        string
	Metrics         map[string]float64
	Metadata        map[string]any
}

type ProjectionMatch struct {
	ExternalKey    string
	CatalogAssetID string
	MatchType      string
	Confidence     float64
	Reason         string
}
