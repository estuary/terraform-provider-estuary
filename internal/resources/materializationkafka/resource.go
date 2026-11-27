package materializationkafka

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/kafka"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// New returns the estuary_materialization_kafka resource.
var New func() resource.Resource = builder.NewMaterialization[kafka.Config, kafka.Resource](builder.Connector{
	TypeName:      "materialization_kafka",
	Image:         kafka.Image,
	ConnectorName: kafka.ConnectorName,
	Description:   "Materializes Estuary collections into Kafka topics, as Avro or JSON messages.",
})
