package sourcezendesksupport

import "testing"

func TestConnectorRotatesSecrets(test *testing.T) {
	if !connector.RotatesSecrets {
		test.Error("the connector doesn't rotate secrets, want the OAuth tokens it replaces not to be drift")
	}
}
