package spectest

import (
	"strings"
	"testing"
)

// UnknownKey is a document whose Key strict decoding must reject.
type UnknownKey struct {
	Key     string
	Content string
}

// AssertUnknownCaptureKeys runs a subtest per document, checking strict capture decoding rejects its key.
func AssertUnknownCaptureKeys[S any](test *testing.T, documents map[string]UnknownKey) {
	test.Helper()

	assertUnknownKeys(test, documents, ParseCaptures[S])
}

// AssertUnknownMaterializationKeys runs a subtest per document, checking strict materialization decoding rejects its key.
func AssertUnknownMaterializationKeys[S any](test *testing.T, documents map[string]UnknownKey) {
	test.Helper()

	assertUnknownKeys(test, documents, ParseMaterializations[S])
}

func assertUnknownKeys[S any](test *testing.T, documents map[string]UnknownKey, parse func([]byte) (map[string]S, error)) {
	test.Helper()

	for name, document := range documents {
		test.Run(name, func(test *testing.T) {
			_, err := parse([]byte(document.Content))

			if err == nil || !strings.Contains(err.Error(), "field "+document.Key+" not found") {
				test.Fatalf("error = %v, want strict decoding to reject the %s key", err, document.Key)
			}
		})
	}
}
