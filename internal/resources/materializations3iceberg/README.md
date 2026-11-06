# estuary_materialization_s3_iceberg

[Back to the provider documentation](../../../README.md)

This resource manages an [Apache Iceberg Tables in Amazon S3 (delta updates)](https://docs.estuary.dev/reference/Connectors/materialization-connectors/amazon-s3-iceberg/) materialization connector. Each binding materializes one Estuary collection into an Iceberg table of an AWS Glue or Iceberg REST catalog, with data files in your S3 bucket. The connector only uses [delta updates](https://docs.estuary.dev/concepts/materialization/#delta-updates): it appends the changes of each transaction as Parquet files, without merging them into one row per key, and needs no compute of its own. To merge changes into tables instead, use [`estuary_materialization_iceberg`](../materializationiceberg/README.md).

| Default image | Connector name |
|---|---|
| `ghcr.io/estuary/materialize-s3-iceberg:v2` | `materialize-s3-iceberg` |

## Table of Contents

- [Example Usage](#example-usage)
- [Attributes](#attributes)
  - [`source`](#source)
  - [`target_naming`](#target_naming)
  - [`config`](#config)
  - [`config.credentials`](#configcredentials)
  - [`config.catalog`](#configcatalog)
  - [`config.advanced`](#configadvanced)
  - [`backfill`](#backfill)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
  - [`bindings.fields`](#bindingsfields)

## Example Usage

An AWS Glue catalog of another AWS account, authenticated with an IAM role, uploading files every 15 minutes:

```hcl
resource "estuary_materialization_s3_iceberg" "events" {
  name = "lake/events"

  config = {
    bucket          = "acme-lake"
    prefix          = "iceberg"
    region          = "us-east-1"
    namespace       = "events"
    upload_interval = "PT15M"

    credentials = {
      auth_type    = "AWSIAM"
      aws_role_arn = "arn:aws:iam::123456789012:role/estuary-s3-iceberg"
      aws_region   = "us-east-1"
    }

    catalog = {
      catalog_type = "AWS Glue"
      glue_id      = "123456789013"
    }
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/page_views"
      resource = { table = "page_views" }
    },
    {
      source = "acmeCo/mongodb/shop/orders"
      resource = {
        table     = "orders"
        namespace = "sales"

        additional_table_properties = {
          "write.parquet.compression-codec" = "zstd"
        }
      }

      fields = {
        recommended = "1"
        exclude     = ["internal_notes"]
      }
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the materialization `acmeCo/lake/events/materialize-s3-iceberg`.

An Iceberg REST catalog, such as Apache Polaris, authenticated with an access key, following a capture so its new collections are materialized as they're added, with Iceberg format v3 nanosecond timestamps and variant columns:

```hcl
variable "aws_secret_access_key" {
  type      = string
  sensitive = true
}

variable "catalog_credential" {
  type      = string
  sensitive = true
}

resource "estuary_materialization_s3_iceberg" "changes" {
  name = "lake/changes"

  source = {
    capture = "acmeCo/aurora/source-amazon-aurora-postgres"
  }

  config = {
    bucket          = "acme-lake"
    region          = "us-east-1"
    namespace       = "changes"
    upload_interval = "PT5M"

    credentials = {
      auth_type             = "AWSAccessKey"
      aws_access_key_id     = "AKIAXXXXXXXXXXXXXXXX"
      aws_secret_access_key = var.aws_secret_access_key
    }

    catalog = {
      catalog_type = "Iceberg REST Server"
      uri          = "https://123456789.snowflakecomputing.com/polaris/api/catalog"
      warehouse    = "lake_catalog"
      credential   = var.catalog_credential
      scope        = "PRINCIPAL_ROLE:ALL"
    }

    advanced = {
      nanosecond_timestamps = true
      variant_columns       = true
    }
  }

  bindings = [
    {
      source     = "acmeCo/aurora/public/orders"
      not_before = "2026-01-01T00:00:00Z"
      resource   = { table = "orders" }
    },
  ]
}
```

S3-compatible storage, such as MinIO, with a REST catalog authenticated by a Bearer token:

```hcl
variable "catalog_token" {
  type      = string
  sensitive = true
}

resource "estuary_materialization_s3_iceberg" "minio" {
  name = "lake/minio"

  config = {
    bucket          = "lake"
    region          = "us-east-1"
    namespace       = "raw"
    upload_interval = "PT1H"
    s3_endpoint     = "https://123.4.567.89:9000"

    credentials = {
      auth_type             = "AWSAccessKey"
      aws_access_key_id     = "AKIAXXXXXXXXXXXXXXXX"
      aws_secret_access_key = var.aws_secret_access_key
    }

    catalog = {
      catalog_type = "Iceberg REST Server"
      uri          = "https://123.4.567.89:8181/catalog"
      warehouse    = "lake"
      token        = var.catalog_token
    }
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/events"
      resource = { table = "events" }
      backfill = { trigger = 1 }
    },
  ]
}
```

> [!NOTE]
> `aws_secret_access_key`, `credential` and `token` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

> [!IMPORTANT]
> Tables keep a row per change, so documents read again, such as by an incremental backfill of the capture, are appended again as duplicates. A [backfill](#backfill) of the materialization drops each table first, so it doesn't duplicate rows.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Materialization name (e.g., `lake/events`). The provider `tenant` is added in front and `/materialize-s3-iceberg` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the materialization was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the materialization. |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/materialize-s3-iceberg:v2`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `source` | Object | No | Settings of the bindings Estuary adds, optionally following a capture. See [`source`](#source). |
| `target_naming` | Object | No | How Estuary names the tables of the bindings it adds. A change only names the bindings added from then on: existing ones keep their schema and table. See [`target_naming`](#target_naming). |
| `on_incompatible_schema_change` | String | No | What an automated change, such as a capture discovery, does when it's incompatible with a materialized table: `backfill` (default) backfills the affected bindings, `disableBinding` disables them, `disableTask` disables the materialization, and `abort` stops the automated change. Bindings can override it. See [Schema Evolution](https://docs.estuary.dev/concepts/advanced/evolutions/#what-do-schema-evolutions-do). |
| `config` | Object | Yes | Bucket, authentication, catalog and default namespace of the materialization. See [`config`](#config). |
| `bindings` | List of Objects | Yes | Materialized collections, one per binding. See [`bindings`](#bindings). |
| `backfill` | Object | No | Backfill control of the whole materialization. See [`backfill`](#backfill). |
| `shards` | Dynamic | No | Shard settings of the materialization, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `source.capture`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |

### `source`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `capture` | String | No | Full name of a capture whose bindings the materialization follows (e.g., `acmeCo/aurora/source-amazon-aurora-postgres`). The provider `tenant` isn't added to it. Each collection added to the capture gets a binding here, and bindings removed from the capture are kept. |
| `delta_updates` | Bool | No | Has no effect: added bindings always use delta updates, the only mode of this connector. |
| `fields_recommended` | String | No | `fields.recommended` of added bindings: `"0"`, `"1"`, `"2"`, … or `"unlimited"`, see [`bindings.fields`](#bindingsfields). Defaults to `"unlimited"`. |

> [!TIP]
> Bindings Estuary adds through `capture` get only these settings and [`target_naming`](#target_naming). Declare a binding to give it its own settings, see [Bindings](../../../docs/guides/bindings.md).

### `target_naming`

The bindings of this connector have no schema, so Estuary can't set the namespace of the tables it adds: they go to `config.namespace`. Only the table name follows `target_naming`.

| Attribute | Type | Required | Description |
|---|---|---|---|
| `strategy` | String | Yes | How added bindings name their table from their collection (e.g., `acmeCo/aurora/public/orders`): `matchSourceStructure`, `singleSchema` or `prefixTableNames`. See [Target Resource Naming Conventions](https://docs.estuary.dev/concepts/materialization/#target-resource-naming-conventions). |
| `schema` | String | No | Schema of every table, with `singleSchema` and `prefixTableNames`. Not used by this connector, whose bindings have no schema. |
| `skip_common_defaults` | Bool | No | With `prefixTableNames`, doesn't prefix the common default schemas `public` and `dbo` (`orders` instead of `public_orders`). Defaults to `false`. |
| `schema_template` | String | No | With `matchSourceStructure`, template of the schema name around `{{schema}}`. Not used by this connector, whose bindings have no schema. |
| `table_template` | String | No | Template of the table name around `{{table}}` (e.g., `{{table}}_v2`). |

> [!TIP]
> Changing `target_naming` only affects the bindings added afterwards, never the existing ones.

### `config`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `bucket` | String | Yes | S3 bucket the data files are written to. |
| `prefix` | String | No | Prefix of the data files in `bucket`. |
| `region` | String | Yes | AWS region of `bucket` (e.g., `us-east-1`). |
| `namespace` | String | Yes | Namespace of the materialized tables, unless a binding sets its own. It can't contain dots. |
| `upload_interval` | String | No | How often data files are uploaded and appended to the tables, as an ISO 8601 duration of at most 4 hours (e.g., `PT5M`, the dashboard default). Set it: the connector's validation fails without a value. |
| `s3_endpoint` | String | No | S3 endpoint URL for S3-compatible storage, which is then addressed path-style. Defaults to the AWS S3 endpoint of `region`. The plan rejects credentials in it, such as `user:password@` or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as `***`. |
| `credentials` | Object | No | How the connector authenticates with AWS. See [`config.credentials`](#configcredentials). Set it, or the deprecated `aws_access_key_id` and `aws_secret_access_key`, for the materialization to connect. |
| `catalog` | Object | Yes | Iceberg catalog the tables are registered in. See [`config.catalog`](#configcatalog). |
| `advanced` | Object | No | Options for advanced users, rarely needed. See [`config.advanced`](#configadvanced). |
| `aws_access_key_id` | String | No | Deprecated form of `credentials` with `auth_type = "AWSAccessKey"`, still accepted for existing materializations. Ignored when `credentials` is set. |
| `aws_secret_access_key` | String | No | Deprecated form of `credentials` with `auth_type = "AWSAccessKey"`, still accepted for existing materializations. Ignored when `credentials` is set. Write-only, see the note above. |
| `aws_secret_access_key_hash` | String | Read-only | Salted hash of `aws_secret_access_key`, used to detect when it changes. |

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `auth_type` | String | Yes | Authentication method: `AWSAccessKey` or `AWSIAM`. Only the attributes of the selected method are used. |
| `aws_access_key_id` | String | With `AWSAccessKey` | AWS access key ID of the IAM user the connector authenticates as. |
| `aws_secret_access_key` | String | With `AWSAccessKey` | AWS secret access key of the IAM user. Write-only, see the note above. |
| `aws_secret_access_key_hash` | String | Read-only | Salted hash of `aws_secret_access_key`, used to detect when it changes. |
| `aws_role_arn` | String | With `AWSIAM` | ARN of the IAM role Estuary assumes to connect, see the [AWS IAM guide](https://docs.estuary.dev/guides/iam-auth/aws/). |
| `aws_region` | String | With `AWSIAM` | AWS region to authenticate in (e.g., `us-east-1`). |

> [!TIP]
> With `AWSAccessKey`, the connector writes the data files with its own keys. With `AWSIAM` and a REST catalog, it uses the S3 credentials the catalog vends.

### `config.catalog`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `catalog_type` | String | Yes | Catalog the tables are registered in: `Iceberg REST Server` or `AWS Glue`. Only the attributes of the selected catalog are used. |
| `glue_id` | String | No | ID of the Glue catalog, with `AWS Glue`, for cross-account access. Defaults to the account ID of the credentials. |
| `uri` | String | With `Iceberg REST Server` | URL of the REST catalog (e.g., `https://<server>/api/catalog`). The plan rejects credentials in it, such as `user:password@` or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as `***`. |
| `warehouse` | String | With `Iceberg REST Server` | Warehouse to connect to in the REST catalog. |
| `credential` | String | No | Credential for the REST catalog, such as `<client_id>:<client_secret>`, with `Iceberg REST Server`. Write-only, see the note above. |
| `credential_hash` | String | Read-only | Salted hash of `credential`, used to detect when it changes. |
| `token` | String | No | Bearer token for the REST catalog, with `Iceberg REST Server`. Write-only, see the note above. |
| `token_hash` | String | Read-only | Salted hash of `token`, used to detect when it changes. |
| `scope` | String | No | Scope of the requested security token, with `Iceberg REST Server` (e.g., `PRINCIPAL_ROLE:ALL`). |

### `config.advanced`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `nanosecond_timestamps` | Bool | No | Stores date-time columns with nanosecond precision (Iceberg format v3) instead of microseconds (v2). Changing it applies to new data only: existing rows read as null in converted columns unless the binding is backfilled. Defaults to `false`. |
| `variant_columns` | Bool | No | Stores object, array and multi-type fields and the root document as Iceberg variant columns (format v3) instead of JSON strings. Changing it applies to new data only: existing rows read as null in converted columns unless the binding is backfilled. Defaults to `false`. |
| `feature_flags` | String | No | Connector settings for edge cases and experimental features, see [Feature Flags](https://docs.estuary.dev/guides/advanced-usage/feature-flags/). Intended for Estuary internal use: change them only as directed by Estuary support. |

> [!TIP]
> Reading format v3 tables, with nanosecond timestamps or variant columns, needs a query engine that supports Iceberg format v3.

### `backfill`

Backfill control of the whole materialization, as the dashboard's **Backfill** button under **Source Collections → Advanced options**. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|---|---|---|---|
| `trigger` | Number | No | Set or raise it to backfill every enabled binding of the materialization, those Estuary adds through `source.capture` included: each table is dropped, created again and materialized again from its collection. Disabled bindings are skipped. Setting it when the materialization is created, or in the first apply after its import, backfills nothing. At least 1, and can't be lowered. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `source` | String | Yes | Estuary collection the binding materializes (e.g., `acmeCo/mongodb/shop/orders`). The provider `tenant` isn't added to it. Changing it keeps the binding, which Estuary identifies by its resource path, see [Bindings](../../../docs/guides/bindings.md). |
| `not_before` | String | No | Materializes only the documents published at or after this RFC 3339 time (e.g., `2026-01-01T00:00:00Z`). It's only a filter: changing it doesn't reprocess the documents already read. See [Time Travel](https://docs.estuary.dev/reference/time-travel/). |
| `not_after` | String | No | Materializes only the documents published before this RFC 3339 time. Like `not_before`, it's only a filter. |
| `resource` | Object | Yes | Iceberg table the collection is materialized into. See [`bindings.resource`](#bindingsresource). |
| `fields` | Object | No | Collection fields materialized as columns. Omit it to keep the binding's live selection, such as one set in the dashboard: the provider keeps it in state without showing a diff, until you set `fields` yourself. A new binding without `fields` gets Estuary's default, `recommended = "unlimited"` (`recommended: true` in the spec). See [`bindings.fields`](#bindingsfields). |
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
| `trigger` | Number | No | Set or raise it to backfill the binding: drop the table, create it again and materialize the whole collection again. Setting it when the binding is added, or in the first apply after an import, backfills nothing. At least 1, can't be lowered, and can't be set or raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `table` | String | Yes | Iceberg table to materialize into, created when missing. The name is lowercased and can't contain dots. Changing it on an existing binding moves it to a new table, which the connector backfills from the start of the collection, leaving the old table in place. |
| `namespace` | String | No | Namespace of the table, overriding `config.namespace`. It's lowercased and can't contain dots. |
| `delta_updates` | Bool | No | Appends the changes of each transaction instead of merging them. The connector only supports delta updates, so it rejects `false`. Defaults to `true`, so you can leave it out. |
| `additional_table_properties` | Map of Strings | No | Iceberg table properties set when the table is created (e.g., `{ "write.parquet.compression-codec" = "zstd" }`). They can't be changed afterwards. |

> [!TIP]
> Iceberg tables can't be truncated, so a backfill drops the table and creates it again, which also applies `additional_table_properties` again. See [Schema changes during backfill](https://docs.estuary.dev/reference/backfilling-data/#schema-changes-during-backfill).

### `bindings.fields`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `recommended` | String | Yes | Depth of the fields selected by default: `"0"` selects only the root document and the required fields, such as the collection key, `"1"` the top-level fields, `"2"` the second-level fields too, and so on, and `"unlimited"` every field. |
| `require` | Set of Strings | No | Fields always materialized, whatever `recommended` selects. |
| `exclude` | List of Strings | No | Fields never materialized. |
| `group_by` | List of Strings | No | Fields keying the table instead of the collection key. Only fields with a scalar type (boolean, integer or string) can be keys, and changing them backfills the binding. See [Group-by keys](https://docs.estuary.dev/guides/customize-materialization-fields/#group-by-keys). |

See [Field Selection](../../../docs/guides/bindings.md#field-selection) for examples and how `flow_document` is selected.
