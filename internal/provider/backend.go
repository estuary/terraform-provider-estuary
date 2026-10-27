package provider

import (
	"fmt"
	"os/exec"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/flowctl"
)

// defaultBackend is the flowctl CLI backend, used unless configured otherwise.
const defaultBackend string = "flowctl"

type backendOptions struct {
	flowctlPath string
	token       string
	dataPlane   string
}

// newBackend returns the catalog backend registered under name.
func newBackend(backendName string, options backendOptions) (catalog.Backend, error) {
	switch backendName {
	case defaultBackend:
		flowctlResolvedPath, err := exec.LookPath(options.flowctlPath)

		if err != nil {
			return nil, fmt.Errorf("flowctl binary %q not found: %w", options.flowctlPath, err)
		}

		return flowctl.NewClient(flowctlResolvedPath, options.token, options.dataPlane), nil
	default:
		return nil, fmt.Errorf("catalog backend %q is not implemented yet. Available backends: [%s]", backendName, defaultBackend)
	}
}
