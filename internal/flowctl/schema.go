package flowctl

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
)

type connectorTagRow struct {
	Endpoint map[string]any `json:"endpoint_spec_schema"`
	Resource map[string]any `json:"resource_spec_schema"`
}

// ConnectorSchema returns the schemas published for the image, which must carry its tag.
func (client *Client) ConnectorSchema(ctx context.Context, image string) (*catalog.ConnectorSchema, error) {
	var separator int = strings.LastIndex(image, ":")

	if separator <= strings.LastIndex(image, "/") {
		return nil, fmt.Errorf("image %q has no tag", image)
	}

	output, err := client.execCommand(ctx, nil, "raw", "get", "--table", "connector_tags",
		"--query", "select=endpoint_spec_schema,resource_spec_schema,connectors!inner(image_name)",
		"--query", "connectors.image_name=eq."+image[:separator],
		"--query", "image_tag=eq."+image[separator:],
		"-o", "json")

	if err != nil {
		return nil, err
	}

	return parseConnectorSchema(output, image)
}

func parseConnectorSchema(output []byte, image string) (*catalog.ConnectorSchema, error) {
	var rows []connectorTagRow

	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("parsing connector schema of %s: %w", image, err)
	}

	if len(rows) != 1 {
		return nil, fmt.Errorf("found %d published tags for %s, want 1", len(rows), image)
	}

	return &catalog.ConnectorSchema{
		Endpoint: rows[0].Endpoint,
		Resource: rows[0].Resource,
	}, nil
}
