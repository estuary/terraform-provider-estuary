# estuary_source_facebook_marketing

[Back to the provider documentation](../../../README.md)

This resource manages a [Facebook Marketing](https://docs.estuary.dev/reference/Connectors/capture-connectors/facebook-marketing-native/) capture connector. Each binding captures one Facebook Marketing resource, such as campaigns, ad sets, ads, ad creatives or an insights report, into an Estuary collection, by polling the Marketing API.

| Default image | Connector name |
|---|---|
| `ghcr.io/estuary/source-facebook-marketing-native:v1` | `source-facebook-marketing-native` |

## Table of Contents

- [Example Usage](#example-usage)
- [Importing](#importing)
- [Attributes](#attributes)
  - [`auto_discover`](#auto_discover)
  - [`backfill`](#backfill)
  - [`config`](#config)
  - [`config.credentials`](#configcredentials)
  - [`config.custom_insights`](#configcustom_insights)
  - [`config.advanced`](#configadvanced)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
- [Data Source](#data-source)

## Example Usage

A capture of two ad accounts with the long-lived access token of your own Meta app, and a custom insights report captured as its own stream:

```hcl
variable "meta_app_id" {
  type      = string
  sensitive = true
}

variable "meta_app_secret" {
  type      = string
  sensitive = true
}

variable "meta_access_token" {
  type      = string
  sensitive = true
}

resource "estuary_source_facebook_marketing" "ads" {
  name = "ads"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    account_ids              = "123456789,987654321"
    start_date               = "2024-01-01T00:00:00Z"
    insights_lookback_window = 28

    custom_insights = [
      {
        name              = "spend_by_age"
        level             = "campaign"
        fields            = "account_id,campaign_id,impressions,clicks,spend"
        breakdowns        = "age,gender"
        action_breakdowns = "action_type"
      },
    ]

    credentials = {
      credentials_title = "OAuth Credentials"
      client_id         = var.meta_app_id
      client_secret     = var.meta_app_secret
      access_token      = var.meta_access_token
    }
  }

  bindings = [
    {
      resource = {
        name     = "campaigns"
        interval = "PT1H"
      }
    },
    {
      resource = {
        name     = "spend_by_age"
        interval = "PT1H"
      }
      target = "acmeCo/ads/spend-by-age"
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the capture `acmeCo/ads/source-facebook-marketing-native`. The `campaigns` binding sets no `target`, so its data lands in the collection the connector discovery names, such as `acmeCo/ads/campaigns`.

A capture created through the dashboard's Facebook login, imported with its secrets left out, see [Importing](#importing):

```hcl
resource "estuary_source_facebook_marketing" "imported" {
  name = "ads/imported"

  config = {
    account_ids = "123456789"
    start_date  = "2024-01-01T00:00:00Z"

    credentials = {
      credentials_title = "OAuth Credentials"
    }
  }

  bindings = []
}
```

> [!NOTE]
> `client_id`, `client_secret` and `access_token` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Importing

```bash
terraform import estuary_source_facebook_marketing.imported acmeCo/ads/imported/source-facebook-marketing-native
```

Secrets and bindings aren't imported, see [Existing Pipelines](../../../docs/guides/existing-pipelines.md). A configuration that matches the live endpoint and sets none of its secrets plans no changes, and applies keep the secrets Estuary holds.

A capture created through the dashboard's Facebook login holds the app ID, app secret and access token of Estuary's Meta app, which you can't know. Leave the three out, as in the last example: Terraform then manages the bindings and the other settings, and the sealed credentials stay live. They stay only while the configuration sets no secret and doesn't change `config`. Once it does either, the apply publishes the endpoint with the secrets the configuration sets and no others. To change `config` on such a capture, such as its `account_ids` or `custom_insights`, set the credentials of your own Meta app in the same change.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name (e.g., `ads`). The provider `tenant` is added in front and `/source-facebook-marketing-native` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the capture was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the capture. |
| `data_plane` | String | No | Data plane the capture runs in (e.g., `ops/dp/public/example-data-plane`), defaulting to the provider `data_plane` when the capture is created. Estuary can't move a live capture, so setting another data plane than it runs in fails the plan. See [Use Several Data Planes](../../../docs/guides/multiple-environments.md#use-several-data-planes). |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/source-facebook-marketing-native:v1`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `auto_discover` | Object | No | Periodically checks the ad accounts for new resources and schema changes. See [`auto_discover`](#auto_discover). |
| `config` | Object | Yes | Ad accounts, authentication and insights settings of the capture. See [`config`](#config). |
| `bindings` | Set of Objects | Yes | Captured Facebook Marketing resources, one per binding. Their order doesn't matter, as Estuary identifies a binding by its resource path, so plans show only the bindings added, removed or changed. See [`bindings`](#bindings). |
| `shards` | Dynamic | No | Shard settings of the capture, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `auto_discover`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |
| `backfill` | Object | No | Backfill control of the whole capture. See [`backfill`](#backfill). |

### `auto_discover`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `add_new_bindings` | Bool | No | Adds a binding for each resource found in the ad accounts that isn't declared, when the capture is created and on each later discovery. Terraform doesn't manage those bindings; declare a binding to manage it. |
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
| `account_ids` | String | Yes | Comma-separated IDs of the Facebook ad accounts to capture (e.g., `123456789,987654321`). |
| `start_date` | String | No | Captures only the data at or after this UTC time, as `YYYY-MM-DDTHH:MM:SSZ`. |
| `insights_lookback_window` | Number | No | Days of past insights the connector captures again, at least 1, as Facebook keeps changing insights for that long. Set it to the attribution window of the Facebook account when it has a custom one. Defaults to `28`. |
| `custom_insights` | List of Objects | No | Insights reports captured besides the built-in ones, each as a stream of its own. See [`config.custom_insights`](#configcustom_insights). |
| `advanced` | Object | No | Options for advanced users, rarely needed. See [`config.advanced`](#configadvanced). |
| `credentials` | Object | Yes | How the connector authenticates with Facebook. See [`config.credentials`](#configcredentials). |

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `credentials_title` | String | Yes | Authentication method, which must be `OAuth Credentials`, the only one the connector has. |
| `client_id` | String | With your own Meta app | App ID of the Meta app the access token belongs to. Leave it out of a capture authenticated through the dashboard login, whose app is Estuary's. Write-only, see the note above. |
| `client_id_hash` | String | Read-only | Salted hash of `client_id`, used to detect when it changes. |
| `client_secret` | String | With your own Meta app | App secret of the Meta app. Leave it out of a capture authenticated through the dashboard login. Write-only, see the note above. |
| `client_secret_hash` | String | Read-only | Salted hash of `client_secret`, used to detect when it changes. |
| `access_token` | String | With your own Meta app | Long-lived access token the connector sends to Facebook, such as a system user's or one of your own Meta app. Leave it out of a capture authenticated through the dashboard login. Write-only, see the note above. |
| `access_token_hash` | String | Read-only | Salted hash of `access_token`, used to detect when it changes. |

> [!TIP]
> The connector sends only `access_token` to Facebook, but it requires `client_id` and `client_secret` too: set them to the ID and secret of the app the token was issued for. See [Configuring Manually with an Access Token](https://docs.estuary.dev/reference/Connectors/capture-connectors/facebook-marketing-native/#configuring-manually-with-an-access-token) for the app, the permissions and the token.

### `config.custom_insights`

Each report becomes a stream named `name`, which discovery lists like the built-in ones: declare a binding with that `name` to capture it.

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Name of the report, which names its stream. |
| `level` | String | No | Level the report aggregates at: `ad` (default), `adset`, `campaign` or `account`. |
| `fields` | String | Yes | Comma-separated fields of the report (e.g., `account_id,ad_id,impressions,clicks,spend`). |
| `breakdowns` | String | Yes | Comma-separated breakdowns of the report (e.g., `age,gender,region`), empty for none. |
| `action_breakdowns` | String | No | Comma-separated action breakdowns of the report (e.g., `action_type`). |
| `start_date` | String | No | Captures only the report data at or after this UTC time, as `YYYY-MM-DDTHH:MM:SSZ`. |
| `insights_lookback_window` | Number | No | Days of past report data the connector captures again, at least 1, as Facebook keeps changing insights for that long. Defaults to `28`. |

### `config.advanced`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `include_deleted` | Bool | No | Also captures the deleted entities of the resources that support it. Defaults to `false`. |
| `fetch_thumbnail_images` | Bool | No | Fetches the `thumbnail_url` of each ad creative and stores the image in `thumbnail_data_url`. Defaults to `false`. |
| `feature_flags` | String | No | Comma-separated experimental feature flags to enable. Change them only as directed by Estuary support. |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `resource` | Object | Yes | Facebook Marketing resource the binding captures. See [`bindings.resource`](#bindingsresource). |
| `target` | String | No | Estuary collection the data is captured into. A binding without it keeps its current collection. A new one gets the collection the connector discovery names, which `terraform plan` runs against the source (e.g., `acmeCo/ads/campaigns`), see [Bindings](../../../docs/guides/bindings.md). |
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

`data "estuary_source_facebook_marketing"` reads every live binding of a capture by name: the declared ones, those discovery (`auto_discover`) adds and those added in the dashboard. Terraform doesn't manage live bindings, so the resource neither stores them nor shows them in plans: read them here, for a capture of this configuration or one managed elsewhere:

```hcl
# A capture of this configuration
data "estuary_source_facebook_marketing" "source" {
  name = estuary_source_facebook_marketing.source.name
}

# A capture managed elsewhere
data "estuary_source_facebook_marketing" "other" {
  name = "ads"
}
```

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name, as in the resource: the provider `tenant` is added in front and `/source-facebook-marketing-native` at the end, unless they're already there. |
| `live_bindings` | List of Objects | Read-only | Every live binding of the capture, with the attributes of [`bindings`](#bindings). |

> [!NOTE]
> See [Bindings](../../../docs/guides/bindings.md) for when the data source is read and how to use it.

