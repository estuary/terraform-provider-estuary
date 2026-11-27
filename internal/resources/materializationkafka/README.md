# estuary_materialization_kafka

[Back to the provider documentation](../../../README.md)

This resource manages an [Apache Kafka](https://docs.estuary.dev/reference/Connectors/materialization-connectors/apache-kafka/) materialization connector. Each binding materializes one Estuary collection into a Kafka topic, which the connector creates when it doesn't exist. Messages are encoded as Avro, with their schemas in a schema registry, or as JSON. Every binding uses [delta updates](https://docs.estuary.dev/reference/Connectors/materialization-connectors/apache-kafka/#delta-updates): each change of a document is written as a new message, delivered at least once.

| Default image | Connector name |
|---|---|
| `ghcr.io/estuary/materialize-kafka:v1` | `materialize-kafka` |

`config.bootstrap_servers`, `config.tls` and the `user_password` credentials are the same as on [`estuary_source_kafka`](../sourcekafka/README.md), so one cluster is configured the same way on both sides.

## Table of Contents

- [Example Usage](#example-usage)
- [Importing](#importing)
- [Attributes](#attributes)
  - [`source`](#source)
  - [`target_naming`](#target_naming)
  - [`config`](#config)
  - [`config.credentials`](#configcredentials)
  - [`config.schema_registry`](#configschema_registry)
  - [`backfill`](#backfill)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
  - [`bindings.fields`](#bindingsfields)

## Example Usage

A materialization of Avro messages, with their schemas in a Confluent schema registry:

```hcl
variable "kafka_password" {
  type      = string
  sensitive = true
}

variable "schema_registry_secret" {
  type      = string
  sensitive = true
}

resource "estuary_materialization_kafka" "events" {
  name = "events"

  config = {
    bootstrap_servers = "broker-1.example.com:9092,broker-2.example.com:9092"
    tls               = "system_certificates"
    message_format    = "Avro"
    compression       = "zstd"

    credentials = {
      auth_type = "user_password"
      mechanism = "SCRAM-SHA-512"
      username  = "flow_materialize"
      password  = var.kafka_password
    }

    schema_registry = {
      endpoint = "https://psrc-12345.us-east-2.aws.confluent.cloud"
      username = "ABCDEFGHIJKLMNOP"
      password = var.schema_registry_secret
    }
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/orders"
      resource = { topic = "orders" }
    },
    {
      source   = "acmeCo/mongodb/shop/users"
      resource = { topic = "users" }

      fields = {
        recommended = "1"
        exclude     = ["internal_notes"]
      }
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the materialization `acmeCo/events/materialize-kafka`.

A materialization of JSON messages following a capture, so its new collections get a topic as they're added, with the partitions and replication of the topics it creates:

```hcl
resource "estuary_materialization_kafka" "stream" {
  name = "events/stream"

  source = {
    capture = "acmeCo/aurora/source-amazon-aurora-postgres"
  }

  config = {
    bootstrap_servers        = "broker-1.example.com:9092"
    message_format           = "JSON"
    topic_partitions         = 12
    topic_replication_factor = 2

    credentials = {
      auth_type = "user_password"
      mechanism = "PLAIN"
      username  = "flow_materialize"
      password  = var.kafka_password
    }
  }

  bindings = [
    {
      source   = "acmeCo/aurora/public/orders"
      resource = { topic = "orders" }
    },
  ]
}
```

> [!NOTE]
> `password`, of the credentials and of the schema registry, is a [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attribute: Terraform never stores it in state or shows it in plans, and keeps only a salted hash of it to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Importing

```bash
terraform import estuary_materialization_kafka.events acmeCo/events/materialize-kafka
```

Secrets and bindings aren't imported, see [Existing Pipelines](../../../docs/guides/existing-pipelines.md). A configuration that matches the live endpoint and sets none of its secrets plans no changes, and applies keep the secrets Estuary holds. Once the configuration sets a secret or changes `config`, the apply publishes the endpoint with the secrets the configuration sets and no others, so set all of them in that change.

`config.schema_registry` requires its `password`, so a materialization of Avro messages always takes that second path: the first plan after its import shows the two `password_hash` attributes, and the apply publishes the endpoint with the configured passwords and leaves the bindings as they are.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Materialization name (e.g., `events`). The provider `tenant` is added in front and `/materialize-kafka` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the materialization was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the materialization. |
| `data_plane` | String | No | Data plane the materialization runs in (e.g., `ops/dp/public/example-data-plane`), defaulting to the provider `data_plane` when the materialization is created. Estuary can't move a live materialization, so setting another data plane than it runs in fails the plan. See [Use Several Data Planes](../../../docs/guides/multiple-environments.md#use-several-data-planes). |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/materialize-kafka:v1`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `source` | Object | No | Settings of the bindings Estuary adds, optionally following a capture. See [`source`](#source). |
| `target_naming` | Object | No | How Estuary names the topics of the bindings it adds. A change only names the bindings added from then on: existing ones keep their schema and table. See [`target_naming`](#target_naming). |
| `on_incompatible_schema_change` | String | No | What an automated change, such as a capture discovery, does when it's incompatible with a binding: `backfill` (default) backfills the affected bindings, `disableBinding` disables them, `disableTask` disables the materialization, and `abort` stops the automated change. Bindings can override it. See [Schema Evolution](https://docs.estuary.dev/concepts/advanced/evolutions/#what-do-schema-evolutions-do). |
| `config` | Object | Yes | Brokers, authentication and message format of the materialization. See [`config`](#config). |
| `bindings` | Set of Objects | Yes | Materialized collections, one per binding. Their order doesn't matter, as Estuary identifies a binding by its resource path, so plans show only the bindings added, removed or changed. See [`bindings`](#bindings). |
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

Bindings Estuary adds get their `topic` from their collection name, as other connectors name a table. The bindings of this connector have no schema, so only the table name settings apply.

| Attribute | Type | Required | Description |
|---|---|---|---|
| `strategy` | String | Yes | How added bindings name their topic from their collection (e.g., `acmeCo/aurora/public/orders`): `matchSourceStructure`, `singleSchema` or `prefixTableNames`. See [Target Resource Naming Conventions](https://docs.estuary.dev/concepts/materialization/#target-resource-naming-conventions). |
| `schema` | String | No | Schema of every table, with `singleSchema` and `prefixTableNames`. Not used by this connector, whose bindings have no schema. |
| `skip_common_defaults` | Bool | No | With `prefixTableNames`, doesn't prefix the common default schemas `public` and `dbo` (`orders` instead of `public_orders`). Defaults to `false`. |
| `schema_template` | String | No | With `matchSourceStructure`, template of the schema name around `{{schema}}`. Not used by this connector, whose bindings have no schema. |
| `table_template` | String | No | Template of the topic around `{{table}}` (e.g., `{{table}}_v2`). |

> [!TIP]
> Changing `target_naming` only affects the bindings added afterwards, never the existing ones.

### `config`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `bootstrap_servers` | String | Yes | Brokers the connector first connects to, as comma-separated `host:port` pairs (e.g., `broker-1.example.com:9092,broker-2.example.com:9092`). They tell it about the other brokers of the cluster. |
| `tls` | String | No | How the connector finds the TLS certificates it verifies the brokers with: `system_certificates`, the default and only setting, uses the certificate authorities its system trusts. |
| `credentials` | Object | Yes | How the connector authenticates with the cluster. See [`config.credentials`](#configcredentials). |
| `message_format` | String | Yes | Format of the materialized messages: `Avro`, which requires `schema_registry`, or `JSON`, which uses no schema registry. See [Supported Message Formats](https://docs.estuary.dev/reference/Connectors/materialization-connectors/apache-kafka/#supported-message-formats). |
| `schema_registry` | Object | With `Avro` | Schema registry the Avro schemas of the messages are registered in. See [`config.schema_registry`](#configschema_registry). |
| `compression` | String | No | Compression of the messages: `none`, `gzip`, `lz4` (default), `snappy` or `zstd`. Not every broker supports every algorithm. |
| `avro_logical_types` | Bool | No | Registers Avro schemas with logical types, so date-time fields such as `flow_published_at` are timestamps instead of strings. Defaults to `true`. Changing it on an existing materialization changes the type of those fields in its topics, which can break their consumers. |
| `topic_partitions` | Number | No | Number of partitions of the topics the connector creates. Defaults to `6`. |
| `topic_replication_factor` | Number | No | Replication factor of the topics the connector creates. Defaults to `3`. |

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `auth_type` | String | Yes | Authentication method, which must be `user_password` (SASL user and password), the only one the connector has. |
| `mechanism` | String | Yes | SASL mechanism the user and password are exchanged with: `PLAIN`, `SCRAM-SHA-256` or `SCRAM-SHA-512`. |
| `username` | String | Yes | SASL user. |
| `password` | String | To create the materialization | Password of the SASL user. An imported materialization can leave it out to keep the one Estuary holds. Write-only, see the note above. |
| `password_hash` | String | Read-only | Salted hash of `password`, used to detect when it changes. |

### `config.schema_registry`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `endpoint` | String | Yes | URL of the schema registry API (e.g., `https://psrc-12345.us-east-2.aws.confluent.cloud`). The plan rejects credentials in it, such as `user:password@` or a password, token, API key or signature option, as state and logs would hold them in plain text: set them in `username` and `password`. Credentials found in the live value show as `***`. |
| `username` | String | Yes | User of the schema registry, the key of a Confluent Cloud schema registry API key. |
| `password` | String | Yes | Password of the schema registry user, the secret of a Confluent Cloud schema registry API key. Write-only, see the note above. |
| `password_hash` | String | Read-only | Salted hash of `password`, used to detect when it changes. |

### `backfill`

Backfill control of the whole materialization, as the dashboard's **Backfill** button under **Source Collections → Advanced options**. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|---|---|---|---|
| `trigger` | Number | No | Set or raise it to backfill every enabled binding of the materialization, those Estuary adds through `source.capture` included: every document of each collection is written again. Disabled bindings are skipped. Setting it when the materialization is created, or in the first apply after its import, backfills nothing. At least 1, and can't be lowered. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `source` | String | Yes | Estuary collection the binding materializes (e.g., `acmeCo/mongodb/shop/orders`). The provider `tenant` isn't added to it. Changing it keeps the binding, which Estuary identifies by its resource path, see [Bindings](../../../docs/guides/bindings.md). |
| `not_before` | String | No | Materializes only the documents published at or after this RFC 3339 time (e.g., `2026-01-01T00:00:00Z`). It's only a filter: changing it doesn't reprocess the documents already read. See [Time Travel](https://docs.estuary.dev/reference/time-travel/). |
| `not_after` | String | No | Materializes only the documents published before this RFC 3339 time. Like `not_before`, it's only a filter. |
| `resource` | Object | Yes | Kafka topic the collection is materialized into. See [`bindings.resource`](#bindingsresource). |
| `fields` | Object | No | Collection fields written in each message. Omit it to keep the binding's live selection, such as one set in the dashboard: the provider keeps it in state without showing a diff, until you set `fields` yourself. A new binding without `fields` gets Estuary's default, `recommended = "unlimited"` (`recommended: true` in the spec). See [`bindings.fields`](#bindingsfields). |
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
| `trigger` | Number | No | Set or raise it to backfill the binding: write every document of the collection again. Setting it on a binding Estuary doesn't have yet, when the materialization is created, or in the first apply after an import, backfills nothing, while setting it on an existing binding Terraform takes over backfills it. At least 1, can't be lowered, and can't be set or raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |

> [!IMPORTANT]
> Nothing is removed from the topic before a backfill: the messages it already holds stay, and every document is written again, so make consumers handle duplicates, such as by the collection key.

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `topic` | String | Yes | Kafka topic to materialize into, which the connector creates when it doesn't exist. Changing it on an existing binding moves it to a new topic, which the connector backfills from the start of the collection, leaving the old topic in place. |

### `bindings.fields`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `recommended` | String | Yes | Depth of the fields selected by default: `"0"` selects only the root document and the required fields, such as the collection key, `"1"` the top-level fields, `"2"` the second-level fields too, and so on, and `"unlimited"` every field. |
| `require` | Set of Strings | No | Fields always written, whatever `recommended` selects. |
| `exclude` | List of Strings | No | Fields never written. |
| `group_by` | List of Strings | No | Fields keying the messages instead of the collection key. Only fields with a scalar type (boolean, integer or string) can be keys, and changing them backfills the binding. See [Group-by keys](https://docs.estuary.dev/guides/customize-materialization-fields/#group-by-keys). |

See [Field Selection](../../../docs/guides/bindings.md#field-selection) for examples.

