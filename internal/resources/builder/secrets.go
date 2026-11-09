package builder

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder/translator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const secretsHashAlgorithm string = "pbkdf2-sha256"
const secretsHashSaltSize int = 16

// secretsHashIterations follows OWASP: https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html.
const secretsHashIterations int = 600000

// verifiedSecrets holds, for this provider process only, each stored hash with the SHA-256 of a secret verified to match.
var verifiedSecrets sync.Map

// secrets handles a schema's write-only attributes and the salted hash beside each.
type secrets struct {
	schema schema.Schema
}

func (secrets secrets) writeOnly(path *tftypes.AttributePath) bool {
	attribute, err := secrets.schema.AttributeAtTerraformPath(context.Background(), path)

	return err == nil && attribute.IsWriteOnly()
}

// attribute reports whether path is a write-only secret or its hash, which plans and live specs never hold.
func (secrets secrets) attribute(path *tftypes.AttributePath) bool {
	_, isHash := secrets.secretPath(path)

	return isHash || secrets.writeOnly(path)
}

// secretPath returns the write-only attribute path of the hash at path, false when path isn't a hash.
func (secrets secrets) secretPath(path *tftypes.AttributePath) (*tftypes.AttributePath, bool) {
	var steps []tftypes.AttributePathStep = path.Steps()

	if len(steps) == 0 {
		return nil, false
	}

	name, isName := steps[len(steps)-1].(tftypes.AttributeName)

	if !isName || !strings.HasSuffix(string(name), translator.HashSuffix) {
		return nil, false
	}

	var secret *tftypes.AttributePath = tftypes.NewAttributePathWithSteps(append(slices.Clone(steps[:len(steps)-1]), tftypes.AttributeName(strings.TrimSuffix(string(name), translator.HashSuffix))))

	return secret, secrets.writeOnly(secret)
}

// attributePath converts a Terraform path into a framework path for diagnostics.
func attributePath(attributePath *tftypes.AttributePath) path.Path {
	var converted path.Path

	for index, step := range attributePath.Steps() {
		switch typed := step.(type) {
		case tftypes.AttributeName:
			if index == 0 {
				converted = path.Root(string(typed))
			} else {
				converted = converted.AtName(string(typed))
			}
		case tftypes.ElementKeyString:
			converted = converted.AtMapKey(string(typed))
		case tftypes.ElementKeyInt:
			converted = converted.AtListIndex(int(typed))
		}
	}

	return converted
}

// hashSecret returns the salted hash of a secret, with a new random salt.
func hashSecret(secret string) (string, error) {
	var salt []byte = make([]byte, secretsHashSaltSize)

	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating secrets hash salt: %w", err)
	}

	return secretHash(secret, salt, secretsHashIterations)
}

// verifySecret reports whether secret matches the stored hash, deriving it once per provider process.
func verifySecret(secret string, stored string) bool {
	var digest [sha256.Size]byte = sha256.Sum256([]byte(secret))
	var key string = stored + "$" + base64.RawStdEncoding.EncodeToString(digest[:])

	if _, verified := verifiedSecrets.Load(key); verified {
		return true
	}

	if !deriveMatches(secret, stored) {
		return false
	}

	verifiedSecrets.Store(key, true)

	return true
}

// deriveMatches reports whether deriving secret with the parameters of the stored hash gives that hash. State is
// untrusted input, so a hash with parameters hashSecret never writes, such as more iterations, doesn't match rather
// than costing every plan its iterations; the plan then shows the secret as changed and the apply hashes it again.
func deriveMatches(secret string, stored string) bool {
	var parts []string = strings.Split(stored, "$")

	if len(parts) != 4 || parts[0] != secretsHashAlgorithm {
		return false
	}

	iterations, err := strconv.Atoi(parts[1])

	if err != nil || iterations <= 0 || iterations > secretsHashIterations {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[2])

	if err != nil || len(salt) != secretsHashSaltSize {
		return false
	}

	hash, err := secretHash(secret, salt, iterations)

	return err == nil && subtle.ConstantTimeCompare([]byte(hash), []byte(stored)) == 1
}

