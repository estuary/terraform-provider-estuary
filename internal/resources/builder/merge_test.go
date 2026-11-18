package builder

import (
	"reflect"
	"testing"
)

func TestKeepLiveFlags(test *testing.T) {
	var live map[string]any = map[string]any{"shards": map[string]any{"flags": map[string]any{"enable-runtime-v2": "true", "debug": "true"}}}

	expectations := map[string]struct {
		document map[string]any
		live     map[string]any
		want     map[string]any
	}{
		"keeps the live flags of unset shards": {
			document: map[string]any{},
			live:     live,
			want:     map[string]any{"shards": map[string]any{"flags": map[string]any{"enable-runtime-v2": "true", "debug": "true"}}},
		},
		"keeps the live flags shards leave out": {
			document: map[string]any{"shards": map[string]any{"disable": true}},
			live:     live,
			want:     map[string]any{"shards": map[string]any{"disable": true, "flags": map[string]any{"enable-runtime-v2": "true", "debug": "true"}}},
		},
		"keeps the planned value of a flag": {
			document: map[string]any{"shards": map[string]any{"flags": map[string]any{"debug": "false"}}},
			live:     live,
			want:     map[string]any{"shards": map[string]any{"flags": map[string]any{"enable-runtime-v2": "true", "debug": "false"}}},
		},
		"keeps the plan without live flags": {
			document: map[string]any{"shards": map[string]any{"disable": true}},
			live:     map[string]any{"shards": map[string]any{"disable": false}},
			want:     map[string]any{"shards": map[string]any{"disable": true}},
		},
	}

	for name, expected := range expectations {
		keepLiveFlags(expected.document, expected.live)

		if !reflect.DeepEqual(expected.document, expected.want) {
			test.Errorf("%s: document = %v, want %v", name, expected.document, expected.want)
		}
	}
}

func TestWithoutFlags(test *testing.T) {
	var spec map[string]any = map[string]any{"shards": map[string]any{"disable": true, "flags": map[string]any{"enable-runtime-v2": "true"}}}
	var flagsOnly map[string]any = map[string]any{"bindings": []any{}, "shards": map[string]any{"flags": map[string]any{"enable-runtime-v2": "true"}}}

	if got := withoutFlags(spec); !reflect.DeepEqual(got, map[string]any{"shards": map[string]any{"disable": true}}) {
		test.Errorf("withoutFlags = %v, want the shards without flags", got)
	}

	if _, kept := spec["shards"].(map[string]any)["flags"]; !kept {
		test.Errorf("spec = %v, want it unchanged", spec)
	}

	if got := withoutFlags(flagsOnly); !reflect.DeepEqual(got, map[string]any{"bindings": []any{}}) {
		test.Errorf("withoutFlags = %v, want no shards left", got)
	}
}
