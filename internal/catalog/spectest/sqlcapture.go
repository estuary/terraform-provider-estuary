package spectest

import (
	"slices"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/sqlcapture"
)

// AssertSQLCaptureBindings checks bindings match the ones every SQL capture full sample declares.
func AssertSQLCaptureBindings(test testing.TB, bindings []capture.Binding[sqlcapture.Resource]) {
	test.Helper()

	expectations := []struct {
		stream   string
		mode     sqlcapture.BackfillMode
		priority int
		backfill int
		disable  bool
	}{
		{"orders", sqlcapture.BackfillModeAutomatic, 0, 0, false},
		{"customers", sqlcapture.BackfillModePrecise, 10, 2, false},
		{"events", sqlcapture.BackfillModeOnlyChanges, -1, 0, false},
		{"logs", sqlcapture.BackfillModeWithoutKey, 0, 0, true},
		{"invoices", sqlcapture.BackfillModeNormal, 0, 0, false},
	}

	if len(bindings) != len(expectations) {
		test.Fatalf("got %d bindings, want %d", len(bindings), len(expectations))
	}

	for index, expected := range expectations {
		var binding capture.Binding[sqlcapture.Resource] = bindings[index]

		if binding.Resource.Stream != expected.stream || binding.Resource.Mode != expected.mode || binding.Resource.Priority != expected.priority {
			test.Errorf("binding %d resource = %+v, want stream %q mode %q priority %d", index, binding.Resource, expected.stream, expected.mode, expected.priority)
		}

		if binding.Backfill != expected.backfill || binding.Disable != expected.disable {
			test.Errorf("binding %d backfill = %d, disable = %v, want %d and %v", index, binding.Backfill, binding.Disable, expected.backfill, expected.disable)
		}
	}

	if bindings[2].Resource.Advanced == nil || bindings[2].Resource.Advanced.AdditionalBackfillFilter != "created_at > '2026-01-01'" {
		test.Errorf("events resource advanced = %+v, want the additional backfill filter", bindings[2].Resource.Advanced)
	}

	if !slices.Equal(bindings[3].Resource.PrimaryKey, []string{"id"}) {
		test.Errorf("logs primary_key = %v, want [id]", bindings[3].Resource.PrimaryKey)
	}
}
