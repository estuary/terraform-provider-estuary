package flowctl

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// graphqlFlowctl records each call and answers the data plane queries with the responses in its environment, failing
// the queries holding FLOWCTL_FAILING.
const graphqlFlowctl string = `#!/bin/sh
echo "$*" >> "$FLOWCTL_CALLS"
if [ -n "$FLOWCTL_FAILING" ]; then
	case "$*" in *"$FLOWCTL_FAILING"*) echo "control plane unreachable" >&2; exit 1 ;; esac
fi
case "$*" in
	*liveSpecs*) echo "$FLOWCTL_LIVE_SPECS" ;;
	*storageMappings*) echo "$FLOWCTL_STORAGE_MAPPINGS" ;;
	*dataPlanes*) echo "$FLOWCTL_DATA_PLANES" ;;
esac
`

// Responses of the control plane to the data plane queries.
const (
	liveSpecsResponse       string = `{"data":{"liveSpecs":{"edges":[{"node":{"catalogName":"acmeCo/shop/source-postgres","liveSpec":{"dataPlaneId":"0eae476e9d7f1c00"}}}]}}}`
	noLiveSpecsResponse     string = `{"data":{"liveSpecs":{"edges":[]}}}`
	dataPlanesResponse      string = `{"data":{"dataPlanes":{"edges":[{"node":{"id":"0eae476e9d7f1c00","name":"ops/dp/private/acmeCo/example-data-plane"}}]}}}`
	noDataPlanesResponse    string = `{"data":{"dataPlanes":{"edges":[]}}}`
	storageMappingsResponse string = `{"data":{"storageMappings":{"edges":[{"node":{"catalogPrefix":"acmeCo/","dataPlanes":[{"id":"15fcff17155ef800","name":"ops/dp/public/other"},{"id":"0eae476e9d7f1c00","name":"ops/dp/private/acmeCo/example-data-plane"}]}}]}}}`
	noMappedPlanesResponse  string = `{"data":{"storageMappings":{"edges":[{"node":{"catalogPrefix":"acmeCo/","dataPlanes":[{"id":"15fcff17155ef800","name":"ops/dp/public/other"}]}}]}}}`
)

// graphqlClient returns a client of graphqlFlowctl answering with the responses, and the file recording its calls.
func graphqlClient(test *testing.T, liveSpecs string, dataPlanes string, storageMappings string) (*Client, string) {
	var calls string = filepath.Join(test.TempDir(), "calls")

	test.Setenv("FLOWCTL_CALLS", calls)
	test.Setenv("FLOWCTL_FAILING", "")
	test.Setenv("FLOWCTL_LIVE_SPECS", liveSpecs)
	test.Setenv("FLOWCTL_DATA_PLANES", dataPlanes)
	test.Setenv("FLOWCTL_STORAGE_MAPPINGS", storageMappings)

	return fakeFlowctl(test, graphqlFlowctl), calls
}

// recordedCalls returns the calls the fake flowctl recorded, one per line.
func recordedCalls(test *testing.T, calls string) []string {
	test.Helper()

	content, err := os.ReadFile(calls)

	if err != nil {
		test.Fatal(err)
	}

	return strings.Split(strings.TrimSpace(string(content)), "\n")
}

func TestReadDataPlaneReadsTheLiveNameAndCachesIt(test *testing.T) {
	client, calls := graphqlClient(test, liveSpecsResponse, dataPlanesResponse, storageMappingsResponse)

	for range 2 {
		dataPlane, err := client.ReadDataPlane(context.Background(), "acmeCo/shop/source-postgres")

		if err != nil || dataPlane != testDataPlane {
			test.Fatalf("data plane = %q, %v, want %s", dataPlane, err, testDataPlane)
		}
	}

	assertCommandLines(test, recordedCalls(test, calls), []string{
		`raw graphql --query=` + liveDataPlaneQuery + ` --variables={"names":["acmeCo/shop/source-postgres"]}`,
		`raw graphql --query=` + dataPlaneNamesQuery + ` --variables={"ids":["0eae476e9d7f1c00"]}`,
		`raw graphql --query=` + liveDataPlaneQuery + ` --variables={"names":["acmeCo/shop/source-postgres"]}`,
	})
}

