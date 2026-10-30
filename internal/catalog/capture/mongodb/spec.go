package mongodb

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
)

// CaptureMode controls how a MongoDB collection is captured, change streams being the default.
type CaptureMode string

// Image is the MongoDB connector image.
const Image string = "ghcr.io/estuary/source-mongodb:v3"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
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
	MaxAwaitTime              string `yaml:"maxAwaitTime,omitempty" description:"Maximum time to wait for new change stream events before returning an empty batch, as a Go duration such as '10s'. Defaults to '1s'."`
	DisablePreImages          bool   `yaml:"disablePreImages,omitempty" description:"Stops requesting pre-images, even when the deployment supports them and they are enabled for the collections."`
	ExclusiveCollectionFilter bool   `yaml:"exclusiveCollectionFilter,omitempty" description:"Adds a pipeline filter to the database change streams so they only match events of enabled bindings. Use it only with a small number of bindings. Cannot be set with exclude_collections."`
	ExcludeCollections        string `yaml:"excludeCollections,omitempty" description:"Comma-separated list of collections to exclude from the database change streams, each formatted as database:collection. Cannot be set with exclusive_collection_filter, nor list a collection with an enabled binding."`
	SkipBackfills             string `yaml:"skipBackfills,omitempty" description:"Comma-separated list of collections not to backfill, each formatted as database:collection. Use *:* to skip the backfill of every change stream collection. Only applies to Change Stream Incremental bindings: naming another binding fails validation, while *:* skips only the change stream ones."`
}

// Config is the source-mongodb configuration: connection, discovered databases and batch polling schedule.
type Config struct {
	Address              string                  `yaml:"address" credentials:"forbidden" description:"Connection URI of the deployment, holding only the host and connection options (e.g., 'mongodb+srv://cluster0.123456789.mongodb.net?authSource=admin'). A mongodb:// host can include its port, a mongodb+srv:// host can't. The plan rejects credentials in it, such as user:password@ or a password option, as state and logs would hold them in plain text: set them in user and password. Credentials found in the live value show as ***."`
	User                 string                  `yaml:"user" description:"Database user to connect as."`
	Password             string                  `yaml:"password" description:"Password of the database user. Write-only."`
	Database             string                  `yaml:"database,omitempty" description:"Comma-separated list of databases to discover. If unset, every available database is discovered."`
	BatchAndChangeStream bool                    `yaml:"batchAndChangeStream,omitempty" description:"Also discovers the collections that can only be batch captured, such as views and time series collections, when the deployment supports change streams. Every collection is batch captured when the deployment doesn't support change streams."`
	PollSchedule         string                  `yaml:"pollSchedule,omitempty" description:"Default polling schedule of batch collections: a Go duration such as '5m' or '6h', or a daily time in UTC such as 'daily at 12:34Z'. Defaults to '24h'."`
	NetworkTunnel        *endpoint.NetworkTunnel `yaml:"networkTunnel,omitempty" description:"Connects through an SSH server acting as a bastion host."`
	Advanced             *Advanced               `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
}

// Resource identifies the captured MongoDB collection, batch capture modes require a CursorField.
type Resource struct {
	Database     string      `yaml:"database" description:"Database of the captured collection."`
	Collection   string      `yaml:"collection" description:"Name of the captured collection."`
	CaptureMode  CaptureMode `yaml:"captureMode,omitempty" enum:"Change Stream Incremental,Batch Snapshot,Batch Incremental" description:"How the collection is captured: 'Change Stream Incremental' (default), 'Batch Snapshot' or 'Batch Incremental'."`
	CursorField  string      `yaml:"cursorField,omitempty" description:"Field used as the cursor of batch capture modes, which require it. It should be indexed. With 'Batch Incremental', every document must have it and its values must strictly increase."`
	PollSchedule string      `yaml:"pollSchedule,omitempty" description:"Polling schedule of a batch binding, overriding config.poll_schedule. Same format, defaults to config.poll_schedule."`
}

// ResourcePath returns the database and collection, satisfying capture.ResourcePather.
func (resource Resource) ResourcePath() []string {
	return []string{resource.Database, resource.Collection}
}

// Binding maps a captured MongoDB resource to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full MongoDB capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
