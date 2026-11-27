# estuary_source_kafka

[Back to the provider documentation](../../../README.md)

This resource manages an [Apache Kafka](https://docs.estuary.dev/reference/Connectors/capture-connectors/apache-kafka/) capture connector. Each binding captures the messages of one Kafka topic into an Estuary collection. The connector decodes Avro and Protobuf messages through a schema registry, and JSON messages with or without one. See [Supported Message Formats](https://docs.estuary.dev/reference/Connectors/capture-connectors/apache-kafka/#supported-message-formats).

| Default image | Connector name |
|---|---|
| `ghcr.io/estuary/source-kafka:v1` | `source-kafka` |

`config.bootstrap_servers`, `config.tls` and the `user_password` credentials are the same as on [`estuary_materialization_kafka`](../materializationkafka/README.md), so one cluster is configured the same way on both sides.

## Table of Contents

- [Example Usage](#example-usage)
- [Importing](#importing)
- [Attributes](#attributes)
  - [`auto_discover`](#auto_discover)
  - [`backfill`](#backfill)
  - [`config`](#config)
  - [`config.credentials`](#configcredentials)
  - [`config.schema_registry`](#configschema_registry)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
- [Data Source](#data-source)

## Example Usage

A capture authenticating with a SASL user, decoding messages through a Confluent schema registry, with auto discovery adding every new topic:

```hcl
variable "kafka_password" {
  type      = string
  sensitive = true
}

variable "schema_registry_secret" {
  type      = string
  sensitive = true
}

resource "estuary_source_kafka" "events" {
  name = "events"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    bootstrap_servers = "broker-1.example.com:9092,broker-2.example.com:9092"
    tls               = "system_certificates"

    credentials = {
      auth_type = "user_password"
      mechanism = "SCRAM-SHA-512"
      username  = "flow_capture"
      password  = var.kafka_password
    }

    schema_registry = {
      schema_registry_type = "confluent_schema_registry"
      endpoint             = "https://psrc-12345.us-east-2.aws.confluent.cloud"
      username             = "ABCDEFGHIJKLMNOP"
      password             = var.schema_registry_secret
    }
  }

  bindings = [
    {
      resource = { topic = "orders" }
    },
    {
      resource = { topic = "payments" }
      target   = "acmeCo/events/kafka-payments"
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the capture `acmeCo/events/source-kafka`. The `orders` binding sets no `target`, so its data lands in the collection the connector discovery names, such as `acmeCo/events/orders`.

A capture of an Amazon MSK cluster authenticating through IAM, with JSON messages and no schema registry:

```hcl
variable "aws_secret_access_key" {
  type      = string
  sensitive = true
}

resource "estuary_source_kafka" "msk" {
  name = "events/msk"

  config = {
    bootstrap_servers = "b-1.acme.abc123.c2.kafka.us-east-1.amazonaws.com:9098"

    credentials = {
      auth_type             = "AWS"
      aws_access_key_id     = "AKIAIOSFODNN7EXAMPLE"
      aws_secret_access_key = var.aws_secret_access_key
      region                = "us-east-1"
    }

    schema_registry = {
      schema_registry_type = "no_schema_registry"
      enable_json_only     = true
    }
  }

  bindings = [
    {
      resource = { topic = "clicks" }
    },
  ]
}
```

> [!NOTE]
> `password` and `aws_secret_access_key` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Importing

```bash
terraform import estuary_source_kafka.events acmeCo/events/source-kafka
```

Secrets and bindings aren't imported, see [Existing Pipelines](../../../docs/guides/existing-pipelines.md). A configuration that matches the live endpoint and sets none of its secrets plans no changes, and applies keep the secrets Estuary holds. Once the configuration sets a secret or changes `config`, the apply publishes the endpoint with the secrets the configuration sets and no others, so set all of them in that change.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name (e.g., `events`). The provider `tenant` is added in front and `/source-kafka` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the capture was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the capture. |
| `data_plane` | String | No | Data plane the capture runs in (e.g., `ops/dp/public/example-data-plane`), defaulting to the provider `data_plane` when the capture is created. Estuary can't move a live capture, so setting another data plane than it runs in fails the plan. See [Use Several Data Planes](../../../docs/guides/multiple-environments.md#use-several-data-planes). |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/source-kafka:v1`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `auto_discover` | Object | No | Periodically checks the cluster for new topics and schema changes. See [`auto_discover`](#auto_discover). |
| `config` | Object | Yes | Brokers, authentication and schema registry of the cluster. See [`config`](#config). |
| `bindings` | Set of Objects | Yes | Captured Kafka topics, one per binding. Their order doesn't matter, as Estuary identifies a binding by its resource path, so plans show only the bindings added, removed or changed. See [`bindings`](#bindings). |
| `shards` | Dynamic | No | Shard settings of the capture, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `auto_discover`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |
| `backfill` | Object | No | Backfill control of the whole capture. See [`backfill`](#backfill). |

### `auto_discover`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `add_new_bindings` | Bool | No | Adds a binding for each topic found in the cluster that isn't declared, when the capture is created and on each later discovery. Terraform doesn't manage those bindings; declare a binding to manage it. |
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
| `bootstrap_servers` | String | Yes | Brokers the connector first connects to, as comma-separated `host:port` pairs (e.g., `broker-1.example.com:9092,broker-2.example.com:9092`). They tell it about the other brokers of the cluster. |
| `tls` | String | No | How the connector finds the TLS certificates it verifies the brokers with: `system_certificates`, the default and only setting, uses the certificate authorities its system trusts. |
| `credentials` | Object | Yes | How the connector authenticates with the cluster. See [`config.credentials`](#configcredentials). |
| `schema_registry` | Object | Yes | Schema registry the messages are decoded with, or none. See [`config.schema_registry`](#configschema_registry). |

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `auth_type` | String | Yes | Authentication method: `user_password` (SASL user and password) or `AWS` (IAM access key of an Amazon MSK cluster). Only the attributes of the selected method are used. |
| `mechanism` | String | With `user_password` | SASL mechanism the user and password are exchanged with: `PLAIN`, `SCRAM-SHA-256` or `SCRAM-SHA-512`. |
| `username` | String | With `user_password` | SASL user. |
| `password` | String | With `user_password` | Password of the SASL user. Write-only, see the note above. |
| `password_hash` | String | Read-only | Salted hash of `password`, used to detect when it changes. |
| `aws_access_key_id` | String | With `AWS` | AWS access key ID of the IAM user the connector authenticates as. |
| `aws_secret_access_key` | String | With `AWS` | AWS secret access key of the IAM user. Write-only, see the note above. |
| `aws_secret_access_key_hash` | String | Read-only | Salted hash of `aws_secret_access_key`, used to detect when it changes. |
| `region` | String | With `AWS` | AWS region of the MSK cluster (e.g., `us-east-1`). |

> [!TIP]
> See [Authentication and Connection Security](https://docs.estuary.dev/reference/Connectors/capture-connectors/apache-kafka/#authentication-and-connection-security) for the SASL mechanisms, and [AWS Managed Streaming Kafka (MSK)](https://docs.estuary.dev/reference/Connectors/capture-connectors/apache-kafka/#aws-managed-streaming-kafka-msk) for the IAM policy an MSK cluster needs.

### `config.schema_registry`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `schema_registry_type` | String | Yes | Whether messages are decoded through a schema registry: `confluent_schema_registry` uses the registry at `endpoint`, `no_schema_registry` uses none. Only the attributes of the selected type are used. |
| `endpoint` | String | With `confluent_schema_registry` | URL of the schema registry API (e.g., `https://psrc-12345.us-east-2.aws.confluent.cloud`). The plan rejects credentials in it, such as `user:password@` or a password, token, API key or signature option, as state and logs would hold them in plain text: set them in `username` and `password`. Credentials found in the live value show as `***`. |
| `username` | String | With `confluent_schema_registry` | User of the schema registry, the key of a Confluent Cloud schema registry API key. |
| `password` | String | With `confluent_schema_registry` | Password of the schema registry user, the secret of a Confluent Cloud schema registry API key. Write-only, see the note above. |
| `password_hash` | String | Read-only | Salted hash of `password`, used to detect when it changes. |
| `enable_json_only` | Bool | With `no_schema_registry` | Confirms that every message is captured as JSON, which `no_schema_registry` requires to be `true`. Every topic is then discovered, with collections keyed by message partition and offset. |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `resource` | Object | Yes | Kafka topic the binding captures. See [`bindings.resource`](#bindingsresource). |
| `target` | String | No | Estuary collection the data is captured into. A binding without it keeps its current collection. A new one gets the collection the connector discovery names, which `terraform plan` runs against the source (e.g., `acmeCo/events/orders`), see [Bindings](../../../docs/guides/bindings.md). |
| `disable` | Bool | No | Stops capturing the topic while keeping the binding in the capture. |
| `backfill` | Object | No | Backfill control of the binding. See [`bindings.backfill`](#bindingsbackfill). |
| `name` | String | Read-only | Collection of the binding, shown in every plan so you can tell which binding changes. |
| `resource_path` | List of String | Read-only | Resource path Estuary identifies the binding by, as in the dashboard. See [Bindings](../../../docs/guides/bindings.md). |

### `bindings.backfill`

Backfill control of the binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Set or raise it to backfill the binding: capture the whole topic again from the start. Setting it on a binding Estuary doesn't have yet, when the capture is created, or in the first apply after an import, backfills nothing, while setting it on an existing binding Terraform takes over backfills it. At least 1, can't be lowered, and can't be set or raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |
| `strategy` | String | No | Overrides the capture's `backfill.strategy` for this binding: `incremental` captures again into the existing collection, `dataflowReset` resets the collection instead, backfilling its materializations and truncating their tables. Changing it alone publishes nothing. |

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `topic` | String | Yes | Kafka topic the binding captures messages from. |

## Data Source

`data "estuary_source_kafka"` reads every live binding of a capture by name: the declared ones, those discovery (`auto_discover`) adds and those added in the dashboard. Terraform doesn't manage live bindings, so the resource neither stores them nor shows them in plans: read them here, for a capture of this configuration or one managed elsewhere:

```hcl
# A capture of this configuration
data "estuary_source_kafka" "source" {
  name = estuary_source_kafka.source.name
}

# A capture managed elsewhere
data "estuary_source_kafka" "other" {
  name = "events"
}
```

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name, as in the resource: the provider `tenant` is added in front and `/source-kafka` at the end, unless they're already there. |
| `live_bindings` | List of Objects | Read-only | Every live binding of the capture, with the attributes of [`bindings`](#bindings). |

> [!NOTE]
> See [Bindings](../../../docs/guides/bindings.md) for when the data source is read and how to use it.

