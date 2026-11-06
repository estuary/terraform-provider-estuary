package builder

import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// liveBindingsDescription describes the live_bindings attribute of data sources.
const liveBindingsDescription string = "Every live binding, the declared ones and those Estuary adds, with the attributes of bindings."

// withLiveBindings sets the live bindings of value to the bindings of live, empty when it has none. Values without
// live bindings are returned unchanged.
func withLiveBindings(value tftypes.Value, live tftypes.Value) (tftypes.Value, error) {
	items, _, err := bindings(live)

	if err != nil {
		return value, err
	}

	if items == nil {
		items = []tftypes.Value{}
	}

	return withAttribute(value, liveBindingsPath, func(current tftypes.Value) tftypes.Value {
		return tftypes.NewValue(current.Type(), items)
	})
}
