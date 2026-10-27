package mongodb

import (
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunCaptureSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[Config, Resource](test)
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, CaptureModes)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		address              string
		database             string
		batchAndChangeStream bool
		sshTunnel            bool
	}{
		"acmeCo/mongodb/change-stream": {"mongo:27017", "", false, false},
		"acmeCo/mongodb/atlas":         {"mongodb+srv://cluster0.123456789.mongodb.net?authSource=admin", "shop,billing", false, false},
		"acmeCo/mongodb/ssh-tunnel":    {"mongodb://123.4.567.89:27017", "", false, true},
		"acmeCo/mongodb/batch":         {"mongo:27017", "", true, false},
	}

	if len(specs) != len(expectations) {
		test.Fatalf("got %d captures, want %d", len(specs), len(expectations))
	}

	for name, expected := range expectations {
		test.Run(name, func(test *testing.T) {
			spec, found := specs[name]

			if !found {
				test.Fatal("capture missing from sample")
			}

			var config Config = spec.Endpoint.Connector.Config

			if config.Address != expected.address || config.Database != expected.database {
				test.Errorf("address = %q, database = %q, want %q and %q", config.Address, config.Database, expected.address, expected.database)
			}

			if config.User != "flow_capture" || config.Password != "<secret>" {
				test.Errorf("user = %q, password = %q, want flow_capture and <secret>", config.User, config.Password)
			}

			if config.BatchAndChangeStream != expected.batchAndChangeStream {
				test.Errorf("batchAndChangeStream = %v, want %v", config.BatchAndChangeStream, expected.batchAndChangeStream)
			}

			if (config.NetworkTunnel != nil) != expected.sshTunnel {
				test.Errorf("networkTunnel = %+v, want present = %v", config.NetworkTunnel, expected.sshTunnel)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/mongodb/all-options"]

	var config Config = spec.Endpoint.Connector.Config

	expectedAdvanced := Advanced{
		MaxAwaitTime:       "10s",
		DisablePreImages:   true,
		ExcludeCollections: "shop:sessions,shop:carts",
		SkipBackfills:      "shop:events",
	}

	if spec.AutoDiscover == nil || !spec.AutoDiscover.AddNewBindings || !spec.AutoDiscover.EvolveIncompatibleCollections {
		test.Errorf("autoDiscover = %+v, want both options enabled", spec.AutoDiscover)
	}

	if config.Database != "shop,metrics" || !config.BatchAndChangeStream || config.PollSchedule != "6h" {
		test.Errorf("database = %q, batchAndChangeStream = %v, pollSchedule = %q", config.Database, config.BatchAndChangeStream, config.PollSchedule)
	}

	if config.Advanced == nil || *config.Advanced != expectedAdvanced {
		test.Errorf("advanced = %+v, want %+v", config.Advanced, expectedAdvanced)
	}

	if config.NetworkTunnel == nil || config.NetworkTunnel.SSHForwarding == nil || !strings.HasPrefix(config.NetworkTunnel.SSHForwarding.PrivateKey, "-----BEGIN RSA PRIVATE KEY-----\n") {
		test.Errorf("networkTunnel = %+v, want an SSH forwarding with a multiline private key", config.NetworkTunnel)
	}
}

func TestSampleBindings(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var bindings []Binding = specs["acmeCo/mongodb/all-options"].Bindings

	expectations := []struct {
		collection   string
		captureMode  CaptureMode
		cursorField  string
		pollSchedule string
		backfill     int
		disable      bool
	}{
		{"users", "", "", "", 0, false},
		{"events", CaptureModeChangeStream, "", "", 2, false},
		{"user_view", CaptureModeBatchSnapshot, "_id", "1h", 0, false},
		{"readings", CaptureModeBatchIncremental, "ts", "daily at 02:00Z", 0, true},
	}

	if len(bindings) != len(expectations) {
		test.Fatalf("got %d bindings, want %d", len(bindings), len(expectations))
	}

	for index, expected := range expectations {
		var binding Binding = bindings[index]

		if binding.Resource.Collection != expected.collection || binding.Resource.CaptureMode != expected.captureMode {
			test.Errorf("binding %d resource = %+v, want collection %q mode %q", index, binding.Resource, expected.collection, expected.captureMode)
		}

		if binding.Resource.CursorField != expected.cursorField || binding.Resource.PollSchedule != expected.pollSchedule {
			test.Errorf("binding %d cursorField = %q, pollSchedule = %q, want %q and %q", index, binding.Resource.CursorField, binding.Resource.PollSchedule, expected.cursorField, expected.pollSchedule)
		}

		if binding.Backfill != expected.backfill || binding.Disable != expected.disable {
			test.Errorf("binding %d backfill = %d, disable = %v, want %d and %v", index, binding.Backfill, binding.Disable, expected.backfill, expected.disable)
		}
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var seenCaptureModes map[CaptureMode]bool = map[CaptureMode]bool{}

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var advanced *Advanced = spec.Endpoint.Connector.Config.Advanced

			if advanced != nil && advanced.ExclusiveCollectionFilter && advanced.ExcludeCollections != "" {
				test.Errorf("%s: sets both exclusiveCollectionFilter and excludeCollections, which the connector rejects", name)
			}

			for _, binding := range spec.Bindings {
				var mode CaptureMode = binding.Resource.CaptureMode

				if mode != "" && !slices.Contains(CaptureModes, mode) {
					test.Errorf("%s: captureMode %q is not in CaptureModes", name, mode)
				}

				if (mode == CaptureModeBatchSnapshot || mode == CaptureModeBatchIncremental) && binding.Resource.CursorField == "" {
					test.Errorf("%s: %s binding %s has no cursorField, which batch modes require", name, mode, binding.Resource.Collection)
				}

				seenCaptureModes[mode] = true
			}
		}
	}

	for _, mode := range CaptureModes {
		if !seenCaptureModes[mode] {
			test.Errorf("captureMode %q has no sample", mode)
		}
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]struct {
		key     string
		content string
	}{
		"credentials is a SQL capture field": {
			key: "credentials",
			content: `
				captures:
					acmeCo/mongodb/credentials:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-mongodb:v3
								config:
									address: mongo:27017
									user: flow_capture
									credentials:
										auth_type: UserPassword
										password: <secret>
						bindings: []
			`,
		},
		"stream is a SQL capture resource field": {
			key: "stream",
			content: `
				captures:
					acmeCo/mongodb/stream:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-mongodb:v3
								config:
									address: mongo:27017
									user: flow_capture
									password: <secret>
						bindings:
							- resource:
									database: shop
									stream: users
								target: acmeCo/mongodb/users
			`,
		},
	}

	for name, document := range documents {
		test.Run(name, func(test *testing.T) {
			_, err := spectest.ParseCaptures[Spec]([]byte(document.content))

			if err == nil || !strings.Contains(err.Error(), "field "+document.key+" not found") {
				test.Fatalf("error = %v, want strict decoding to reject the %s key", err, document.key)
			}
		})
	}
}

func TestResourcePath(test *testing.T) {
	var resource Resource = Resource{Database: "sample_mflix", Collection: "users", CaptureMode: "Batch Snapshot"}

	if path := resource.ResourcePath(); !slices.Equal(path, []string{"sample_mflix", "users"}) {
		test.Errorf("resource path = %q, want [sample_mflix users]", path)
	}
}
