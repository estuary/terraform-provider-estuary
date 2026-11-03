package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

const sopsKey string = "sops"

const encryptedSuffix string = "_sops"

const encryptedPrefix string = "ENC["

// WithoutSecrets returns a copy of the spec without sops metadata and encrypted values, which can't be decrypted.
func WithoutSecrets(spec map[string]any) map[string]any {
	stripped, _ := withoutSecrets(spec).(map[string]any)

	return stripped
}

func withoutSecrets(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		var stripped map[string]any = make(map[string]any, len(typed))

		for key, nested := range typed {
			if key == sopsKey || encryptedField(key, nested) {
				continue
			}

			stripped[key] = withoutSecrets(nested)
		}

		return stripped
	case []any:
		var stripped []any = make([]any, len(typed))

		for index, item := range typed {
			stripped[index] = withoutSecrets(item)
		}

		return stripped
	}

	return value
}

// encryptedField reports whether a field holds a value sops encrypted: named with the encrypted suffix, or wrapped.
func encryptedField(key string, value any) bool {
	text, isString := value.(string)

	return strings.HasSuffix(key, encryptedSuffix) || isString && strings.HasPrefix(text, encryptedPrefix)
}

// SecretsFingerprint returns a digest of the encrypted values of a sealed spec's endpoint, empty when it holds none.
// Estuary encrypts each secret with a new nonce, so the digest changes whenever a secret is set again, even to the
// same value, and never otherwise: sops metadata is left out.
func SecretsFingerprint(spec map[string]any) string {
	var entries []string

	encryptedEntries(spec[EndpointKey], "/"+EndpointKey, &entries)

	if len(entries) == 0 {
		return ""
	}

	slices.Sort(entries)

	var digest [sha256.Size]byte = sha256.Sum256([]byte(strings.Join(entries, "\n")))

	return "sha256:" + hex.EncodeToString(digest[:])
}

func encryptedEntries(value any, path string, entries *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if key == sopsKey {
				continue
			}

			if encryptedField(key, nested) {
				*entries = append(*entries, fmt.Sprintf("%s/%s=%v", path, key, nested))
				continue
			}

			encryptedEntries(nested, path+"/"+key, entries)
		}
	case []any:
		for index, item := range typed {
			encryptedEntries(item, fmt.Sprintf("%s/%d", path, index), entries)
		}
	}
}
