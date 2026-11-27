# estuary_source_apple_app_store

[Back to the provider documentation](../../../README.md)

This resource manages an [Apple App Store](https://docs.estuary.dev/reference/Connectors/capture-connectors/apple-app-store/) capture connector. Each binding captures one stream of the App Store Connect API, the customer reviews or one of the analytics reports, into an Estuary collection.

| Default image | Connector name |
|---|---|
| `ghcr.io/estuary/source-apple-app-store:v1` | `source-apple-app-store` |

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

A capture of the customer reviews of two apps, with auto discovery adding every other stream:

```hcl
variable "app_store_private_key" {
  type      = string
  sensitive = true
}

variable "app_store_key_id" {
  type      = string
  sensitive = true
}

variable "app_store_issuer_id" {
  type      = string
  sensitive = true
}

resource "estuary_source_apple_app_store" "apps" {
  name = "apps"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    app_ids = ["1234567890", "9876543210"]

    credentials = {
      credentials_title = "Private App Credentials"
      private_key       = var.app_store_private_key
      key_id            = var.app_store_key_id
      issuer_id         = var.app_store_issuer_id
    }
  }

  bindings = [
    {
      resource = {
        name     = "app_reviews"
        interval = "PT1H"
      }
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the capture `acmeCo/apps/source-apple-app-store`. The binding sets no `target`, so its data lands in the collection the connector discovery names, such as `acmeCo/apps/app_reviews`.

> [!NOTE]
> `private_key`, `key_id`, `issuer_id` and `access_token` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Importing

```bash
terraform import estuary_source_apple_app_store.apps acmeCo/apps/source-apple-app-store
```

Secrets and bindings aren't imported, see [Existing Pipelines](../../../docs/guides/existing-pipelines.md). A configuration that matches the live endpoint and leaves the API key out plans no changes, and applies keep the key Estuary holds. Once the configuration sets one of the secrets or changes `config`, the apply publishes the endpoint with the configured secrets and no others, so set `private_key`, `key_id` and `issuer_id` in that change.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name (e.g., `apps`). The provider `tenant` is added in front and `/source-apple-app-store` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the capture was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the capture. |
| `data_plane` | String | No | Data plane the capture runs in (e.g., `ops/dp/public/example-data-plane`), defaulting to the provider `data_plane` when the capture is created. Estuary can't move a live capture, so setting another data plane than it runs in fails the plan. See [Use Several Data Planes](../../../docs/guides/multiple-environments.md#use-several-data-planes). |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/source-apple-app-store:v1`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `auto_discover` | Object | No | Periodically checks App Store Connect for new streams and schema changes. See [`auto_discover`](#auto_discover). |
| `config` | Object | Yes | API key and apps of the capture. See [`config`](#config). |
| `bindings` | Set of Objects | Yes | Captured App Store streams, one per binding. Their order doesn't matter, as Estuary identifies a binding by its resource path, so plans show only the bindings added, removed or changed. See [`bindings`](#bindings). |
| `shards` | Dynamic | No | Shard settings of the capture, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `auto_discover`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |
| `backfill` | Object | No | Backfill control of the whole capture. See [`backfill`](#backfill). |

### `auto_discover`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `add_new_bindings` | Bool | No | Adds a binding for each stream found in App Store Connect that isn't declared, when the capture is created and on each later discovery. Terraform doesn't manage those bindings; declare a binding to manage it. |
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
| `credentials` | Object | Yes | App Store Connect API key the connector authenticates with. See [`config.credentials`](#configcredentials). |
| `app_ids` | List of Strings | No | IDs of the apps to capture. Unset, every app the API key can access is captured. |

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `credentials_title` | String | Yes | Authentication method, which must be `Private App Credentials`, the only one the connector has. |
| `private_key` | String | To create the capture | Contents of the private key file (`.p8`) of the App Store Connect API key, with its `BEGIN` and `END` lines. The connector requires it; an imported capture can leave it out to keep the one Estuary holds. Write-only, see the note above. |
| `private_key_hash` | String | Read-only | Salted hash of `private_key`, used to detect when it changes. |
| `key_id` | String | To create the capture | Key ID of the App Store Connect API key. The connector requires it; an imported capture can leave it out. Write-only, see the note above. |
| `key_id_hash` | String | Read-only | Salted hash of `key_id`, used to detect when it changes. |
| `issuer_id` | String | To create the capture | Issuer ID of the App Store Connect API keys, under **Users and Access > Keys**. The connector requires it; an imported capture can leave it out. Write-only, see the note above. |
| `issuer_id_hash` | String | Read-only | Salted hash of `issuer_id`, used to detect when it changes. |
| `access_token` | String | No | Access token, which the dashboard hides and leaves empty. Leave it unset. Write-only, see the note above. |
| `access_token_hash` | String | Read-only | Salted hash of `access_token`, used to detect when it changes. |

> [!TIP]
> The analytics reports need an API key with the Admin role, and Apple lets you download its private key only once. See [Prerequisites](https://docs.estuary.dev/reference/Connectors/capture-connectors/apple-app-store/#prerequisites).

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `resource` | Object | Yes | App Store stream the binding captures. See [`bindings.resource`](#bindingsresource). |
| `target` | String | No | Estuary collection the data is captured into. A binding without it keeps its current collection. A new one gets the collection the connector discovery names, which `terraform plan` runs against the source (e.g., `acmeCo/apps/app_reviews`), see [Bindings](../../../docs/guides/bindings.md). |
| `disable` | Bool | No | Stops capturing the stream while keeping the binding in the capture. |
| `backfill` | Object | No | Backfill control of the binding. See [`bindings.backfill`](#bindingsbackfill). |
| `name` | String | Read-only | Collection of the binding, shown in every plan so you can tell which binding changes. |
| `resource_path` | List of String | Read-only | Resource path Estuary identifies the binding by, as in the dashboard. See [Bindings](../../../docs/guides/bindings.md). |

### `bindings.backfill`

Backfill control of the binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Set or raise it to backfill the binding: capture the whole stream again from the start. Setting it on a binding Estuary doesn't have yet, when the capture is created, or in the first apply after an import, backfills nothing, while setting it on an existing binding Terraform takes over backfills it. At least 1, can't be lowered, and can't be set or raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |
| `strategy` | String | No | Overrides the capture's `backfill.strategy` for this binding: `incremental` captures again into the existing collection, `dataflowReset` resets the collection instead, backfilling its materializations and truncating their tables. Changing it alone publishes nothing. |

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Name of the captured stream, as the connector discovery and the dashboard list it. |
| `interval` | String | No | Time between two polls of the stream, as an ISO 8601 duration such as `PT5M`. Set it to the interval discovery gives the stream, as the dashboard shows: omitted, the connector polls again right away (`PT0S`). |

## Data Source

`data "estuary_source_apple_app_store"` reads every live binding of a capture by name: the declared ones, those discovery (`auto_discover`) adds and those added in the dashboard. Terraform doesn't manage live bindings, so the resource neither stores them nor shows them in plans: read them here, for a capture of this configuration or one managed elsewhere:

```hcl
# A capture of this configuration
data "estuary_source_apple_app_store" "source" {
  name = estuary_source_apple_app_store.source.name
}

# A capture managed elsewhere
data "estuary_source_apple_app_store" "other" {
  name = "apps"
}
```

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name, as in the resource: the provider `tenant` is added in front and `/source-apple-app-store` at the end, unless they're already there. |
| `live_bindings` | List of Objects | Read-only | Every live binding of the capture, with the attributes of [`bindings`](#bindings). |

> [!NOTE]
> See [Bindings](../../../docs/guides/bindings.md) for when the data source is read and how to use it.

