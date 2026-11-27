# estuary_source_http_ingest

[Back to the provider documentation](../../../README.md)

This resource manages an [HTTP Ingest](https://docs.estuary.dev/reference/Connectors/capture-connectors/http-ingest/) capture connector, also called the webhook capture. Estuary hosts an HTTP endpoint for the capture, and each binding captures the JSON documents sent to one of its URL paths, by `POST` or `PUT`, into an Estuary collection. There is no source system to connect to: the paths, their bindings and their collections all come from your configuration. See [Webhook URLs](https://docs.estuary.dev/reference/Connectors/capture-connectors/http-ingest/#webhook-urls) for the address of the endpoint.

| Default image | Connector name |
|---|---|
| `ghcr.io/estuary/source-http-ingest:v1` | `source-http-ingest` |

## Table of Contents

- [Example Usage](#example-usage)
- [Importing](#importing)
- [Attributes](#attributes)
  - [`auto_discover`](#auto_discover)
  - [`backfill`](#backfill)
  - [`config`](#config)
  - [`config.signature_config`](#configsignature_config)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
- [Data Source](#data-source)

## Example Usage

A capture of two paths that requires a bearer token, with the delivery ID of one sender as document ID:

```hcl
variable "webhook_token" {
  type      = string
  sensitive = true
}

resource "estuary_source_http_ingest" "webhooks" {
  name = "webhooks"

  config = {
    paths                = ["/github", "/vendors/{vendorId}/products"]
    allowed_cors_origins = []
    require_auth_token   = var.webhook_token
  }

  bindings = [
    {
      resource = {
        stream         = "/github"
        path           = "/github"
        id_from_header = "X-GitHub-Delivery"
      }
      target = "acmeCo/webhooks/github"
    },
    {
      resource = {
        stream = "/vendors/{vendorId}/products"
        path   = "/vendors/{vendorId}/products"
      }
      target = "acmeCo/webhooks/vendor-products"
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the capture `acmeCo/webhooks/source-http-ingest`.

A capture verifying the signature of Zoom webhooks:

```hcl
variable "zoom_secret_token" {
  type      = string
  sensitive = true
}

resource "estuary_source_http_ingest" "zoom" {
  name = "webhooks/zoom"

  config = {
    paths = ["/zoom"]

    signature_config = {
      provider   = "zoom"
      public_key = var.zoom_secret_token
    }
  }

  bindings = [
    {
      resource = {
        stream = "/zoom"
        path   = "/zoom"
      }
      target = "acmeCo/webhooks/zoom"
    },
  ]
}
```

A capture verifying signatures a sender computes its own way, here an HMAC of the timestamp and the body:

```hcl
variable "webhook_signing_secret" {
  type      = string
  sensitive = true
}

resource "estuary_source_http_ingest" "signed" {
  name = "webhooks/signed"

  config = {
    paths                = ["/events"]
    allowed_cors_origins = ["https://app.acme.example"]

    signature_config = {
      provider                = "custom"
      algorithm               = "hmac_sha256"
      public_key              = var.webhook_signing_secret
      signature_header        = "X-Signature"
      signature_encoding      = "base64"
      signing_string_template = "{TIMESTAMP}:{PAYLOAD}"
      timestamp_header        = "X-Timestamp"
      max_signature_age       = 120
    }
  }

  bindings = [
    {
      resource = {
        stream = "/events"
        path   = "/events"
      }
      target = "acmeCo/webhooks/events"
    },
  ]
}
```

> [!NOTE]
> `require_auth_token` and `public_key` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Importing

```bash
terraform import estuary_source_http_ingest.webhooks acmeCo/webhooks/source-http-ingest
```

Secrets and bindings aren't imported, see [Existing Pipelines](../../../docs/guides/existing-pipelines.md). A configuration that matches the live endpoint and sets none of its secrets plans no changes, and applies keep the secrets Estuary holds. Once the configuration sets a secret or changes `config`, the apply publishes the endpoint with the secrets the configuration sets and no others, so set all of them in that change.

To match the live endpoint:

- Set `name` with the tenant, such as `acmeCo/webhooks`, as the import stores it.
- Write an empty list where the live spec has one. A capture the dashboard created with an auth token has `allowedCorsOrigins: []`, so set `allowed_cors_origins = []`. An unset list and an empty one differ, see [`config`](#config).

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name (e.g., `webhooks`). The provider `tenant` is added in front and `/source-http-ingest` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the capture was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the capture. |
| `data_plane` | String | No | Data plane the capture runs in (e.g., `ops/dp/public/example-data-plane`), defaulting to the provider `data_plane` when the capture is created. Estuary can't move a live capture, so setting another data plane than it runs in fails the plan. See [Use Several Data Planes](../../../docs/guides/multiple-environments.md#use-several-data-planes). |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/source-http-ingest:v1`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `auto_discover` | Object | No | Periodically discovers the paths of `config.paths` and schema changes. See [`auto_discover`](#auto_discover). |
| `config` | Object | Yes | URL paths, authentication and signature verification of the endpoint. See [`config`](#config). |
| `bindings` | Set of Objects | Yes | URL paths accepting requests, one per binding. Their order doesn't matter, as Estuary identifies a binding by its resource path, so plans show only the bindings added, removed or changed. See [`bindings`](#bindings). |
| `shards` | Dynamic | No | Shard settings of the capture, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `auto_discover`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |
| `backfill` | Object | No | Backfill control of the whole capture. See [`backfill`](#backfill). |

### `auto_discover`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `add_new_bindings` | Bool | No | Adds a binding for each path of `config.paths` that isn't declared, when the capture is created and on each later discovery. Terraform doesn't manage those bindings; declare a binding to manage it. |
| `evolve_incompatible_collections` | Bool | No | Automatically updates the target collections when the source schema changes in a way they can't accept. |

### `backfill`

Backfill control of the whole capture, as the dashboard's **Backfill** button and **Backfill mode**. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Set or raise it to backfill every enabled binding of the capture, including those Terraform doesn't manage. Disabled bindings are skipped. Setting it when the capture is created, or in the first apply after its import, backfills nothing. At least 1, and can't be lowered. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |
| `strategy` | String | No | Default strategy of the bindings, which a binding's `backfill.strategy` overrides: `incremental` (default) captures again into the existing collection, `dataflowReset` resets the collection instead, backfilling its materializations and truncating their tables. Changing it alone publishes nothing. |

### `config`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `paths` | List of Strings | No | URL paths the capture accepts requests at, each starting with `/` (e.g., `/orders`). Discovery lists one binding per path. A path can have parameters, as in OpenAPI path templating (e.g., `/vendors/{vendorId}/products`), which are captured under `_meta/pathParams`, see [Path Parameters](https://docs.estuary.dev/reference/Connectors/capture-connectors/http-ingest/#path-parameters). Unset, the connector's own default is `["/webhook-data"]`. |
| `allowed_cors_origins` | List of Strings | No | Request origins allowed through the `Access-Control-Allow-Origin` header. An empty list disables CORS. It can't hold `*` with `require_auth_token`. Unset, the connector's own default is `["*"]`, which allows every origin. See [CORS Allowed Origins](https://docs.estuary.dev/reference/Connectors/capture-connectors/http-ingest/#cors-allowed-origins). |
| `require_auth_token` | String | No | Bearer token every request must send in its `Authorization` header. Unset, anyone who knows the URL of the capture can write to its collections. Write-only, see the note above. |
| `require_auth_token_hash` | String | Read-only | Salted hash of `require_auth_token`, used to detect when it changes. |
| `signature_config` | Object | No | Verifies the signature of each request, rejecting the unsigned and wrongly signed ones. See [`config.signature_config`](#configsignature_config). |

> [!IMPORTANT]
> Set `require_auth_token`, `signature_config` or both. Without either, anyone who learns the URL can write documents to your collections. With `require_auth_token`, also set `allowed_cors_origins` to a list without `*`, such as `[]`: the connector rejects the default `["*"]` alongside a token.

### `config.signature_config`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `provider` | String | Yes | Sender whose signatures are verified: `none` verifies nothing, `twilio` (Twilio SendGrid), `zoom` and `knock` verify theirs with `public_key`, and `custom` verifies the signature the other attributes describe. Only the attributes of the selected provider are used. |
| `public_key` | String | With every provider but `none` | Key the signatures are verified with: the PEM-encoded ECDSA public key of the Twilio SendGrid event webhook, the Zoom webhook secret token, the Knock signing key, or with `custom` the PEM-encoded public key for `ecdsa` or the shared secret for `hmac_sha256`. Write-only, see the note above. |
| `public_key_hash` | String | Read-only | Salted hash of `public_key`, used to detect when it changes. |
| `max_signature_age` | Number | No | Seconds after which a signed request is rejected, from 1 to 259200. With `custom`, it only applies when `timestamp_header` is set. Defaults to `300`. |
| `algorithm` | String | With `custom` | Signature algorithm: `hmac_sha256` or `ecdsa`. |
| `signature_header` | String | With `custom` | HTTP header holding the signature. |
| `signature_encoding` | String | No | Encoding of the signature in its header, with `custom`: `hex` (default) or `base64`. |
| `signing_string_template` | String | With `custom` | Template of the string that is signed. It must include `{PAYLOAD}` and may include `{TIMESTAMP}` (e.g., `{TIMESTAMP}:{PAYLOAD}`). |
| `timestamp_header` | String | No | HTTP header holding the timestamp of the request, with `custom`. |

See [Webhook Signature Verification](https://docs.estuary.dev/reference/Connectors/capture-connectors/http-ingest/#webhook-signature-verification).

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `resource` | Object | Yes | URL path the binding accepts requests at. See [`bindings.resource`](#bindingsresource). |
| `target` | String | No | Estuary collection the data is captured into. A binding without it keeps its current collection. A new one gets the collection the connector discovery names, which `terraform plan` runs against the source (e.g., `acmeCo/webhooks/orders`), see [Bindings](../../../docs/guides/bindings.md). |
| `disable` | Bool | No | Stops accepting requests at the path while keeping the binding in the capture. |
| `backfill` | Object | No | Backfill control of the binding. See [`bindings.backfill`](#bindingsbackfill). |
| `name` | String | Read-only | Collection of the binding, shown in every plan so you can tell which binding changes. |
| `resource_path` | List of String | Read-only | Resource path Estuary identifies the binding by, as in the dashboard. See [Bindings](../../../docs/guides/bindings.md). |

### `bindings.backfill`

Backfill control of the binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Set or raise it to backfill the binding: increment its backfill counter. The connector keeps no documents, so nothing is captured again. Setting it on a binding Estuary doesn't have yet, when the capture is created, or in the first apply after an import, backfills nothing, while setting it on an existing binding Terraform takes over backfills it. At least 1, can't be lowered, and can't be set or raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |
| `strategy` | String | No | Overrides the capture's `backfill.strategy` for this binding: `incremental` captures again into the existing collection, `dataflowReset` resets the collection instead, backfilling its materializations and truncating their tables. Changing it alone publishes nothing. |

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `stream` | String | Yes | Name identifying the binding in the capture, which discovery sets to its URL path (e.g., `/orders`). It is the binding's resource path. |
| `path` | String | No | URL path the binding accepts requests at (e.g., `/orders`). Defaults to the name of its collection. |
| `id_from_header` | String | No | HTTP header whose value becomes the `_meta/webhookId` of each document, which requests must then send. Unset, each document gets a random ID. See [Webhook IDs](https://docs.estuary.dev/reference/Connectors/capture-connectors/http-ingest/#webhook-ids). |

## Data Source

`data "estuary_source_http_ingest"` reads every live binding of a capture by name: the declared ones, those discovery (`auto_discover`) adds and those added in the dashboard. Terraform doesn't manage live bindings, so the resource neither stores them nor shows them in plans: read them here, for a capture of this configuration or one managed elsewhere:

```hcl
# A capture of this configuration
data "estuary_source_http_ingest" "source" {
  name = estuary_source_http_ingest.source.name
}

# A capture managed elsewhere
data "estuary_source_http_ingest" "other" {
  name = "webhooks"
}
```

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name, as in the resource: the provider `tenant` is added in front and `/source-http-ingest` at the end, unless they're already there. |
| `live_bindings` | List of Objects | Read-only | Every live binding of the capture, with the attributes of [`bindings`](#bindings). |

> [!NOTE]
> See [Bindings](../../../docs/guides/bindings.md) for when the data source is read and how to use it.

