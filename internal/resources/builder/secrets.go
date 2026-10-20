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

	"github.com/estuary/terraform-provider-estuary/internal/resources/builder/translator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const secretsHashAlgorithm string = "pbkdf2-sha256"
const secretsHashSaltSize int = 16

// secretsHashIterations follows the OWASP recommendation. https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html.
const secretsHashIterations int = 600000

func (codec *Codec[D, S]) writeOnly(path *tftypes.AttributePath) bool {
	attribute, err := codec.schema.AttributeAtTerraformPath(context.Background(), path)

	return err == nil && attribute.IsWriteOnly()
}

// secretAttribute reports whether the attribute at path is a write-only secret or its hash, which plans and live specs
// never hold.
func (codec *Codec[D, S]) secretAttribute(path *tftypes.AttributePath) bool {
	_, isHash := codec.secretPath(path)

	return isHash || codec.writeOnly(path)
}

// secretPath returns the path of the write-only attribute whose hash is at path, found being false when path isn't a hash.
func (codec *Codec[D, S]) secretPath(path *tftypes.AttributePath) (*tftypes.AttributePath, bool) {
	var steps []tftypes.AttributePathStep = path.Steps()

	if len(steps) == 0 {
		return nil, false
	}

	name, isName := steps[len(steps)-1].(tftypes.AttributeName)

	if !isName || !strings.HasSuffix(string(name), translator.HashSuffix) {
		return nil, false
	}

	var secret *tftypes.AttributePath = tftypes.NewAttributePathWithSteps(append(slices.Clone(steps[:len(steps)-1]), tftypes.AttributeName(strings.TrimSuffix(string(name), translator.HashSuffix))))

	return secret, codec.writeOnly(secret)
}

// attributePath converts a Terraform path into the framework path diagnostics point to.
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

// hashSecret returns the salted hash of a secret stored beside it, with a new random salt.
func hashSecret(secret string) (string, error) {
	var salt []byte = make([]byte, secretsHashSaltSize)

	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating secrets hash salt: %w", err)
	}

	return secretHash(secret, salt, secretsHashIterations)
}

// verifySecret reports whether the secret is the one the stored hash was computed from.
func verifySecret(secret string, stored string) bool {
	var parts []string = strings.Split(stored, "$")

	if len(parts) != 4 || parts[0] != secretsHashAlgorithm {
		return false
	}

	iterations, err := strconv.Atoi(parts[1])

	if err != nil || iterations <= 0 {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[2])

	if err != nil {
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

// NullSecrets returns value with its write-only attributes null, as Terraform never stores them.
func (codec *Codec[D, S]) NullSecrets(value tftypes.Value) (tftypes.Value, error) {
	return nullAttributes(value, codec.writeOnly)
}

// nullSecretsHash returns value with its hashes null, the plan ones being unknown until apply.
func (codec *Codec[D, S]) nullSecretsHash(value tftypes.Value) (tftypes.Value, error) {
	return nullAttributes(value, func(path *tftypes.AttributePath) bool {
		_, isHash := codec.secretPath(path)

		return isHash
	})
}

// nullAttributes returns value with the attributes matching null.
func nullAttributes(value tftypes.Value, matching func(path *tftypes.AttributePath) bool) (tftypes.Value, error) {
	return tftypes.Transform(value, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		if current.IsNull() || !matching(path) {
			return current, nil
		}

		return tftypes.NewValue(current.Type(), nil), nil
	})
}

// WithConfigSecrets returns value with its write-only attributes taken from config, the only value holding them.
func (codec *Codec[D, S]) WithConfigSecrets(value tftypes.Value, config tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(value, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		if !codec.writeOnly(path) {
			return current, nil
		}

		secret, err := attributeValue(config, path)

		if err != nil {
			return current, nil
		}

		return secret, nil
	})
}

// PlanSecretsHash returns plan with the hash beside each write-only attribute: the state one while the secret in config
// still matches it, unknown until apply when the secret is set and changed, and null when it is unset.
// Changed holds the paths of the secrets whose state hash no longer matches.
func (codec *Codec[D, S]) PlanSecretsHash(plan tftypes.Value, config tftypes.Value, state tftypes.Value) (tftypes.Value, []*tftypes.AttributePath, error) {
	var changed []*tftypes.AttributePath

	planned, err := tftypes.Transform(plan, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		secretPath, isHash := codec.secretPath(path)

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

// StateSecretsHash returns plan with each hash it doesn't know yet computed from the secret beside it in planned, the
// plan holding the config secrets, with a new salt.
func (codec *Codec[D, S]) StateSecretsHash(plan tftypes.Value, planned tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(plan, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		secretPath, isHash := codec.secretPath(path)

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

// SecretsChanged reports whether plan changes the secrets of prior, a hash being unknown until apply or differing.
func (codec *Codec[D, S]) SecretsChanged(plan tftypes.Value, prior tftypes.Value) bool {
	var planned map[string]tftypes.Value = codec.secretsHashes(plan)
	var stored map[string]tftypes.Value = codec.secretsHashes(prior)

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

// secretsHashes returns the hashes set in value by their path.
func (codec *Codec[D, S]) secretsHashes(value tftypes.Value) map[string]tftypes.Value {
	var hashes map[string]tftypes.Value = map[string]tftypes.Value{}

	tftypes.Walk(value, func(path *tftypes.AttributePath, current tftypes.Value) (bool, error) {
		if _, isHash := codec.secretPath(path); isHash && !current.IsNull() {
			hashes[path.String()] = current
		}

		return true, nil
	})

	return hashes
}

// attributeValue returns the value at path in value, an error when value or one of the path parents is null.
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
