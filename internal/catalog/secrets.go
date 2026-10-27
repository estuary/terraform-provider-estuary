package catalog

import "strings"

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
			if key == sopsKey || strings.HasSuffix(key, encryptedSuffix) || isEncrypted(nested) {
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

func isEncrypted(value any) bool {
	text, isString := value.(string)

	return isString && strings.HasPrefix(text, encryptedPrefix)
}