func secretHash(secret string, salt []byte, iterations int) (string, error) {
	key, err := pbkdf2.Key(sha256.New, secret, salt, iterations, sha256.Size)

	if err != nil {
		return "", fmt.Errorf("hashing secrets: %w", err)
	}

	return fmt.Sprintf("%s$%d$%s$%s", secretsHashAlgorithm, iterations, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// nullSecrets nulls the write-only attributes, as Terraform never stores them.
func (secrets secrets) nullSecrets(value tftypes.Value) (tftypes.Value, error) {
	return nullAttributes(value, secrets.writeOnly)
}

// withoutLiveSecrets nulls the write-only attributes of a live value and redacts the credentials of attributes
// forbidding them, such as a MongoDB address with user info, returning the redacted paths.
func (secrets secrets) withoutLiveSecrets(value tftypes.Value) (tftypes.Value, []*tftypes.AttributePath, error) {
	var redactedPaths []*tftypes.AttributePath

	value, err := secrets.nullSecrets(value)

	if err != nil {
		return value, nil, err
	}

	value, err = tftypes.Transform(value, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		text, credentials, restricted := secrets.restricted(path, current)

		if !restricted {
			return current, nil
		}

		if safe := translator.RedactCredentials(text, credentials); safe != text {
			redactedPaths = append(redactedPaths, path)

			return tftypes.NewValue(current.Type(), safe), nil
		}

		return current, nil
	})

	return value, redactedPaths, err
}

// credentialPaths returns the paths of the attributes holding the credentials their field forbids. Plan validators skip
// values unknown at plan time, such as an address built from another resource, so the apply checks them again.
func (secrets secrets) credentialPaths(value tftypes.Value) []string {
	var paths []string

	tftypes.Walk(value, func(valuePath *tftypes.AttributePath, current tftypes.Value) (bool, error) {
		if text, credentials, restricted := secrets.restricted(valuePath, current); restricted && translator.HoldsCredentials(text, credentials) {
			paths = append(paths, attributePath(valuePath).String())
		}

		return true, nil
	})

	return paths
}

// restricted returns the text of a known string attribute at path whose field forbids credentials, and its rule,
// reporting false for other attributes.
func (secrets secrets) restricted(path *tftypes.AttributePath, value tftypes.Value) (string, catalog.Credentials, bool) {
	var text string

	if !value.Type().Is(tftypes.String) || !value.IsKnown() || value.IsNull() || value.As(&text) != nil {
		return "", catalog.CredentialsAllowed, false
	}

	attribute, err := secrets.schema.AttributeAtTerraformPath(context.Background(), path)

	if err != nil {
		return "", catalog.CredentialsAllowed, false
	}

	var credentials catalog.Credentials = translator.AttributeCredentials(attribute)

	return text, credentials, credentials != catalog.CredentialsAllowed
}

// nullHashes nulls the hashes, plan ones being unknown until apply.
func (secrets secrets) nullHashes(value tftypes.Value) (tftypes.Value, error) {
	return nullAttributes(value, func(path *tftypes.AttributePath) bool {
		_, isHash := secrets.secretPath(path)

		return isHash
	})
}

// values returns the config secrets in value, for backends to mask.
func (secrets secrets) values(value tftypes.Value) []string {
	var values []string

	tftypes.Walk(value, func(path *tftypes.AttributePath, current tftypes.Value) (bool, error) {
		var text string

		if secrets.writeOnly(path) && current.IsKnown() && !current.IsNull() && current.As(&text) == nil && text != "" {
			values = append(values, text)
		}

		return true, nil
	})

	return values
}

// nullAttributes nulls the attributes whose path matching reports.
func nullAttributes(value tftypes.Value, matching func(path *tftypes.AttributePath) bool) (tftypes.Value, error) {
	return tftypes.Transform(value, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		if current.IsNull() || !matching(path) {
			return current, nil
		}

		return tftypes.NewValue(current.Type(), nil), nil
	})
}

// withConfigSecrets fills the write-only attributes from config, the only value holding them.
func (secrets secrets) withConfigSecrets(value tftypes.Value, config tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(value, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		if !secrets.writeOnly(path) {
			return current, nil
		}

		secret, err := attributeValue(config, path)

		if err != nil {
			return current, nil
		}

		return secret, nil
	})
}

// planHashes plans each hash: the state one while the secret matches, unknown when changed, null when unset.
// Changed holds the paths of the changed secrets.
func (secrets secrets) planHashes(plan tftypes.Value, config tftypes.Value, state tftypes.Value) (tftypes.Value, []*tftypes.AttributePath, error) {
	var changed []*tftypes.AttributePath

	planned, err := tftypes.Transform(plan, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		secretPath, isHash := secrets.secretPath(path)

		if !isHash {
			return current, nil
		}

		secret, err := attributeValue(config, secretPath)

		if err != nil || secret.IsNull() {
			return tftypes.NewValue(current.Type(), nil), nil
		}

		stored, err := attributeValue(state, path)

		if err != nil || stored.IsNull() || !secret.IsKnown() {
			return tftypes.NewValue(current.Type(), tftypes.UnknownValue), nil
		}

		var secretText, storedText string

		if secret.As(&secretText) != nil || stored.As(&storedText) != nil || !verifySecret(secretText, storedText) {
			changed = append(changed, secretPath)

			return tftypes.NewValue(current.Type(), tftypes.UnknownValue), nil
		}

		return stored, nil
	})

	return planned, changed, err
}

