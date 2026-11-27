package httpingest

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
)

// SignatureProvider is the sender whose webhook signatures the connector verifies.
type SignatureProvider string

// SignatureAlgorithm is how a custom provider signs its requests.
type SignatureAlgorithm string

// SignatureEncoding is how a custom provider encodes the signature in its header.
type SignatureEncoding string

// Image is the HTTP Ingest connector image.
const Image string = "ghcr.io/estuary/source-http-ingest:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "source-http-ingest"

const (
	SignatureProviderNone   SignatureProvider = "none"
	SignatureProviderTwilio SignatureProvider = "twilio"
	SignatureProviderZoom   SignatureProvider = "zoom"
	SignatureProviderKnock  SignatureProvider = "knock"
	SignatureProviderCustom SignatureProvider = "custom"
)

// SignatureProviders lists every supported signature provider, useful for schema validators.
var SignatureProviders = []SignatureProvider{
	SignatureProviderNone,
	SignatureProviderTwilio,
	SignatureProviderZoom,
	SignatureProviderKnock,
	SignatureProviderCustom,
}

const (
	SignatureAlgorithmHMACSHA256 SignatureAlgorithm = "hmac_sha256"
	SignatureAlgorithmECDSA      SignatureAlgorithm = "ecdsa"
)

// SignatureAlgorithms lists every supported signature algorithm, useful for schema validators.
var SignatureAlgorithms = []SignatureAlgorithm{
	SignatureAlgorithmHMACSHA256,
	SignatureAlgorithmECDSA,
}

const (
	SignatureEncodingHex    SignatureEncoding = "hex"
	SignatureEncodingBase64 SignatureEncoding = "base64"
)

// SignatureEncodings lists every supported signature encoding, useful for schema validators.
var SignatureEncodings = []SignatureEncoding{
	SignatureEncodingHex,
	SignatureEncodingBase64,
}

// SignatureConfig verifies the signature of each request, with the fields of its Provider.
type SignatureConfig struct {
	Provider              SignatureProvider  `yaml:"provider" enum:"none,twilio,zoom,knock,custom" description:"Sender whose signatures are verified: 'none' verifies nothing, 'twilio' (Twilio SendGrid), 'zoom' and 'knock' verify theirs with public_key, and 'custom' verifies the signature the other attributes describe. Only the attributes of the selected provider are used."`
	PublicKey             string             `yaml:"publicKey,omitempty" secret:"true" description:"Key the signatures are verified with, required by every provider but 'none': the PEM-encoded ECDSA public key of the Twilio SendGrid event webhook, the Zoom webhook secret token, the Knock signing key, or with 'custom' the PEM-encoded public key for 'ecdsa' or the shared secret for 'hmac_sha256'. Write-only."`
	MaxSignatureAge       int                `yaml:"maxSignatureAge,omitempty" description:"Seconds after which a signed request is rejected, from 1 to 259200. With 'custom', it only applies when timestamp_header is set. Defaults to 300."`
	Algorithm             SignatureAlgorithm `yaml:"algorithm,omitempty" enum:"hmac_sha256,ecdsa" description:"Signature algorithm, required with 'custom': 'hmac_sha256' or 'ecdsa'."`
	SignatureHeader       string             `yaml:"signatureHeader,omitempty" description:"HTTP header holding the signature, required with 'custom'."`
	SignatureEncoding     SignatureEncoding  `yaml:"signatureEncoding,omitempty" enum:"hex,base64" description:"Encoding of the signature in its header, with 'custom': 'hex' (default) or 'base64'."`
	SigningStringTemplate string             `yaml:"signingStringTemplate,omitempty" description:"Template of the string that is signed, required with 'custom'. It must include {PAYLOAD} and may include {TIMESTAMP} (e.g., '{TIMESTAMP}:{PAYLOAD}')."`
	TimestampHeader       string             `yaml:"timestampHeader,omitempty" description:"HTTP header holding the timestamp of the request, with 'custom'."`
}

// Config is the source-http-ingest connector configuration. Paths and AllowedCORSOrigins are pointers because an
// empty list differs from an unset one, which takes the connector's default.
type Config struct {
	Paths              *[]string        `yaml:"paths,omitempty" description:"URL paths the capture accepts requests at, each starting with / (e.g., '/orders'). Discovery lists one binding per path. A path can have parameters, as in OpenAPI path templating (e.g., '/vendors/{vendorId}/products'), which are captured under _meta/pathParams. Unset, the connector's own default is ['/webhook-data']."`
	AllowedCORSOrigins *[]string        `yaml:"allowedCorsOrigins,omitempty" description:"Request origins allowed through the Access-Control-Allow-Origin header. An empty list disables CORS. It can't hold '*' with require_auth_token. Unset, the connector's own default is ['*'], which allows every origin."`
	RequireAuthToken   string           `yaml:"requireAuthToken,omitempty" description:"Bearer token every request must send in its Authorization header. Unset, anyone who knows the URL of the capture can write to its collections. Write-only."`
	SignatureConfig    *SignatureConfig `yaml:"signatureConfig,omitempty" description:"Verifies the signature of each request, rejecting the unsigned and wrongly signed ones."`
}

// Resource identifies the URL path a binding accepts requests at.
type Resource struct {
	Stream       string `yaml:"stream" description:"Name identifying the binding in the capture, which discovery sets to its URL path (e.g., '/orders')."`
	Path         string `yaml:"path,omitempty" description:"URL path the binding accepts requests at (e.g., '/orders'). Defaults to the name of its collection."`
	IDFromHeader string `yaml:"idFromHeader,omitempty" description:"HTTP header whose value becomes the _meta/webhookId of each document, which requests must then send. Unset, each document gets a random ID."`
}

// ResourcePath returns the stream, satisfying capture.ResourcePather.
func (resource Resource) ResourcePath() []string {
	return []string{resource.Stream}
}

// Binding maps a URL path to the collection its requests are captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full HTTP Ingest capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
