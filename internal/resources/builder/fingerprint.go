package builder

import (
	"context"
	"encoding/json"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// secretsFingerprintKey is the private state key of the fingerprint of the entry's encrypted secrets as Terraform
// last published or imported them, see catalog.SecretsFingerprint.
const secretsFingerprintKey string = "secrets_fingerprint"

// importedKey is the private state key marking an entry imported and not applied since, whose backfill triggers record
// their starting value, as on a create.
const importedKey string = "imported"

// importedMark is the importedKey value of an imported entry.
var importedMark []byte = []byte("true")

// imported reports whether data marks the entry imported and not applied since.
func imported(ctx context.Context, data privateData) bool {
	if data == nil {
		return false
	}

	content, diagnostics := data.GetKey(ctx, importedKey)

	return !diagnostics.HasError() && string(content) == string(importedMark)
}

// privateData is the provider-private state of a resource, which Terraform keeps beside state without showing it.
type privateData interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

// optionalPrivate returns data as a privateData, nil when it is a nil pointer, as outside Terraform.
func optionalPrivate[P interface {
	comparable
	privateData
}](data P) privateData {
	var none P

	if data == none {
		return nil
	}

	return data
}

// storedFingerprint returns the stored secrets fingerprint, empty when none was stored.
func storedFingerprint(ctx context.Context, data privateData) (string, diag.Diagnostics) {
	var fingerprint string

	if data == nil {
		return "", nil
	}

	content, diagnostics := data.GetKey(ctx, secretsFingerprintKey)

	if diagnostics.HasError() || len(content) == 0 {
		return "", diagnostics
	}

	if err := json.Unmarshal(content, &fingerprint); err != nil {
		diagnostics.AddError("Reading the stored secrets fingerprint", err.Error())
	}

	return fingerprint, diagnostics
}

// storeFingerprint stores the secrets fingerprint, empty leaving the next refresh to take the live one.
func storeFingerprint(ctx context.Context, data privateData, fingerprint string) diag.Diagnostics {
	if data == nil {
		return nil
	}

	content, err := json.Marshal(fingerprint)

	if err != nil {
		var diagnostics diag.Diagnostics

		diagnostics.AddError("Storing the secrets fingerprint", err.Error())

		return diagnostics
	}

	return data.SetKey(ctx, secretsFingerprintKey, content)
}

// refreshedFingerprint compares the live secrets with the stored fingerprint. It returns the fingerprint to keep, the
// live one unless secrets changed outside Terraform, in which case refreshed has its secret hashes nulled, so the plan
// publishes the configured secrets again, and a warning explains it. Entries whose state holds no secret, which the
// apply would keep live, and connectors rotating their own secrets never drift.
func (entryResource *entryResource[D, S]) refreshedFingerprint(catalogName string, stored string, sealed map[string]any, refreshed tftypes.Value) (string, tftypes.Value, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	var live string = catalog.SecretsFingerprint(sealed)

	if stored == "" || stored == live || entryResource.connector.RotatesSecrets || len(entryResource.codec.secrets.hashes(refreshed)) == 0 {
		return live, refreshed, diagnostics
	}

	refreshed, err := entryResource.codec.secrets.nullHashes(refreshed)

	if err != nil {
		diagnostics.AddError("Refreshing the secrets of "+entryResource.catalogType.name+" "+catalogName, err.Error())
		return stored, refreshed, diagnostics
	}

	diagnostics.AddWarning("Secrets changed outside Terraform in "+catalogName,
		"The encrypted secrets of "+catalogName+" changed since Terraform last published them, such as a password entered "+
			"again in the dashboard. Terraform can't read secrets back, so the apply publishes the configured ones again. To "+
			"keep the new ones, set them in the configuration before applying.")

	return stored, refreshed, diagnostics
}

// publishedFingerprint returns the secrets fingerprint of the entry once published as document: that of the sealed
// secrets it kept, else, when it sent secrets for Estuary to encrypt, that of the entry read again. An unreadable
// entry returns empty with a warning, leaving the next refresh to take the live one.
func (entryResource *entryResource[D, S]) publishedFingerprint(ctx context.Context, catalogName string, document map[string]any, planned tftypes.Value) (string, diag.Diagnostics) {
	var diagnostics diag.Diagnostics

	if kept := catalog.SecretsFingerprint(document); kept != "" || len(entryResource.codec.secrets.values(planned)) == 0 {
		return kept, diagnostics
	}

	sealed, err := entryResource.readSealed(ctx, catalogName)

	if err != nil {
		diagnostics.AddWarning("Couldn't read the published secrets of "+entryResource.catalogType.name+" "+catalogName,
			err.Error()+"\n\nThe next refresh treats the live secrets as the ones Terraform published.")

		return "", diagnostics
	}

	return catalog.SecretsFingerprint(sealed), diagnostics
}
