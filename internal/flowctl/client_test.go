package flowctl

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecCommandMasksSecretsInErrors(test *testing.T) {

	var binary string = filepath.Join(test.TempDir(), "flowctl")
	var script string = "#!/bin/sh\necho \"invalid password hunter2 for token $FLOW_AUTH_TOKEN\" >&2\nexit 1\n"
	var client *Client = NewClient(binary, "<api-token>", "")

	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		test.Fatal(err)
	}

	_, err := client.execCommand(context.Background(), []string{"hunter2"}, "catalog", "publish")

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
	var binary string = filepath.Join(test.TempDir(), "flowctl")
	var script string = "#!/bin/sh\neval spec=\\${$#}\necho \"args: $*\" > \"$spec\"\n"
	var client *Client = NewClient(binary, "<api-token>", "")

	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		test.Fatal(err)
	}

	content, directory, err := client.runOnSpec(context.Background(), map[string][]byte{"dekaf.config.yaml": []byte("token: x")}, "--target", "catalog", "pull-specs")

	if err != nil {
		test.Fatal(err)
	}

	defer os.RemoveAll(directory)

	if want := "args: catalog pull-specs --target " + filepath.Join(directory, specFile) + "\n"; string(content) != want {
		test.Errorf("spec = %q, want %q", content, want)
	}

	if _, err := os.Stat(filepath.Join(directory, "dekaf.config.yaml")); err != nil {
		test.Errorf("workspace lost its files: %v", err)
	}
}

func TestRunOnSpecRemovesTheWorkspaceOnFailure(test *testing.T) {
	var binary string = filepath.Join(test.TempDir(), "flowctl")
	var client *Client = NewClient(binary, "<api-token>", "")

	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		test.Fatal(err)
	}

	if _, directory, err := client.runOnSpec(context.Background(), nil, "--target", "catalog", "pull-specs"); err == nil || directory != "" {
		test.Errorf("runOnSpec = %q, %v, want no workspace and the command failure", directory, err)
	}
}
