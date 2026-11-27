# estuary_source_google_search_console

[Back to the provider documentation](../../../README.md)

This resource manages a [Google Search Console](https://docs.estuary.dev/reference/Connectors/capture-connectors/google-search-console/) capture connector. Each binding captures one Search Console stream into an Estuary collection: the search analytics by date, country, device, page or query, the sitemaps, the sites, or a [custom report](https://docs.estuary.dev/reference/Connectors/capture-connectors/google-search-console/#custom-reports).

| Default image | Connector name |
|---|---|
| `ghcr.io/estuary/source-google-search-console:v2` | `source-google-search-console` |

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

A capture of two properties authenticating with a service account key, with a custom report:

```hcl
variable "search_console_service_account" {
  type      = string
  sensitive = true
}

resource "estuary_source_google_search_console" "seo" {
  name = "seo"

  config = {
    site_urls  = ["https://www.acme.example/", "sc-domain:acme.example"]
    start_date = "2024-01-01"

    custom_reports = jsonencode([
      { name = "by_country_and_device", dimensions = ["date", "country", "device"] },
    ])

    credentials = {
      auth_type            = "Service"
      service_account_info = var.search_console_service_account
      email                = "admin@acme.example"
    }
  }

  bindings = [
    {
      resource = {
        stream       = "search_analytics_by_date"
        sync_mode    = "incremental"
        cursor_field = ["date"]
      }
    },
    {
      resource = {
        stream       = "by_country_and_device"
        sync_mode    = "incremental"
        cursor_field = ["date"]
      }
      target = "acmeCo/seo/by-country-and-device"
    },
    {
      resource = {
        stream    = "sites"
        sync_mode = "full_refresh"
      }
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the capture `acmeCo/seo/source-google-search-console`. The bindings without a `target` land in the collections the connector discovery names, such as `acmeCo/seo/sites`.

A capture authenticating with the OAuth client of your own Google Cloud project:

```hcl
variable "google_client_id" {
  type      = string
  sensitive = true
}

variable "google_client_secret" {
  type      = string
  sensitive = true
}

variable "google_refresh_token" {
  type      = string
  sensitive = true
}

resource "estuary_source_google_search_console" "oauth" {
  name = "seo/oauth"

  config = {
    site_urls  = ["sc-domain:acme.example"]
    start_date = "2024-01-01"

    credentials = {
      auth_type     = "Client"
      client_id     = var.google_client_id
      client_secret = var.google_client_secret
      refresh_token = var.google_refresh_token
    }
  }

  bindings = [
    {
      resource = {
        stream       = "search_analytics_by_page"
        sync_mode    = "incremental"
        cursor_field = ["date"]
      }
    },
  ]
}
```

A capture created through the dashboard's Google login, imported with its secrets left out, see [Importing](#importing):

```hcl
resource "estuary_source_google_search_console" "imported" {
  name = "seo/imported"

  config = {
    site_urls  = ["sc-domain:acme.example"]
    start_date = "2024-01-01"

    credentials = {
      auth_type = "Client"
    }
  }

  bindings = []
}
```

> [!NOTE]
> `client_id`, `client_secret`, `refresh_token` and `service_account_info` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Importing

```bash
terraform import estuary_source_google_search_console.imported acmeCo/seo/imported/source-google-search-console
```

Secrets and bindings aren't imported, see [Existing Pipelines](../../../docs/guides/existing-pipelines.md). A configuration that matches the live endpoint and sets none of its secrets plans no changes, and applies keep the secrets Estuary holds.

A capture created through the dashboard's Google login uses the `Client` method with the client ID, client secret and refresh token of Estuary's OAuth client, which you can't know. Leave the three out, as in the last example: Terraform then manages the bindings and the other settings, and the sealed credentials stay live. They stay only while the configuration sets no secret and doesn't change `config`. Once it does either, the apply publishes the endpoint with the secrets the configuration sets and no others. To change `config` on such a capture, such as its `site_urls` or `custom_reports`, switch it to `Service`, or to the `Client` credentials of your own OAuth client, in the same change.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name (e.g., `seo`). The provider `tenant` is added in front and `/source-google-search-console` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the capture was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the capture. |
| `data_plane` | String | No | Data plane the capture runs in (e.g., `ops/dp/public/example-data-plane`), defaulting to the provider `data_plane` when the capture is created. Estuary can't move a live capture, so setting another data plane than it runs in fails the plan. See [Use Several Data Planes](../../../docs/guides/multiple-environments.md#use-several-data-planes). |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/source-google-search-console:v2`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `auto_discover` | Object | No | Periodically checks the Search Console properties for new streams and schema changes. See [`auto_discover`](#auto_discover). |
| `config` | Object | Yes | Properties, date range, custom reports and authentication of the capture. See [`config`](#config). |
| `bindings` | Set of Objects | Yes | Captured Search Console streams, one per binding. Their order doesn't matter, as Estuary identifies a binding by its resource path, so plans show only the bindings added, removed or changed. See [`bindings`](#bindings). |
| `shards` | Dynamic | No | Shard settings of the capture, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `auto_discover`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |
| `backfill` | Object | No | Backfill control of the whole capture. See [`backfill`](#backfill). |

### `auto_discover`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `add_new_bindings` | Bool | No | Adds a binding for each stream found in the Search Console properties that isn't declared, when the capture is created and on each later discovery. Terraform doesn't manage those bindings; declare a binding to manage it. |
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
| `site_urls` | List of Strings | Yes | Search Console properties to capture, as their URL (e.g., `https://example.com/`) or domain property (e.g., `sc-domain:example.com`). |
| `start_date` | String | Yes | Captures only the data at or after this UTC date, as `YYYY-MM-DD`. |
| `end_date` | String | No | Captures only the data at or before this UTC date, as `YYYY-MM-DD`, at or after `start_date`. Unset, the capture keeps following new data. |
| `custom_reports` | String | No | Custom reports captured besides the built-in ones, as a JSON array of objects with a `name` and its `dimensions`, which `jsonencode` builds. Each report becomes a stream. See [Custom Reports](https://docs.estuary.dev/reference/Connectors/capture-connectors/google-search-console/#custom-reports). |
| `credentials` | Object | Yes | How the connector authenticates with Google. See [`config.credentials`](#configcredentials). |

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `auth_type` | String | Yes | Authentication method: `Client` (an OAuth client with a refresh token, as the dashboard login stores) or `Service` (the JSON key of a Google service account). Only the attributes of the selected method are used. |
| `client_id` | String | With your own client's `Client` | Client ID of the OAuth client the connector authenticates as. Leave it out of a capture authenticated through the dashboard login, whose OAuth client is Estuary's. Write-only, see the note above. |
| `client_id_hash` | String | Read-only | Salted hash of `client_id`, used to detect when it changes. |
| `client_secret` | String | With your own client's `Client` | Client secret of the OAuth client. Leave it out of a capture authenticated through the dashboard login. Write-only, see the note above. |
| `client_secret_hash` | String | Read-only | Salted hash of `client_secret`, used to detect when it changes. |
| `refresh_token` | String | With your own client's `Client` | Refresh token the OAuth client was granted. Leave it out of a capture authenticated through the dashboard login. Write-only, see the note above. |
| `refresh_token_hash` | String | Read-only | Salted hash of `refresh_token`, used to detect when it changes. |
| `service_account_info` | String | With `Service` | JSON key of the Google service account. Write-only, see the note above. |
| `service_account_info_hash` | String | Read-only | Salted hash of `service_account_info`, used to detect when it changes. |
| `email` | String | With `Service` | Email of the Google Workspace user the service account acts as, through domain-wide delegation. |

> [!TIP]
> With `Service`, the service account needs domain-wide delegation for the `https://www.googleapis.com/auth/webmasters.readonly` scope. See [Authenticating Manually with a Service Account Key](https://docs.estuary.dev/reference/Connectors/capture-connectors/google-search-console/#authenticating-manually-with-a-service-account-key).

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `resource` | Object | Yes | Search Console stream the binding captures. See [`bindings.resource`](#bindingsresource). |
| `target` | String | No | Estuary collection the data is captured into. A binding without it keeps its current collection. A new one gets the collection the connector discovery names, which `terraform plan` runs against the source (e.g., `acmeCo/seo/sites`), see [Bindings](../../../docs/guides/bindings.md). |
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
| `stream` | String | Yes | Name of the captured stream, as the connector discovery and the dashboard list it (e.g., `search_analytics_by_date`). |
| `sync_mode` | String | Yes | How the stream is captured: `incremental` reads what changed since the last sync, `full_refresh` reads all of it each time. Use the mode discovery gives the stream. |
| `namespace` | String | No | Namespace of the stream. The streams of this connector have none. |
| `cursor_field` | List of Strings | No | Path of the field an incremental stream is ordered by (e.g., `["date"]`). Unset, the stream's own default cursor is used. |

## Data Source

`data "estuary_source_google_search_console"` reads every live binding of a capture by name: the declared ones, those discovery (`auto_discover`) adds and those added in the dashboard. Terraform doesn't manage live bindings, so the resource neither stores them nor shows them in plans: read them here, for a capture of this configuration or one managed elsewhere:

```hcl
# A capture of this configuration
data "estuary_source_google_search_console" "source" {
  name = estuary_source_google_search_console.source.name
}

# A capture managed elsewhere
data "estuary_source_google_search_console" "other" {
  name = "seo"
}
```

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name, as in the resource: the provider `tenant` is added in front and `/source-google-search-console` at the end, unless they're already there. |
| `live_bindings` | List of Objects | Read-only | Every live binding of the capture, with the attributes of [`bindings`](#bindings). |

> [!NOTE]
> See [Bindings](../../../docs/guides/bindings.md) for when the data source is read and how to use it.