func TestReadDataPlaneFallsBackToTheStorageMappings(test *testing.T) {
	client, calls := graphqlClient(test, liveSpecsResponse, noDataPlanesResponse, storageMappingsResponse)

	dataPlane, err := client.ReadDataPlane(context.Background(), "acmeCo/shop/source-postgres")

	if err != nil || dataPlane != testDataPlane {
		test.Fatalf("data plane = %q, %v, want %s from the storage mapping", dataPlane, err, testDataPlane)
	}

	if lines := recordedCalls(test, calls); len(lines) != 3 || lines[2] != `raw graphql --query=`+mappedDataPlanesQuery+` --variables={"prefixes":["acmeCo/","acmeCo/shop/"]}` {
		test.Errorf("calls = %q, want the storage mappings of every prefix of the name read last", lines)
	}
}

func TestReadDataPlaneUnreadNamesAreEmpty(test *testing.T) {
	expectations := map[string]struct {
		liveSpecs string
		calls     int
	}{
		"no entry has the name":           {noLiveSpecsResponse, 1},
		"no query names the data plane":   {liveSpecsResponse, 3},
		"another entry than the one read": {strings.ReplaceAll(liveSpecsResponse, "acmeCo/shop/", "acmeCo/other/"), 1},
	}

	for name, expected := range expectations {
		client, calls := graphqlClient(test, expected.liveSpecs, noDataPlanesResponse, noMappedPlanesResponse)

		dataPlane, err := client.ReadDataPlane(context.Background(), "acmeCo/shop/source-postgres")

		if err != nil || dataPlane != "" {
			test.Errorf("%s: data plane = %q, %v, want none and no error", name, dataPlane, err)
		}

		if lines := recordedCalls(test, calls); len(lines) != expected.calls {
			test.Errorf("%s: calls = %q, want %d", name, lines, expected.calls)
		}
	}
}

func TestReadDataPlaneFailsOnAnUnreadableResponse(test *testing.T) {
	client, _ := graphqlClient(test, "", dataPlanesResponse, storageMappingsResponse)

	if dataPlane, err := client.ReadDataPlane(context.Background(), "acmeCo/shop/source-postgres"); err == nil || dataPlane != "" {
		test.Errorf("data plane = %q, %v, want the undecodable response to fail", dataPlane, err)
	}
}

func TestReadDataPlaneFailsWhenAQueryFails(test *testing.T) {
	expectations := map[string]struct {
		failing    string
		dataPlanes string
		calls      int
	}{
		"the live spec query":       {"liveSpecs", dataPlanesResponse, 1},
		"the data plane query":      {"dataPlanes(", dataPlanesResponse, 2},
		"the storage mapping query": {"storageMappings", noDataPlanesResponse, 3},
	}

	for name, expected := range expectations {
		client, calls := graphqlClient(test, liveSpecsResponse, expected.dataPlanes, storageMappingsResponse)

		test.Setenv("FLOWCTL_FAILING", expected.failing)

		if dataPlane, err := client.ReadDataPlane(context.Background(), "acmeCo/shop/source-postgres"); err == nil || dataPlane != "" || !strings.Contains(err.Error(), "control plane unreachable") {
			test.Errorf("%s: data plane = %q, %v, want the flowctl failure", name, dataPlane, err)
		}

		if lines := recordedCalls(test, calls); len(lines) != expected.calls {
			test.Errorf("%s: calls = %q, want %d", name, lines, expected.calls)
		}
	}
}

func TestReadDataPlaneSkipsUnnamedDataPlanesAndCachesMappedOnes(test *testing.T) {
	client, calls := graphqlClient(test, liveSpecsResponse, strings.ReplaceAll(dataPlanesResponse, testDataPlane, ""), storageMappingsResponse)

	for range 2 {
		dataPlane, err := client.ReadDataPlane(context.Background(), "acmeCo/shop/source-postgres")

		if err != nil || dataPlane != testDataPlane {
			test.Fatalf("data plane = %q, %v, want %s from the storage mapping, as the data plane came back unnamed", dataPlane, err, testDataPlane)
		}
	}

	if lines := recordedCalls(test, calls); len(lines) != 4 {
		test.Errorf("calls = %q, want three for the first read and only the live spec for the second", lines)
	}
}
