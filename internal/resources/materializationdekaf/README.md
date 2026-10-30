# Dekaf Materializations

[Back to the provider documentation](../../../README.md)

These resources manage [Dekaf](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Dekaf/) materializations. Dekaf is Estuary's Kafka-compatible API: each binding exposes one Estuary collection as a Kafka topic, which Kafka consumers read from Estuary's broker. There is no destination to connect to, so the configuration only holds the token consumers authenticate with and how topics and deletions are exposed.

Estuary offers a dedicated Dekaf connector for each of these consumers, each with its own resource. They are configured the same way, so this page documents them together. Pick the one matching your consumer:

| Resource | Consumer | Connector name |
|---|---|---|
| `estuary_materialization_dekaf_generic` | [Any Kafka consumer](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Dekaf/) | `dekaf-generic` |
| `estuary_materialization_dekaf_bytewax` | [Bytewax](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Dekaf/bytewax/) | `dekaf-bytewax` |
| `estuary_materialization_dekaf_clickhouse` | [ClickHouse](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Dekaf/clickhouse/) through ClickPipes | `dekaf-clickhouse` |
| `estuary_materialization_dekaf_imply_polaris` | [Imply Polaris](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Dekaf/imply-polaris/) | `dekaf-imply-polaris` |
| `estuary_materialization_dekaf_materialize` | [Materialize](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Dekaf/materialize/) | `dekaf-materialize` |
| `estuary_materialization_dekaf_risingwave` | [RisingWave](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Dekaf/risingwave/) | `dekaf-risingwave` |
| `estuary_materialization_dekaf_singlestore` | [SingleStore](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Dekaf/singlestore/) | `dekaf-singlestore` |
| `estuary_materialization_dekaf_startree` | [StarTree](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Dekaf/startree/) | `dekaf-startree` |
| `estuary_materialization_dekaf_tinybird` | [Tinybird](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Dekaf/tinybird/) | `dekaf-tinybird` |

## Table of Contents

