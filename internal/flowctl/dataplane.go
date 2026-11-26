package flowctl

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Control plane queries reading the data plane of a live entry: its ID, then the name of that ID, else the data planes
// the storage mappings of its prefixes list.
const (
	liveDataPlaneQuery    string = `query($names:[Name!]){ liveSpecs(by:{names:$names}){ edges{ node{ catalogName liveSpec{ dataPlaneId } } } } }`
	dataPlaneNamesQuery   string = `query($ids:[Id!]){ dataPlanes(filter:{id:{in:$ids}}){ edges{ node{ id name } } } }`
	mappedDataPlanesQuery string = `query($prefixes:[String!]){ storageMappings(filter:{catalogPrefix:{in:$prefixes}}){ edges{ node{ catalogPrefix dataPlanes{ id name } } } } }`
)

// dataPlane is a data plane as the control plane returns it.
type dataPlane struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// graphqlData is what the data plane queries return, each filling its own field.
type graphqlData struct {
	LiveSpecs struct {
		Edges []struct {
			Node struct {
				CatalogName string `json:"catalogName"`
				LiveSpec    struct {
					DataPlaneID string `json:"dataPlaneId"`
				} `json:"liveSpec"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"liveSpecs"`
	DataPlanes struct {
		Edges []struct {
			Node dataPlane `json:"node"`
		} `json:"edges"`
	} `json:"dataPlanes"`
	StorageMappings struct {
		Edges []struct {
			Node struct {
				DataPlanes []dataPlane `json:"dataPlanes"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"storageMappings"`
}

// graphql runs a control plane query with variables and returns its data. Flowctl fails on GraphQL errors.
func (client *Client) graphql(ctx context.Context, query string, variables map[string]any) (graphqlData, error) {
	var response struct {
		Data graphqlData `json:"data"`
	}

	encoded, err := json.Marshal(variables)

	if err != nil {
		return response.Data, fmt.Errorf("encoding the query variables: %w", err)
	}

	output, err := client.execCommand(ctx, "raw", "graphql", flagValue("--query", query), flagValue("--variables", string(encoded)))

	if err != nil {
		return response.Data, err
	}

	if err := json.Unmarshal(output, &response); err != nil {
		return response.Data, fmt.Errorf("decoding the response of flowctl raw graphql: %w", err)
	}

	return response.Data, nil
}

// prefixes returns every prefix of a catalog name, shortest first, as storage mappings name them.
func prefixes(name string) []string {
	var prefixes []string

	for index, character := range name {
		if character == '/' {
			prefixes = append(prefixes, name[:index+1])
		}
	}

	return prefixes
}

// ReadDataPlane returns the name of the data plane the live entry runs in, empty without an entry or a name. The name
// of its data plane ID is read, once, from the data planes, else from the storage mappings of the entry's prefixes.
func (client *Client) ReadDataPlane(ctx context.Context, name string) (string, error) {
	var id string

	live, err := client.graphql(ctx, liveDataPlaneQuery, map[string]any{"names": []string{name}})

	if err != nil {
		return "", err
	}

	for _, edge := range live.LiveSpecs.Edges {
		if edge.Node.CatalogName == name {
			id = edge.Node.LiveSpec.DataPlaneID
		}
	}

	if id == "" {
		return "", nil
	}

	if cached, found := client.dataPlaneNames.Load(id); found {
		return cached.(string), nil
	}

	named, err := client.graphql(ctx, dataPlaneNamesQuery, map[string]any{"ids": []string{id}})

	if err != nil {
		return "", err
	}

	for _, edge := range named.DataPlanes.Edges {
		client.cacheDataPlane(edge.Node)
	}

	if cached, found := client.dataPlaneNames.Load(id); found {
		return cached.(string), nil
	}

	mapped, err := client.graphql(ctx, mappedDataPlanesQuery, map[string]any{"prefixes": prefixes(name)})

	if err != nil {
		return "", err
	}

	for _, edge := range mapped.StorageMappings.Edges {
		for _, plane := range edge.Node.DataPlanes {
			client.cacheDataPlane(plane)
		}
	}

	cached, _ := client.dataPlaneNames.Load(id)
	resolved, _ := cached.(string)

	return resolved, nil
}

// cacheDataPlane caches the name of a data plane by ID, skipping one the control plane returned without either.
func (client *Client) cacheDataPlane(plane dataPlane) {
	if plane.ID != "" && strings.TrimSpace(plane.Name) != "" {
		client.dataPlaneNames.Store(plane.ID, plane.Name)
	}
}
