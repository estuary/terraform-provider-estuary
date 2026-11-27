# estuary_source_stripe

[Back to the provider documentation](../../../README.md)

This resource manages a [Stripe](https://docs.estuary.dev/reference/Connectors/capture-connectors/stripe-realtime/) capture connector. Each binding captures one Stripe resource, such as charges, customers or invoices, into an Estuary collection. The connector backfills each resource, then follows its changes in near real time through the Stripe Events API. A few resources have no events, so the connector backfills them again [on a schedule](https://docs.estuary.dev/reference/Connectors/capture-connectors/stripe-realtime/#streams-requiring-scheduled-backfills).

| Default image | Connector name |
|---|---|
| `ghcr.io/estuary/source-stripe-native:v1` | `source-stripe-native` |

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

A capture of two resources, with auto discovery adding every new one:

```hcl
variable "stripe_api_key" {
  type      = string
  sensitive = true
}

resource "estuary_source_stripe" "payments" {
  name = "payments"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    start_date = "2024-01-01T00:00:00Z"

    credentials = {
      credentials_title = "Private App Credentials"
      access_token      = var.stripe_api_key
    }
  }

  bindings = [
    {
      resource = {
        name     = "Charges"
        interval = "PT5M"
      }
    },
    {
      resource = {
        name     = "Customers"
        interval = "PT5M"
      }
      target = "acmeCo/payments/stripe-customers"
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the capture `acmeCo/payments/source-stripe-native`. The `Charges` binding sets no `target`, so its data lands in the collection the connector discovery names, such as `acmeCo/payments/Charges`.

A platform account capturing its connected accounts on a pinned API version, with a resource backfilled every night:

```hcl
resource "estuary_source_stripe" "platform" {
  name = "payments/platform"

  config = {
    capture_connected_accounts = true

    credentials = {
      credentials_title = "Private App Credentials"
      access_token      = var.stripe_api_key
    }

    advanced = {
      api_version             = "2024-06-20"
      incremental_window_size = "PT1H"
    }
  }

  bindings = [
    {
      resource = {
        name     = "Accounts"
        interval = "PT5M"
        schedule = "0 0 * * *"
      }
    },
  ]
}
```

> [!NOTE]
> `access_token` is a [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attribute: Terraform never stores it in state or shows it in plans, and keeps only a salted hash of it to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Importing

```bash
terraform import estuary_source_stripe.payments acmeCo/payments/source-stripe-native
```

Secrets and bindings aren't imported, see [Existing Pipelines](../../../docs/guides/existing-pipelines.md). A configuration that matches the live endpoint and leaves `access_token` out plans no changes, and applies keep the API key Estuary holds. Once the configuration sets `access_token` or changes `config`, the apply publishes the endpoint with the configured key, so set it in that change.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name (e.g., `payments`). The provider `tenant` is added in front and `/source-stripe-native` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the capture was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the capture. |
| `data_plane` | String | No | Data plane the capture runs in (e.g., `ops/dp/public/example-data-plane`), defaulting to the provider `data_plane` when the capture is created. Estuary can't move a live capture, so setting another data plane than it runs in fails the plan. See [Use Several Data Planes](../../../docs/guides/multiple-environments.md#use-several-data-planes). |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/source-stripe-native:v1`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `auto_discover` | Object | No | Periodically checks the Stripe account for new resources and schema changes. See [`auto_discover`](#auto_discover). |
| `config` | Object | Yes | Authentication and capture settings of the Stripe account. See [`config`](#config). |
| `bindings` | Set of Objects | Yes | Captured Stripe resources, one per binding. Their order doesn't matter, as Estuary identifies a binding by its resource path, so plans show only the bindings added, removed or changed. See [`bindings`](#bindings). |
| `shards` | Dynamic | No | Shard settings of the capture, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `auto_discover`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |
| `backfill` | Object | No | Backfill control of the whole capture. See [`backfill`](#backfill). |

### `auto_discover`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `add_new_bindings` | Bool | No | Adds a binding for each resource found in the Stripe account that isn't declared, when the capture is created and on each later discovery. Terraform doesn't manage those bindings; declare a binding to manage it. |
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
| `credentials` | Object | Yes | How the connector authenticates with Stripe. See [`config.credentials`](#configcredentials). |
| `start_date` | String | No | Captures only the data created at or after this UTC time, as `YYYY-MM-DDTHH:MM:SSZ`. Defaults to 30 days before the capture is created. |
| `capture_connected_accounts` | Bool | No | Also captures the data of the accounts connected to the platform account. Defaults to `false`. See [Connected Accounts](https://docs.estuary.dev/reference/Connectors/capture-connectors/stripe-realtime/#connected-accounts). |
| `advanced` | Object | No | Options for advanced users, rarely needed. See [`config.advanced`](#configadvanced). |

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `credentials_title` | String | Yes | Authentication method, which must be `Private App Credentials`, the only one the connector has. |
| `access_token` | String | To create the capture | Stripe API key, a secret or restricted key (e.g., `sk_live_...`). The connector requires it; an imported capture can leave it out to keep the key Estuary holds. Write-only, see the note above. |
| `access_token_hash` | String | Read-only | Salted hash of `access_token`, used to detect when it changes. |

### `config.advanced`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `api_version` | String | No | Stripe API version of every request (e.g., `2024-06-20`). Defaults to the account's default version. Streams that need a newer version than the one in effect are disabled. See [Stripe API Limitations](https://docs.estuary.dev/reference/Connectors/capture-connectors/stripe-realtime/#stripe-api-limitations). |
| `incremental_window_size` | String | No | Longest time span one incremental sweep processes, as an ISO 8601 duration such as `PT1H`. It bounds the catch-up work done at once after many changes in a short time. |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `resource` | Object | Yes | Stripe resource the binding captures. See [`bindings.resource`](#bindingsresource). |
| `target` | String | No | Estuary collection the data is captured into. A binding without it keeps its current collection. A new one gets the collection the connector discovery names, which `terraform plan` runs against the source (e.g., `acmeCo/payments/Charges`), see [Bindings](../../../docs/guides/bindings.md). |
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
| `schedule` | String | No | Cron expression of the scheduled work the connector runs for the stream, in UTC with five fields (e.g., `55 23 * * *`). On Stripe, it backfills the stream again, which the resources without events need. Omitted, nothing is scheduled. |

## Data Source

`data "estuary_source_stripe"` reads every live binding of a capture by name: the declared ones, those discovery (`auto_discover`) adds and those added in the dashboard. Terraform doesn't manage live bindings, so the resource neither stores them nor shows them in plans: read them here, for a capture of this configuration or one managed elsewhere:

```hcl
# A capture of this configuration
data "estuary_source_stripe" "source" {
  name = estuary_source_stripe.source.name
}

# A capture managed elsewhere
data "estuary_source_stripe" "other" {
  name = "payments"
}
```

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name, as in the resource: the provider `tenant` is added in front and `/source-stripe-native` at the end, unless they're already there. |
| `live_bindings` | List of Objects | Read-only | Every live binding of the capture, with the attributes of [`bindings`](#bindings). |

> [!NOTE]
> See [Bindings](../../../docs/guides/bindings.md) for when the data source is read and how to use it.

