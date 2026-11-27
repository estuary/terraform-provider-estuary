# estuary_source_zendesk_support

[Back to the provider documentation](../../../README.md)

This resource manages a [Zendesk Support](https://docs.estuary.dev/reference/Connectors/capture-connectors/zendesk-support-native/) capture connector. Each binding captures one Zendesk Support resource, such as tickets, users or organizations, into an Estuary collection, by polling the Zendesk API.

| Default image | Connector name |
|---|---|
| `ghcr.io/estuary/source-zendesk-support-native:v1` | `source-zendesk-support-native` |

The resource manages captures on the `source-zendesk-support-native` image only. A capture on the legacy `source-zendesk-support` image has to be created again on this connector before Terraform can manage it.

## Table of Contents

- [Example Usage](#example-usage)
- [Importing](#importing)
- [Attributes](#attributes)
  - [`auto_discover`](#auto_discover)
  - [`backfill`](#backfill)
  - [`config`](#config)
  - [`config.credentials`](#configcredentials)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
- [Data Source](#data-source)

## Example Usage

A capture authenticating with an agent's email and an API token, with auto discovery adding every new resource:

```hcl
variable "zendesk_api_token" {
  type      = string
  sensitive = true
}

resource "estuary_source_zendesk_support" "support" {
  name = "support"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    subdomain  = "acme"
    start_date = "2024-01-01T00:00:00Z"

    credentials = {
      credentials_title = "Email & API Token"
      username          = "support-bot@acme.example"
      password          = var.zendesk_api_token
    }
  }

  bindings = [
    {
      resource = {
        name     = "tickets"
        interval = "PT5M"
      }
    },
    {
      resource = {
        name     = "users"
        interval = "PT5M"
      }
      target = "acmeCo/support/zendesk-users"
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the capture `acmeCo/support/source-zendesk-support-native`. The `tickets` binding sets no `target`, so its data lands in the collection the connector discovery names, such as `acmeCo/support/tickets`.

A capture created through the dashboard's Zendesk login, imported with its secrets left out, see [Importing](#importing):

```hcl
resource "estuary_source_zendesk_support" "imported" {
  name = "support/imported"

  config = {
    subdomain = "acme"

    credentials = {
      credentials_title = "OAuth Credentials"
    }
  }

  bindings = []
}
```

> [!NOTE]
> `password`, `client_id`, `client_secret`, `refresh_token`, `access_token` and `access_token_expires_at` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Importing

```bash
terraform import estuary_source_zendesk_support.imported acmeCo/support/imported/source-zendesk-support-native
```

Secrets and bindings aren't imported, see [Existing Pipelines](../../../docs/guides/existing-pipelines.md). A configuration that matches the live endpoint and sets none of its secrets plans no changes, and applies keep the secrets Estuary holds.

A capture created through the dashboard's Zendesk login uses `OAuth Credentials` with the client and tokens of Estuary's OAuth client, which you can't know. Leave them out, as in the last example: Terraform then manages the bindings and the other settings, and the sealed credentials stay live. They stay only while the configuration sets no secret and doesn't change `config`. Once it does either, the apply publishes the endpoint with the secrets the configuration sets and no others. To change `config` on such a capture, switch it to `Email & API Token`, or to the `OAuth Credentials` of your own OAuth client, in the same change.

With `OAuth Credentials`, the connector replaces its access token, its refresh token and the token's expiry each time the access token expires. Terraform doesn't report that as a change: the three are write-only, and the provider knows this connector rotates its secrets. The tokens in your configuration go stale, though, once the connector has replaced them. An apply that publishes the endpoint, such as one changing `subdomain` or `start_date`, sends the configured tokens again, so set fresh ones in that change. `Email & API Token` credentials don't rotate, which makes them the simpler choice for a capture Terraform creates.

The connector also accepts `Deprecated OAuth Credentials`, which the resource doesn't have. A capture using them imports, but no configuration can match it, as `credentials_title` only accepts the two methods above. Move the capture to one of them in the dashboard first.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name (e.g., `support`). The provider `tenant` is added in front and `/source-zendesk-support-native` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the capture was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the capture. |
| `data_plane` | String | No | Data plane the capture runs in (e.g., `ops/dp/public/example-data-plane`), defaulting to the provider `data_plane` when the capture is created. Estuary can't move a live capture, so setting another data plane than it runs in fails the plan. See [Use Several Data Planes](../../../docs/guides/multiple-environments.md#use-several-data-planes). |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/source-zendesk-support-native:v1`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `auto_discover` | Object | No | Periodically checks the Zendesk account for new resources and schema changes. See [`auto_discover`](#auto_discover). |
| `config` | Object | Yes | Account, authentication and capture settings of Zendesk Support. See [`config`](#config). |
| `bindings` | Set of Objects | Yes | Captured Zendesk Support resources, one per binding. Their order doesn't matter, as Estuary identifies a binding by its resource path, so plans show only the bindings added, removed or changed. See [`bindings`](#bindings). |
| `shards` | Dynamic | No | Shard settings of the capture, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `auto_discover`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |
| `backfill` | Object | No | Backfill control of the whole capture. See [`backfill`](#backfill). |

### `auto_discover`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `add_new_bindings` | Bool | No | Adds a binding for each resource found in the Zendesk account that isn't declared, when the capture is created and on each later discovery. Terraform doesn't manage those bindings; declare a binding to manage it. |
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
| `subdomain` | String | Yes | Subdomain of the Zendesk account: `acme` for `https://acme.zendesk.com`. |
| `start_date` | String | No | Captures only the data created or changed at or after this UTC time, as `YYYY-MM-DDTHH:MM:SSZ`. Defaults to 30 days before the capture is created. |
| `credentials` | Object | Yes | How the connector authenticates with Zendesk. See [`config.credentials`](#configcredentials). |

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `credentials_title` | String | Yes | Authentication method: `Email & API Token` (an agent's email and a Zendesk API token) or `OAuth Credentials` (an OAuth client with the tokens it was granted, as the dashboard login stores). Only the attributes of the selected method are used. |
| `username` | String | With `Email & API Token` | Email of the Zendesk agent the API token belongs to. |
| `password` | String | With `Email & API Token` | Zendesk API token. Write-only, see the note above. |
| `password_hash` | String | Read-only | Salted hash of `password`, used to detect when it changes. |
| `client_id` | String | With your own client's `OAuth Credentials` | Client ID of the OAuth client the connector authenticates as. Leave it out of a capture authenticated through the dashboard login, whose OAuth client is Estuary's. Write-only, see the note above. |
| `client_id_hash` | String | Read-only | Salted hash of `client_id`, used to detect when it changes. |
| `client_secret` | String | With your own client's `OAuth Credentials` | Client secret of the OAuth client. Leave it out of a capture authenticated through the dashboard login. Write-only, see the note above. |
| `client_secret_hash` | String | Read-only | Salted hash of `client_secret`, used to detect when it changes. |
| `refresh_token` | String | With your own client's `OAuth Credentials` | Refresh token the OAuth client was granted. The connector replaces it each time it refreshes the access token. Leave it out of a capture authenticated through the dashboard login. Write-only, see the note above. |
| `refresh_token_hash` | String | Read-only | Salted hash of `refresh_token`, used to detect when it changes. |
| `access_token` | String | With your own client's `OAuth Credentials` | Access token the OAuth client was granted. The connector replaces it when it expires. Leave it out of a capture authenticated through the dashboard login. Write-only, see the note above. |
| `access_token_hash` | String | Read-only | Salted hash of `access_token`, used to detect when it changes. |
| `access_token_expires_at` | String | With your own client's `OAuth Credentials` | UTC time `access_token` expires at, as `YYYY-MM-DDTHH:MM:SSZ`. The connector replaces it with each new access token, so Terraform sets it and never reads it back. Leave it out of a capture authenticated through the dashboard login. Write-only, see the note above. |
| `access_token_expires_at_hash` | String | Read-only | Salted hash of `access_token_expires_at`, used to detect when it changes. |

> [!TIP]
> See [API Token Authentication](https://docs.estuary.dev/reference/Connectors/capture-connectors/zendesk-support-native/#api-token-authentication) to generate the API token, and [OAuth2 Authentication](https://docs.estuary.dev/reference/Connectors/capture-connectors/zendesk-support-native/#oauth2-authentication) for what an OAuth client needs.

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `resource` | Object | Yes | Zendesk Support resource the binding captures. See [`bindings.resource`](#bindingsresource). |
| `target` | String | No | Estuary collection the data is captured into. A binding without it keeps its current collection. A new one gets the collection the connector discovery names, which `terraform plan` runs against the source (e.g., `acmeCo/support/tickets`), see [Bindings](../../../docs/guides/bindings.md). |
| `disable` | Bool | No | Stops capturing the resource while keeping the binding in the capture. |
| `backfill` | Object | No | Backfill control of the binding. See [`bindings.backfill`](#bindingsbackfill). |
| `name` | String | Read-only | Collection of the binding, shown in every plan so you can tell which binding changes. |
| `resource_path` | List of String | Read-only | Resource path Estuary identifies the binding by, as in the dashboard. See [Bindings](../../../docs/guides/bindings.md). |

### `bindings.backfill`

Backfill control of the binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Set or raise it to backfill the binding: capture the whole resource again from the start. Setting it on a binding Estuary doesn't have yet, when the capture is created, or in the first apply after an import, backfills nothing, while setting it on an existing binding Terraform takes over backfills it. At least 1, can't be lowered, and can't be set or raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |
| `strategy` | String | No | Overrides the capture's `backfill.strategy` for this binding: `incremental` captures again into the existing collection, `dataflowReset` resets the collection instead, backfilling its materializations and truncating their tables. Changing it alone publishes nothing. |

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Name of the captured stream, as the connector discovery and the dashboard list it. |
| `interval` | String | No | Time between two polls of the stream, as an ISO 8601 duration such as `PT5M`. Set it to the interval discovery gives the stream, as the dashboard shows: omitted, the connector polls again right away (`PT0S`). |

## Data Source

`data "estuary_source_zendesk_support"` reads every live binding of a capture by name: the declared ones, those discovery (`auto_discover`) adds and those added in the dashboard. Terraform doesn't manage live bindings, so the resource neither stores them nor shows them in plans: read them here, for a capture of this configuration or one managed elsewhere:

```hcl
# A capture of this configuration
data "estuary_source_zendesk_support" "source" {
  name = estuary_source_zendesk_support.source.name
}

# A capture managed elsewhere
data "estuary_source_zendesk_support" "other" {
  name = "support"
}
```

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name, as in the resource: the provider `tenant` is added in front and `/source-zendesk-support-native` at the end, unless they're already there. |
| `live_bindings` | List of Objects | Read-only | Every live binding of the capture, with the attributes of [`bindings`](#bindings). |

> [!NOTE]
> See [Bindings](../../../docs/guides/bindings.md) for when the data source is read and how to use it.

