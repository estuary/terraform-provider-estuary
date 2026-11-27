package sourcekafka

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/kafka"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var connector builder.Connector = builder.Connector{
	TypeName:      "source_kafka",
	Image:         kafka.Image,
	ConnectorName: kafka.ConnectorName,
	Description:   "Captures the messages of Kafka topics into Estuary collections, decoding Avro and Protobuf through a schema registry, or JSON.",
}

// New returns the estuary_source_kafka resource.
var New func() resource.Resource = builder.NewCapture[kafka.Config, kafka.Resource](connector)

// NewDataSource returns the estuary_source_kafka data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[kafka.Config, kafka.Resource](connector)