// removed returns the paths of the secrets state holds a hash for and config leaves out, which the apply deletes.
func (secrets secrets) removed(config tftypes.Value, state tftypes.Value) []*tftypes.AttributePath {
	var removed []*tftypes.AttributePath

	if state.Type() == nil || state.IsNull() {
		return nil
	}

	tftypes.Walk(state, func(path *tftypes.AttributePath, current tftypes.Value) (bool, error) {
		secretPath, isHash := secrets.secretPath(path)

		if !isHash || current.IsNull() {
			return true, nil
		}

		if secret, err := attributeValue(config, secretPath); err != nil || secret.IsNull() {
			removed = append(removed, secretPath)
		}

		return true, nil
	})

	return removed
}

// unplanned returns the paths of the secrets unknown at plan that config resolves empty, or null while prior holds their
// hash, as the plan couldn't warn about deleting them.
func (secrets secrets) unplanned(plan tftypes.Value, config tftypes.Value, prior tftypes.Value) []*tftypes.AttributePath {
	var unplanned []*tftypes.AttributePath

	tftypes.Walk(plan, func(path *tftypes.AttributePath, current tftypes.Value) (bool, error) {
		secretPath, isHash := secrets.secretPath(path)

		if !isHash || current.IsKnown() {
			return true, nil
		}

		secret, err := attributeValue(config, secretPath)

		if err != nil || secret.IsNull() {
			if stored, err := attributeValue(prior, path); err == nil && !stored.IsNull() {
				unplanned = append(unplanned, secretPath)
			}

			return true, nil
		}

		var text string

		if secret.IsKnown() && secret.As(&text) == nil && text == "" {
			unplanned = append(unplanned, secretPath)
		}

		return true, nil
	})

	return unplanned
}

// stateHashes computes each unknown hash from the planned secret beside it, with a new salt.
func (secrets secrets) stateHashes(plan tftypes.Value, planned tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(plan, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		secretPath, isHash := secrets.secretPath(path)

		if !isHash || (current.IsKnown() && !current.IsNull()) {
			return current, nil
		}

		secret, err := attributeValue(planned, secretPath)

		if err != nil || secret.IsNull() {
			return tftypes.NewValue(current.Type(), nil), nil
		}

		var secretText string

		if err := secret.As(&secretText); err != nil {
			return current, err
		}

		hash, err := hashSecret(secretText)

		if err != nil {
			return current, err
		}

		return tftypes.NewValue(current.Type(), hash), nil
	})
}

// changed reports whether plan changes the secrets of prior.
func (secrets secrets) changed(plan tftypes.Value, prior tftypes.Value) bool {
	var planned map[string]tftypes.Value = secrets.hashes(plan)
	var stored map[string]tftypes.Value = secrets.hashes(prior)

	if len(planned) != len(stored) {
		return true
	}

	for key, hash := range planned {
		if !hash.IsKnown() || !hash.Equal(stored[key]) {
			return true
		}
	}

	return false
}

// hashes returns the hashes set in value, by path.
func (secrets secrets) hashes(value tftypes.Value) map[string]tftypes.Value {
	var hashes map[string]tftypes.Value = map[string]tftypes.Value{}

	tftypes.Walk(value, func(path *tftypes.AttributePath, current tftypes.Value) (bool, error) {
		if _, isHash := secrets.secretPath(path); isHash && !current.IsNull() {
			hashes[path.String()] = current
		}

		return true, nil
	})

	return hashes
}

// attributeValue returns the value at path, an error when value or a parent is null.
func attributeValue(value tftypes.Value, path *tftypes.AttributePath) (tftypes.Value, error) {
	if value.Type() == nil || value.IsNull() {
		return tftypes.Value{}, fmt.Errorf("no value at %s", path)
	}

	found, _, err := tftypes.WalkAttributePath(value, path)

	if err != nil {
		return tftypes.Value{}, err
	}

	attribute, isValue := found.(tftypes.Value)

	if !isValue {
		return tftypes.Value{}, fmt.Errorf("no value at %s", path)
	}

	return attribute, nil
}
