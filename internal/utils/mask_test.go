package utils_test

import (
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
)

func TestMaskYAML(test *testing.T) {
	var content []byte = spectest.Dedent([]byte(`
		materializations:
			acmeCo/snowflake/materialize:
				endpoint:
					connector:
						config:
							host: orgname-accountname.snowflakecomputing.com
							credentials:
								auth_type: jwt
								user: snowflake_user
								private_key: <private-key>
							dbt_job_trigger:
								api_key: <dbt-api-key>
							advanced:
								ssl_client_key: <client-key>
							networkTunnel:
								sshForwarding:
									privateKey: <ssh-key>
	`))

	var masked string = string(utils.MaskYAML(content))

	for _, secret := range []string{"<private-key>", "<dbt-api-key>", "<client-key>", "<ssh-key>"} {
		if strings.Contains(masked, secret) {
			test.Errorf("masked spec still holds %s:\n%s", secret, masked)
		}
	}

	for _, kept := range []string{"orgname-accountname.snowflakecomputing.com", "auth_type: jwt", "user: snowflake_user"} {
		if !strings.Contains(masked, kept) {
			test.Errorf("masked spec lost %s:\n%s", kept, masked)
		}
	}
}

func TestSecretValues(test *testing.T) {
	var content []byte = spectest.Dedent([]byte(`
		materializations:
			acmeCo/snowflake/materialize:
				endpoint:
					connector:
						config:
							host: orgname-accountname.snowflakecomputing.com
							credentials:
								user: snowflake_user
								private_key: <private-key>
								password: ""
							dbt_job_trigger:
								api_key: <dbt-api-key>
	`))

	var secrets []string = utils.SecretValues(content)

	sort.Strings(secrets)

	if want := []string{"<dbt-api-key>", "<private-key>"}; !slices.Equal(secrets, want) {
		test.Errorf("secrets = %v, want %v", secrets, want)
	}
}

func TestMaskStrings(test *testing.T) {
	var masked string = utils.MaskStrings("auth failed for hunter2 at host", []string{"hunter2", ""})

	if masked != "auth failed for *** at host" {
		test.Errorf("masked = %q", masked)
	}
}

func TestMaskYAMLHidesUnparseableContent(test *testing.T) {
	if masked := string(utils.MaskYAML([]byte("password: [unclosed"))); strings.Contains(masked, "unclosed") {
		test.Errorf("masked = %q, want unparseable content hidden", masked)
	}
}
