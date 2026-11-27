# estuary_source_salesforce

[Back to the provider documentation](../../../README.md)

This resource manages a [Salesforce](https://docs.estuary.dev/reference/Connectors/capture-connectors/Salesforce/salesforce-native/) capture connector. Each binding captures one Salesforce object, standard or custom, into an Estuary collection. The connector backfills through the Bulk API 2.0 and polls the REST API for changes.

| Default image | Connector name |
|---|---|
| `ghcr.io/estuary/source-salesforce-native:v1` | `source-salesforce-native` |

The resource manages captures on the `source-salesforce-native` image only. A capture on a legacy image, such as `source-salesforce` or `source-salesforce-next`, has to be created again on this connector before Terraform can manage it.

## Table of Contents

- [Example Usage](#example-usage)
- [Importing](#importing)
- [Attributes](#attributes)
  - [`auto_discover`](#auto_discover)
  - [`backfill`](#backfill)
  - [`config`](#config)
  - [`config.credentials`](#configcredentials)
  - [`config.advanced`](#configadvanced)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
- [Data Source](#data-source)

## Example Usage

A capture authenticating through the client credentials flow of an external client app, with auto discovery adding every new object:

```hcl
variable "salesforce_consumer_key" {
  type      = string
  sensitive = true
}

variable "salesforce_consumer_secret" {
  type      = string
  sensitive = true
}

resource "estuary_source_salesforce" "crm" {
  name = "crm"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    my_domain  = "acme.my.salesforce.com"
    start_date = "2024-01-01T00:00:00Z"

    credentials = {
      credentials_title = "Client Credentials"
      client_id         = var.salesforce_consumer_key
      client_secret     = var.salesforce_consumer_secret
    }
  }

  bindings = [
    {
      resource = {
        name     = "Account"
        interval = "PT5M"
        schedule = "55 23 * * *"
      }
    },
    {
      resource = {
        name     = "Opportunity"
        interval = "PT5M"
      }
      target = "acmeCo/crm/opportunities"
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the capture `acmeCo/crm/source-salesforce-native`. The `Account` binding sets no `target`, so its data lands in the collection the connector discovery names, such as `acmeCo/crm/Account`.

A sandbox capture authenticating with the OAuth credentials of your own connected app, with a smaller incremental query window:

```hcl
variable "salesforce_client_id" {
  type      = string
  sensitive = true
}

variable "salesforce_client_secret" {
  type      = string
  sensitive = true
}

variable "salesforce_refresh_token" {
  type      = string
  sensitive = true
}

resource "estuary_source_salesforce" "sandbox" {
  name = "crm/sandbox"

  config = {
    is_sandbox = true

    credentials = {
      credentials_title = "OAuth Credentials"
      client_id         = var.salesforce_client_id
      client_secret     = var.salesforce_client_secret
      refresh_token     = var.salesforce_refresh_token
      instance_url      = "https://acme--uat.sandbox.my.salesforce.com"
    }

    advanced = {
      window_size = {
        window_type = "days"
        days        = 30
      }
    }
  }

  bindings = [
    {
      resource = {
        name     = "Contact"
        interval = "PT15M"
      }
    },
  ]
}
```

A capture created through the dashboard's Salesforce login, imported with its secrets left out, see [Importing](#importing):

```hcl
resource "estuary_source_salesforce" "imported" {
  name = "crm/imported"

  config = {
    credentials = {
      credentials_title = "OAuth Credentials"
      instance_url      = "https://acme.my.salesforce.com"
    }
  }

  bindings = []
}
```

> [!NOTE]
> `client_id`, `client_secret` and `refresh_token` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Importing

```bash
terraform import estuary_source_salesforce.imported acmeCo/crm/imported/source-salesforce-native
```

Secrets and bindings aren't imported, see [Existing Pipelines](../../../docs/guides/existing-pipelines.md). A configuration that matches the live endpoint and sets none of its secrets plans no changes, and applies keep the secrets Estuary holds.

A capture created through the dashboard's Salesforce login uses `OAuth Credentials` with the client ID, client secret and refresh token of Estuary's OAuth app, which you can't know. Leave the three out, as in the last example: Terraform then manages the bindings and the other settings, and the sealed credentials stay live. They stay only while the configuration sets no secret and doesn't change `config`. Once it does either, the apply publishes the endpoint with the secrets the configuration sets and no others. To change `config` on such a capture, switch it to `Client Credentials`, or to the `OAuth Credentials` of your own connected app, in the same change.

The connector also accepts the deprecated `Username, Password, & Security Token` credentials, which the resource doesn't have. A capture using them imports with a warning naming `username`, but no configuration can match it, as `credentials_title` only accepts the two methods above. Move the capture to one of them in the dashboard first.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name (e.g., `crm`). The provider `tenant` is added in front and `/source-salesforce-native` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the capture was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the capture. |
| `data_plane` | String | No | Data plane the capture runs in (e.g., `ops/dp/public/example-data-plane`), defaulting to the provider `data_plane` when the capture is created. Estuary can't move a live capture, so setting another data plane than it runs in fails the plan. See [Use Several Data Planes](../../../docs/guides/multiple-environments.md#use-several-data-planes). |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/source-salesforce-native:v1`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `auto_discover` | Object | No | Periodically checks the Salesforce org for new objects and schema changes. See [`auto_discover`](#auto_discover). |
| `config` | Object | Yes | Authentication and capture settings of the Salesforce org. See [`config`](#config). |
| `bindings` | Set of Objects | Yes | Captured Salesforce objects, one per binding. Their order doesn't matter, as Estuary identifies a binding by its resource path, so plans show only the bindings added, removed or changed. See [`bindings`](#bindings). |
| `shards` | Dynamic | No | Shard settings of the capture, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `auto_discover`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |
| `backfill` | Object | No | Backfill control of the whole capture. See [`backfill`](#backfill). |

### `auto_discover`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `add_new_bindings` | Bool | No | Adds a binding for each object found in the Salesforce org that isn't declared, when the capture is created and on each later discovery. Terraform doesn't manage those bindings; declare a binding to manage it. |
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
| `credentials` | Object | Yes | How the connector authenticates with Salesforce. See [`config.credentials`](#configcredentials). |
| `start_date` | String | No | Captures only the records created or changed at or after this UTC time, as `YYYY-MM-DDTHH:MM:SSZ`. Defaults to `1999-02-03T00:00:00Z`, Salesforce's founding date. |
| `is_sandbox` | Bool | No | Whether the org is a Salesforce sandbox. Defaults to `false`. |
| `my_domain` | String | No | My Domain login host of the org, ending in `.my.salesforce.com` (e.g., `acme.my.salesforce.com` or `acme--uat.sandbox.my.salesforce.com`). Required with `Client Credentials`. Unset, the connector logs in through the standard login or test endpoint. |
| `advanced` | Object | No | Options for advanced users, rarely needed. See [`config.advanced`](#configadvanced). |

> [!TIP]
> Give the connector a dedicated read-only Salesforce user. `Client Credentials` needs an external client app with the client credentials flow enabled and that user as its "Run As" user. See [Prerequisites](https://docs.estuary.dev/reference/Connectors/capture-connectors/Salesforce/salesforce-native/#prerequisites).

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `credentials_title` | String | Yes | Authentication method: `Client Credentials` (the consumer key and secret of your connected app or external client app, through the client credentials flow) or `OAuth Credentials` (an OAuth app with a refresh token, as the dashboard login stores). Only the attributes of the selected method are used. |
| `client_id` | String | With `Client Credentials`, or your own app's `OAuth Credentials` | Client ID of the OAuth app the connector authenticates as, which Salesforce calls the consumer key. Leave it out of a capture authenticated through the dashboard login, whose OAuth app is Estuary's. Write-only, see the note above. |
| `client_id_hash` | String | Read-only | Salted hash of `client_id`, used to detect when it changes. |
| `client_secret` | String | With `Client Credentials`, or your own app's `OAuth Credentials` | Client secret of the OAuth app, which Salesforce calls the consumer secret. Leave it out of a capture authenticated through the dashboard login. Write-only, see the note above. |
| `client_secret_hash` | String | Read-only | Salted hash of `client_secret`, used to detect when it changes. |
| `refresh_token` | String | With your own app's `OAuth Credentials` | Refresh token the OAuth app was granted. Leave it out of a capture authenticated through the dashboard login. Write-only, see the note above. |
| `refresh_token_hash` | String | Read-only | Salted hash of `refresh_token`, used to detect when it changes. |
| `instance_url` | String | With `OAuth Credentials` | URL of the Salesforce instance the refresh token belongs to (e.g., `https://acme.my.salesforce.com`). |

### `config.advanced`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `window_size` | Object | No | How much time one incremental query covers when the connector sweeps for changes. Leave it unset unless Estuary support or the connector logs say otherwise. |
| `window_size.window_type` | String | Yes | Unit of the window: `interval` sizes it with `interval`, `days` with `days`. |
| `window_size.interval` | String | With `interval` | Size of the window as an ISO 8601 duration such as `PT1H`. |
| `window_size.days` | Number | With `days` | Size of the window in whole days, at least 1. |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `resource` | Object | Yes | Salesforce object the binding captures. See [`bindings.resource`](#bindingsresource). |
| `target` | String | No | Estuary collection the data is captured into. A binding without it keeps its current collection. A new one gets the collection the connector discovery names, which `terraform plan` runs against the source (e.g., `acmeCo/crm/Account`), see [Bindings](../../../docs/guides/bindings.md). |
| `disable` | Bool | No | Stops capturing the object while keeping the binding in the capture. |
| `backfill` | Object | No | Backfill control of the binding. See [`bindings.backfill`](#bindingsbackfill). |
| `name` | String | Read-only | Collection of the binding, shown in every plan so you can tell which binding changes. |
| `resource_path` | List of String | Read-only | Resource path Estuary identifies the binding by, as in the dashboard. See [Bindings](../../../docs/guides/bindings.md). |

### `bindings.backfill`

Backfill control of the binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Set or raise it to backfill the binding: capture the whole object again from the start. Setting it on a binding Estuary doesn't have yet, when the capture is created, or in the first apply after an import, backfills nothing, while setting it on an existing binding Terraform takes over backfills it. At least 1, can't be lowered, and can't be set or raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |
| `strategy` | String | No | Overrides the capture's `backfill.strategy` for this binding: `incremental` captures again into the existing collection, `dataflowReset` resets the collection instead, backfilling its materializations and truncating their tables. Changing it alone publishes nothing. |

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Name of the captured stream, as the connector discovery and the dashboard list it. |
| `interval` | String | No | Time between two polls of the stream, as an ISO 8601 duration such as `PT5M`. Set it to the interval discovery gives the stream, as the dashboard shows: omitted, the connector polls again right away (`PT0S`). |
| `schedule` | String | No | Cron expression of the scheduled work the connector runs for the stream, in UTC with five fields (e.g., `55 23 * * *`). On Salesforce, it refreshes the formula fields of the object. Omitted, nothing is scheduled. |

> [!TIP]
> Salesforce doesn't report changes to formula fields, so the connector refreshes them on `schedule`, which discovery sets to `55 23 * * *` for the objects that have some. See [Formula Fields](https://docs.estuary.dev/reference/Connectors/capture-connectors/Salesforce/salesforce-native/#formula-fields).

## Data Source

`data "estuary_source_salesforce"` reads every live binding of a capture by name: the declared ones, those discovery (`auto_discover`) adds and those added in the dashboard. Terraform doesn't manage live bindings, so the resource neither stores them nor shows them in plans: read them here, for a capture of this configuration or one managed elsewhere:

```hcl
# A capture of this configuration
data "estuary_source_salesforce" "source" {
  name = estuary_source_salesforce.source.name
}

# A capture managed elsewhere
data "estuary_source_salesforce" "other" {
  name = "crm"
}
```

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name, as in the resource: the provider `tenant` is added in front and `/source-salesforce-native` at the end, unless they're already there. |
| `live_bindings` | List of Objects | Read-only | Every live binding of the capture, with the attributes of [`bindings`](#bindings). |

> [!NOTE]
> See [Bindings](../../../docs/guides/bindings.md) for when the data source is read and how to use it.

