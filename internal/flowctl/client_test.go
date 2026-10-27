package flowctl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
)

func TestExecCommandMasksSecretsInErrors(test *testing.T) {
	var client *Client = fakeFlowctl(test, "#!/bin/sh\necho \"invalid password hunter2 for token $FLOW_AUTH_TOKEN\" >&2\nexit 1\n")

	_, err := client.execCommand(catalog.WithSecrets(context.Background(), "hunter2"), "catalog", "publish")

	if err == nil {
		test.Fatal("execCommand succeeded, want the script failure")
	}

	for _, secret := range []string{"hunter2", "<api-token>"} {
		if strings.Contains(err.Error(), secret) {
			test.Errorf("error still holds %s: %v", secret, err)
		}
	}

	if !strings.Contains(err.Error(), "invalid password *** for token ***") {
		test.Errorf("error lost the stderr message: %v", err)
	}
}

func TestRunOnSpecReturnsTheWrittenSpec(test *testing.T) {
	var client *Client = fakeFlowctl(test, "#!/bin/sh\neval spec=\\${$#}\necho \"args: $*\" > \"$spec\"\n")
	var workspace string

	err := inWorkspace(map[string][]byte{"dekaf.config.yaml": []byte("token: x")}, func(directory string) error {
		workspace = directory

		content, err := client.runOnSpec(context.Background(), directory, specFile, "--target", "catalog", "pull-specs")

		if err != nil {
			return err
		}

		if want := "args: catalog pull-specs --target " + filepath.Join(directory, specFile) + "\n"; string(content) != want {
			test.Errorf("spec = %q, want %q", content, want)
		}

		if _, err := os.Stat(filepath.Join(directory, "dekaf.config.yaml")); err != nil {
			test.Errorf("workspace lost its files: %v", err)
		}

		return nil
	})

	if err != nil {
		test.Fatal(err)
	}

	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		test.Errorf("workspace %s still exists: %v", workspace, err)
	}
}

func TestInWorkspaceRemovesTheWorkspaceOnFailure(test *testing.T) {
	var workspace string

	err := inWorkspace(nil, func(directory string) error {
		workspace = directory

		return errors.New("command failed")
	})

	if err == nil || err.Error() != "command failed" {
		test.Errorf("inWorkspace = %v, want the command failure", err)
	}

	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		test.Errorf("workspace %s still exists: %v", workspace, err)
	}
}
