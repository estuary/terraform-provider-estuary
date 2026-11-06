# estuary_materialization_webhook

[Back to the provider documentation](../../../README.md)

This resource manages an [HTTP Webhook](https://docs.estuary.dev/reference/Connectors/materialization-connectors/http-webhook/) materialization connector. Each binding sends the documents of one Estuary collection to a URL of your server: `config.address` joined with the binding's `relative_path`. The connector only uses [delta updates](https://docs.estuary.dev/concepts/materialization/#delta-updates): it sends every change as it comes, without reading anything back from your server.

| Default image | Connector name |
|---|---|
| `ghcr.io/estuary/materialize-webhook:v1` | `materialize-webhook` |

## Table of Contents

- [Example Usage](#example-usage)
- [Requests](#requests)
- [Attributes](#attributes)
  - [`source`](#source)
  - [`target_naming`](#target_naming)
  - [`config`](#config)
  - [`config.headers`](#configheaders)
  - [`config.advanced`](#configadvanced)
  - [`backfill`](#backfill)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
  - [`bindings.fields`](#bindingsfields)

## Example Usage

Two collections sent to an HTTPS endpoint authenticated by a Bearer token, one of them with its top-level fields and the full document:

```hcl
variable "webhook_token" {
  type      = string
  sensitive = true
}

resource "estuary_materialization_webhook" "orders" {
  name = "hooks/orders"

  config = {
    address = "https://hooks.example.com/estuary/"

    headers = {
      custom_headers = [
        { name = "Authorization", value = "Bearer ${var.webhook_token}" },
        { name = "X-Source", value = "estuary" },
      ]
    }
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/orders"
      resource = { relative_path = "orders" }
    },
    {
      source   = "acmeCo/mongodb/shop/customers"
      resource = { relative_path = "crm/customers" }

      fields = {
        recommended = "1"
        require     = ["flow_document"]
      }
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the materialization `acmeCo/hooks/orders/materialize-webhook`, which posts to `https://hooks.example.com/estuary/orders` and `https://hooks.example.com/estuary/crm/customers`.

Every collection of a capture sent to an internal server, starting with the documents published from 2026 on:

```hcl
resource "estuary_materialization_webhook" "events" {
  name = "hooks/events"

  source = {
    capture = "acmeCo/aurora/source-amazon-aurora-postgres"
  }

  config = {
    address = "https://123.4.567.89:3000/webhook/"
  }

  bindings = [
    {
      source     = "acmeCo/aurora/public/events"
      not_before = "2026-01-01T00:00:00Z"
      resource   = { relative_path = "events" }
    },
  ]
}
```

> [!NOTE]
> Header `value`s are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes, as headers usually carry credentials: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

> [!IMPORTANT]
> Send credentials only in headers. The plan rejects credentials in `address` or `relative_path`, such as `user:password@` or a `token` query option, as state and logs would hold them in plain text. Use an `https://` address: over `http://`, headers travel unencrypted.

## Requests

Each request is a `POST` with `Content-Type: application/json` and your custom headers, whose body is a JSON array of documents from one binding. A transaction is sent in requests of about 1 MiB each. A request failing, or answered with a status other than 2xx, is retried with a growing delay, and the materialization fails after 11 attempts.

Each element is a JSON object of the binding's [selected fields](#bindingsfields), keyed by field name (e.g., `[{"id": 1, "status": "shipped", "flow_published_at": "2026-10-23T14:00:00Z"}]`). Nested locations are separate fields named by their path, such as `customer/name`: select fields to a depth of 1 with `fields.recommended = "1"` to send nested objects whole. Locations the collection schema doesn't describe are only sent within the full document, `flow_document`, which isn't selected by default: add it to `fields.require` to include it. See [Payload format](https://docs.estuary.dev/reference/Connectors/materialization-connectors/http-webhook/#payload-format).

> [!WARNING]
> Materializations created before field selection reached this connector (October 30, 2026) send each full document instead, without field selection. Set `advanced.feature_flags = "field_selection"` to switch one to selected fields, which changes the request body your server receives; `no_field_selection` keeps full documents on newer materializations.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Materialization name (e.g., `hooks/orders`). The provider `tenant` is added in front and `/materialize-webhook` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the materialization was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the materialization. |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/materialize-webhook:v1`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `source` | Object | No | Settings of the bindings Estuary adds, optionally following a capture. See [`source`](#source). |
| `target_naming` | Object | No | How Estuary names the paths of the bindings it adds. A change only names the bindings added from then on: existing ones keep their schema and table. See [`target_naming`](#target_naming). |
| `on_incompatible_schema_change` | String | No | What an automated change, such as a capture discovery, does when it's incompatible with a binding: `backfill` (default) backfills the affected bindings, `disableBinding` disables them, `disableTask` disables the materialization, and `abort` stops the automated change. Bindings can override it. See [Schema Evolution](https://docs.estuary.dev/concepts/advanced/evolutions/#what-do-schema-evolutions-do). |
| `config` | Object | Yes | Base URL and headers of the requests. See [`config`](#config). |
| `bindings` | List of Objects | Yes | Materialized collections, one per binding. See [`bindings`](#bindings). |
| `backfill` | Object | No | Backfill control of the whole materialization. See [`backfill`](#backfill). |
| `shards` | Dynamic | No | Shard settings of the materialization, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `source.capture`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |

### `source`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `capture` | String | No | Full name of a capture whose bindings the materialization follows (e.g., `acmeCo/aurora/source-amazon-aurora-postgres`). The provider `tenant` isn't added to it. Each collection added to the capture gets a binding here, and bindings removed from the capture are kept. |
| `delta_updates` | Bool | No | Has no effect: bindings always use delta updates, the only mode of this connector. |
| `fields_recommended` | String | No | `fields.recommended` of added bindings: `"0"`, `"1"`, `"2"`, … or `"unlimited"`, see [`bindings.fields`](#bindingsfields). Defaults to `"unlimited"`. |

> [!TIP]
> Bindings Estuary adds through `capture` get only these settings and [`target_naming`](#target_naming). Declare a binding to give it its own settings, see [Bindings](../../../docs/guides/bindings.md).

### `target_naming`

Bindings Estuary adds get their `relative_path` from their collection name, as other connectors name a table. The bindings of this connector have no schema, so only the table name settings apply.

| Attribute | Type | Required | Description |
|---|---|---|---|
| `strategy` | String | Yes | How added bindings name their path from their collection (e.g., `acmeCo/aurora/public/orders`): `matchSourceStructure`, `singleSchema` or `prefixTableNames`. See [Target Resource Naming Conventions](https://docs.estuary.dev/concepts/materialization/#target-resource-naming-conventions). |
| `schema` | String | No | Schema of every table, with `singleSchema` and `prefixTableNames`. Not used by this connector, whose bindings have no schema. |
| `skip_common_defaults` | Bool | No | With `prefixTableNames`, doesn't prefix the common default schemas `public` and `dbo` (`orders` instead of `public_orders`). Defaults to `false`. |
| `schema_template` | String | No | With `matchSourceStructure`, template of the schema name around `{{schema}}`. Not used by this connector, whose bindings have no schema. |
| `table_template` | String | No | Template of the path around `{{table}}` (e.g., `{{table}}_v2`). |

> [!TIP]
> Changing `target_naming` only affects the bindings added afterwards, never the existing ones.

### `config`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `address` | String | Yes | Base URL the requests are sent to, ending in a trailing `/` (e.g., `https://hooks.example.com/estuary/`). The plan rejects credentials in it, such as `user:password@` or a `token` query option, as state and logs would hold them in plain text: send them in a header instead. Credentials found in the live value show as `***`. |
| `headers` | Object | No | HTTP headers of the requests. See [`config.headers`](#configheaders). |
| `advanced` | Object | No | Options for advanced users, rarely needed. See [`config.advanced`](#configadvanced). |

### `config.headers`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `custom_headers` | List of Objects | No | Additional headers included in every request. |
| `custom_headers.name` | String | Yes | Name of the header (e.g., `Authorization`). It can't be empty. |
| `custom_headers.value` | String | Yes | Value of the header, such as a token. It can't be empty. Write-only, see the note above. |
| `custom_headers.value_hash` | String | Read-only | Salted hash of `value`, used to detect when it changes. |

### `config.advanced`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `feature_flags` | String | No | Connector settings for edge cases and experimental features, such as `field_selection`, see [Requests](#requests) and [Feature Flags](https://docs.estuary.dev/guides/advanced-usage/feature-flags/). Intended for Estuary internal use: change them only as directed by Estuary support. |

### `backfill`

Backfill control of the whole materialization, as the dashboard's **Backfill** button under **Source Collections → Advanced options**. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|---|---|---|---|
| `trigger` | Number | No | Set or raise it to backfill every enabled binding of the materialization, those Estuary adds through `source.capture` included: every document of each collection is sent again. Disabled bindings are skipped. Setting it when the materialization is created, or in the first apply after its import, backfills nothing. At least 1, and can't be lowered. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `source` | String | Yes | Estuary collection the binding materializes (e.g., `acmeCo/mongodb/shop/orders`). The provider `tenant` isn't added to it. Changing it keeps the binding, which Estuary identifies by its resource path, see [Bindings](../../../docs/guides/bindings.md). |
| `not_before` | String | No | Materializes only the documents published at or after this RFC 3339 time (e.g., `2026-01-01T00:00:00Z`). It's only a filter: changing it doesn't reprocess the documents already read. See [Time Travel](https://docs.estuary.dev/reference/time-travel/). |
| `not_after` | String | No | Materializes only the documents published before this RFC 3339 time. Like `not_before`, it's only a filter. |
| `resource` | Object | Yes | URL path the documents are sent to. See [`bindings.resource`](#bindingsresource). |
| `fields` | Object | No | Collection fields sent in each document. Omit it to keep the binding's live selection, such as one set in the dashboard: the provider keeps it in state without showing a diff, until you set `fields` yourself. A new binding without `fields` gets Estuary's default, `recommended = "unlimited"` (`recommended: true` in the spec). See [`bindings.fields`](#bindingsfields). |
| `disable` | Bool | No | Stops materializing the collection while keeping the binding. Defaults to `false`. |
| `priority` | Number | No | Processing order among bindings: when priorities differ, all ready documents of a higher-priority binding are processed before any of a lower one. Defaults to `0` and can be negative, such as `-1` for a binding with a large backlog. See [Processing order](https://docs.estuary.dev/concepts/materialization/#processing-order). |
| `backfill` | Object | No | Backfill control of the binding. See [`bindings.backfill`](#bindingsbackfill). |
| `on_incompatible_schema_change` | String | No | Overrides the materialization's `on_incompatible_schema_change` for this binding: `backfill`, `disableBinding`, `disableTask` or `abort`. |
| `name` | String | Read-only | Collection of the binding, shown in every plan so you can tell which binding changes. |
| `resource_path` | List of String | Read-only | Resource path Estuary identifies the binding by, such as its schema and table, as in the dashboard. See [Bindings](../../../docs/guides/bindings.md). |

### `bindings.backfill`

Backfill control of the binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Set or raise it to backfill the binding: send every document of the collection again. Setting it when the binding is added, or in the first apply after an import, backfills nothing. At least 1, can't be lowered, and can't be set or raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |

> [!IMPORTANT]
> Nothing is cleared on your server before a backfill: the documents it already received are sent again, so make your server handle duplicates, such as by the collection key.

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `relative_path` | String | Yes | Path joined with `config.address` to build the URL the binding's documents are posted to, as a relative URL reference (e.g., `orders` or `/orders`). The plan rejects credentials in it, as in `address`. |

> [!TIP]
> `relative_path` is resolved like a link in a web page: `orders` is appended to `address` (`https://hooks.example.com/estuary/orders`), while `/orders` replaces the path of `address` (`https://hooks.example.com/orders`). The resulting URL must be absolute.

### `bindings.fields`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `recommended` | String | Yes | Depth of the fields selected by default: `"0"` selects only the root document and the required fields, such as the collection key, `"1"` the top-level fields, `"2"` the second-level fields too, and so on, and `"unlimited"` every field. |
| `require` | Set of Strings | No | Fields always sent, whatever `recommended` selects. |
| `exclude` | List of Strings | No | Fields never sent. |
| `group_by` | List of Strings | No | Fields keying the binding instead of the collection key. Only fields with a scalar type (boolean, integer or string) can be keys, and changing them backfills the binding. See [Group-by keys](https://docs.estuary.dev/guides/customize-materialization-fields/#group-by-keys). |

See [Field Selection](../../../docs/guides/bindings.md#field-selection) for examples, and [Requests](#requests) for how the selected fields are sent.