- [Example Usage](#example-usage)
- [Connecting Consumers](#connecting-consumers)
- [Attributes](#attributes)
  - [`source`](#source)
  - [`target_naming`](#target_naming)
  - [`config`](#config)
  - [`backfill`](#backfill)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
  - [`bindings.fields`](#bindingsfields)

## Example Usage

Two collections exposed to Tinybird, with deletions emitted as documents so Tinybird can apply them as hard deletes:

```hcl
variable "dekaf_token" {
  type      = string
  sensitive = true
}

resource "estuary_materialization_dekaf_tinybird" "analytics" {
  name = "analytics"

  config = {
    token              = var.dekaf_token
    strict_topic_names = false
    deletions          = "cdc"
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/orders"
      resource = { topic_name = "orders" }
    },
    {
      source   = "acmeCo/mongodb/shop/users"
      resource = { topic_name = "users" }
      fields = {
        recommended = "1"
        exclude     = ["internal_notes"]
      }
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the materialization `acmeCo/analytics/dekaf-tinybird`, the username Tinybird connects with.

Any Kafka consumer, following a capture so each collection it adds is exposed as a topic, and skipping the documents published before 2026 on one topic:

```hcl
resource "estuary_materialization_dekaf_generic" "events" {
  name = "events"

  source = {
    capture = "acmeCo/aurora/source-amazon-aurora-postgres"
  }

  config = {
    token              = var.dekaf_token
    strict_topic_names = true
    deletions          = "kafka"
  }

  bindings = [
    {
      source     = "acmeCo/aurora/public/events"
      not_before = "2026-01-01T00:00:00Z"
      resource   = { topic_name = "events" }
    },
  ]
}
```

> [!NOTE]
> `token` is a [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attribute: Terraform sends it to Estuary but never stores it in the state or shows it in plans. To still tell you when it changes, the provider keeps a read-only salted hash of it (`token_hash`) and warns in the plan when the value no longer matches. Write-only attributes require Terraform 1.11 or later.

## Connecting Consumers

Consumers connect to Estuary with these settings, see each consumer's page linked above for its own steps:

| Setting | Value |
|---|---|
| Broker address | `dekaf.estuary-data.com:9092` |
| Schema registry address | `https://dekaf.estuary-data.com` |
| Security protocol | `SASL_SSL` |
| SASL mechanism | `PLAIN` |
| SASL and schema registry username | Full catalog name of the materialization (e.g., `acmeCo/analytics/dekaf-tinybird`) |
| SASL and schema registry password | `config.token` |
| Topic | `bindings.resource.topic_name` |

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Materialization name (e.g., `analytics`). The provider `tenant` is added in front and the connector name of the resource (e.g., `/dekaf-tinybird`) at the end, unless they're already there. |
| `source` | Object | No | Settings of the bindings Estuary adds, optionally following a capture. See [`source`](#source). |
| `target_naming` | Object | No | How Estuary names the topics of the bindings it adds. See [`target_naming`](#target_naming). |
| `on_incompatible_schema_change` | String | No | What an automated change, such as a capture discovery, does when it's incompatible with a materialized binding: `backfill` (default) backfills the affected bindings, `disableBinding` disables them, `disableTask` disables the materialization, and `abort` stops the automated change. Bindings can override it. See [Schema Evolution](https://docs.estuary.dev/concepts/advanced/evolutions/#what-do-schema-evolutions-do). |
| `config` | Object | Yes | Authentication and topic settings Kafka consumers see. See [`config`](#config). |
| `bindings` | List of Objects | Yes | Materialized collections, one topic per binding. See [`bindings`](#bindings). |
| `backfill` | Object | No | Backfill control of the whole materialization. See [`backfill`](#backfill). |
| `shards` | Dynamic | No | Shard settings of the materialization, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../README.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `source.capture`. Defaults to `true`. A provider control Estuary never sees. See [Bindings Lifecycle](../../../README.md#bindings-lifecycle). |

> [!WARNING]
> Changing `name` so that the resulting catalog name changes destroys the materialization and creates a new one. Changing the provider `tenant` doesn't: Terraform looks for the materialization under the new tenant, plans a new one and leaves the old one in Estuary, no longer managed. Destroy the materialization before changing `tenant`, or delete the old one in Estuary afterwards. Consumers authenticate with that name, so they need the new one too.

> [!WARNING]
> With the default `keep_unmanaged_bindings = true`, Terraform never removes bindings it doesn't manage: those Estuary or the dashboard adds, including the ones kept when you remove `source.capture`, stay in the materialization and aren't shown in plans. Set `keep_unmanaged_bindings = false` to manage every binding and remove the undeclared ones; the plan lists them before the apply.

### `source`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `capture` | String | No | Full name of a capture whose bindings the materialization follows (e.g., `acmeCo/aurora/source-amazon-aurora-postgres`). The provider `tenant` isn't added to it. Each collection added to the capture gets a binding here, and bindings removed from the capture are kept. |
| `delta_updates` | Bool | No | Whether added bindings use delta updates. Defaults to `false`. |
| `fields_recommended` | String | No | `fields.recommended` of added bindings: `"0"`, `"1"`, `"2"`, … or `"unlimited"`, see [`bindings.fields`](#bindingsfields). Defaults to `"unlimited"`. |

> [!TIP]
> With `capture` set, Terraform manages only the bindings in your configuration and keeps the ones Estuary adds, see [Bindings Lifecycle](../../../README.md#bindings-lifecycle). Bindings Estuary adds get only these settings and `target_naming`: declare a binding to give it its own, and the plan warns about it when `capture` is set.

### `target_naming`

Topics have no schema, so only the topic name of the bindings Estuary adds follows `target_naming`.

| Attribute | Type | Required | Description |
|---|---|---|---|
| `strategy` | String | Yes | How added bindings name their topic from their collection (e.g., `acmeCo/aurora/public/orders`): `matchSourceStructure`, `singleSchema` or `prefixTableNames`. See [Target Resource Naming Conventions](https://docs.estuary.dev/concepts/materialization/#target-resource-naming-conventions). |
| `schema` | String | No | Schema of every table, with `singleSchema` and `prefixTableNames`. Not used by Dekaf, whose topics have no schema. |
| `skip_common_defaults` | Bool | No | With `prefixTableNames`, doesn't prefix the common default schemas `public` and `dbo` (`orders` instead of `public_orders`). Defaults to `false`. |
| `schema_template` | String | No | With `matchSourceStructure`, template of the schema name around `{{schema}}`. Not used by Dekaf, whose topics have no schema. |
| `table_template` | String | No | Template of the topic name around `{{table}}` (e.g., `{{table}}_v2`). |

> [!TIP]
> Changing `target_naming` only affects the bindings added afterwards, never the existing ones.

### `config`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `token` | String | Yes | Password Kafka consumers authenticate with, using the full materialization name as username. Write-only, see the note above. |
| `token_hash` | String | Read-only | Salted hash of `token`, used to detect when it changes. |
| `strict_topic_names` | Bool | Yes | Exposes topic names in a strictly Kafka-compliant format, for consumers that require it. The dashboard sets `false`. |
| `deletions` | String | Yes | How deletions reach consumers: `kafka` emits Kafka records with null values, `cdc` emits the deletion document with an `_is_deleted` field, useful for hard deletes. The dashboard sets `kafka`. |

### `backfill`

Backfill control of the whole materialization, as the dashboard's **Backfill** button under **Source Collections → Advanced options**. See [Backfill Control](../../../README.md#backfill-control).

| Attribute | Type | Required | Description |
|---|---|---|---|
| `trigger` | Number | No | Raise it to backfill every enabled binding of the materialization, those Estuary adds through `source.capture` included. Disabled bindings are skipped, as in the dashboard: they keep their counter. **Not Estuary's backfill counter**, but a provider control Estuary never sees: each raise raises each binding's counter by one from its live value. Starts at 1 and only goes up. |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `source` | String | Yes | Estuary collection the binding exposes (e.g., `acmeCo/mongodb/shop/orders`). The provider `tenant` isn't added to it. Changing it replaces the binding, see [Bindings Lifecycle](../../../README.md#bindings-lifecycle). |
| `not_before` | String | No | Exposes only the documents published at or after this RFC 3339 time (e.g., `2026-01-01T00:00:00Z`). It's only a filter: changing it doesn't reprocess the documents already read. See [Time Travel](https://docs.estuary.dev/reference/time-travel/). |
| `not_after` | String | No | Exposes only the documents published before this RFC 3339 time. Like `not_before`, it's only a filter. |
| `resource` | Object | Yes | Topic the collection is exposed as. See [`bindings.resource`](#bindingsresource). |
| `fields` | Object | No | Collection fields included in the topic messages. Omit it to keep the binding's live selection, such as one set in the dashboard: the provider keeps it in state without showing a diff, until you set `fields` yourself. A new binding without `fields` gets Estuary's default, `recommended = "unlimited"` (`recommended: true` in the spec). See [`bindings.fields`](#bindingsfields). |
| `disable` | Bool | No | Stops exposing the collection while keeping the binding. Defaults to `false`. |
| `priority` | Number | No | Processing order among bindings: when priorities differ, all ready documents of a higher-priority binding are processed before any of a lower one. Defaults to `0` and can be negative. See [Processing order](https://docs.estuary.dev/concepts/materialization/#processing-order). |
| `backfill` | Object | No | Backfill control of the binding. See [`bindings.backfill`](#bindingsbackfill). |
| `on_incompatible_schema_change` | String | No | Overrides the materialization's `on_incompatible_schema_change` for this binding: `backfill`, `disableBinding`, `disableTask` or `abort`. |
| `name` | String | Read-only | Collection of the binding, shown in every plan so you can tell which binding changes. |

### `bindings.backfill`

Backfill control of the binding. See [Backfill Control](../../../README.md#backfill-control).

| Attribute | Type | Required | Description |
|---|---|---|---|
| `trigger` | Number | No | Raise it to backfill the binding. **Not Estuary's backfill counter**, but a provider control Estuary never sees: each raise raises Estuary's counter by one from its live value. Starts at 1 and only goes up. Raising it on a disabled binding is an error: set `disable = false` in the same change. |

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `topic_name` | String | Yes | Kafka topic the collection is exposed as, through the Kafka metadata and discovery APIs. Consumers subscribe to it. |

### `bindings.fields`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `recommended` | String | Yes | Depth of the fields selected by default: `"0"` selects only the root document and the required fields, such as the collection key, `"1"` the top-level fields, `"2"` the second-level fields too, and so on, and `"unlimited"` every field. |
| `require` | Set of Strings | No | Fields always included, whatever `recommended` selects. |
| `exclude` | List of Strings | No | Fields never included. |
| `group_by` | List of Strings | No | Fields keying the messages instead of the collection key. Only fields with a scalar type (boolean, integer or string) can be keys, and changing them backfills the binding. See [Group-by keys](https://docs.estuary.dev/guides/customize-materialization-fields/#group-by-keys). |

See [Field Selection](../../../README.md#field-selection) for more examples.
