package mongodb

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
)

// CaptureMode controls how a MongoDB collection is captured, change streams being the default.
type CaptureMode string

// Image is the MongoDB connector image.
const Image string = "ghcr.io/estuary/source-mongodb:v3"

// ConnectorName ends the catalog name of every task of the connector, as the dashboard names them.
const ConnectorName string = "source-mongodb"

const (
	CaptureModeChangeStream     CaptureMode = "Change Stream Incremental"
	CaptureModeBatchSnapshot    CaptureMode = "Batch Snapshot"
	CaptureModeBatchIncremental CaptureMode = "Batch Incremental"
)

// CaptureModes lists every supported capture mode, useful for schema validators.
var CaptureModes = []CaptureMode{
	CaptureModeChangeStream,
	CaptureModeBatchSnapshot,
	CaptureModeBatchIncremental,
}

// Advanced carries the advanced endpoint options, including the change stream and backfill settings.
// ExcludeCollections cannot be set together with ExclusiveCollectionFilter.
type Advanced struct {
	MaxAwaitTime              string `yaml:"maxAwaitTime,omitempty"`
	DisablePreImages          bool   `yaml:"disablePreImages,omitempty"`
	ExclusiveCollectionFilter bool   `yaml:"exclusiveCollectionFilter,omitempty"`
	ExcludeCollections        string `yaml:"excludeCollections,omitempty"`
	SkipBackfills             string `yaml:"skipBackfills,omitempty"`
}

// Config carries the ghcr.io/estuary/source-mongodb connector configurations, including
// the deployment connection, the discovered databases and the batch polling schedule.
type Config struct {
	Address              string                  `yaml:"address"`
	User                 string                  `yaml:"user"`
	Password             string                  `yaml:"password"`
	Database             string                  `yaml:"database,omitempty"`
	BatchAndChangeStream bool                    `yaml:"batchAndChangeStream,omitempty"`
	PollSchedule         string                  `yaml:"pollSchedule,omitempty"`
	NetworkTunnel        *endpoint.NetworkTunnel `yaml:"networkTunnel,omitempty"`
	Advanced             *Advanced               `yaml:"advanced,omitempty"`
}

// Resource identifies the captured MongoDB collection, batch capture modes require a CursorField.
type Resource struct {
	Database     string      `yaml:"database"`
	Collection   string      `yaml:"collection"`
	CaptureMode  CaptureMode `yaml:"captureMode,omitempty"`
	CursorField  string      `yaml:"cursorField,omitempty"`
	PollSchedule string      `yaml:"pollSchedule,omitempty"`
}

// RecommendedName returns database/collection, the collection name the connector discovery recommends.
func (resource Resource) RecommendedName() string {
	return resource.Database + "/" + resource.Collection
}

// Binding maps a captured MongoDB resource to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full MongoDB capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
