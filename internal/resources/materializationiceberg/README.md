# estuary_materialization_iceberg

[Back to the provider documentation](../../../README.md)

This resource manages an [Apache Iceberg](https://docs.estuary.dev/reference/Connectors/materialization-connectors/apache-iceberg/) materialization connector. Each binding materializes one Estuary collection into an Iceberg table of a REST catalog, such as AWS Glue, S3 Tables or Snowflake Open Catalog. The connector stages the data in S3 and runs Spark jobs on your AWS EMR Serverless application to merge it into the tables, one row per key. To append changes to tables instead, use [`estuary_materialization_s3_iceberg`](../materializations3iceberg/README.md).

| Default image | Connector name |
|---|---|
| `ghcr.io/estuary/materialize-iceberg:v1` | `materialize-iceberg` |

## Table of Contents

- [Example Usage](#example-usage)
- [Attributes](#attributes)
  - [`source`](#source)
  - [`target_naming`](#target_naming)
  - [`config`](#config)
  - [`config.credentials`](#configcredentials)
  - [`config.compute`](#configcompute)
  - [`config.compute.credentials`](#configcomputecredentials)
  - [`config.compute.spark_job_properties`](#configcomputespark_job_properties)
  - [`config.sync_schedule`](#configsync_schedule)
  - [`config.dbt_job_trigger`](#configdbt_job_trigger)
  - [`config.advanced`](#configadvanced)
  - [`config.glue_optimizers`](#configglue_optimizers)
  - [`config.catalog_authentication`](#configcatalog_authentication)
  - [`backfill`](#backfill)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
  - [`bindings.fields`](#bindingsfields)

## Example Usage

An AWS Glue catalog authenticated with an IAM role that the EMR jobs reuse, compacting the tables with the Glue optimizers and committing every 5 minutes during business hours:

```hcl
resource "estuary_materialization_iceberg" "lake" {
  name = "lake"

  config = {
    url           = "https://glue.us-east-1.amazonaws.com/iceberg"
    warehouse     = "123456789012"
    namespace     = "analytics"
    base_location = "s3://acme-lake/iceberg/"

    credentials = {
      auth_type    = "AWS IAM"
      aws_role_arn = "arn:aws:iam::123456789012:role/estuary-iceberg"
      aws_region   = "us-east-1"
    }

    compute = {
      compute_type       = "AWS EMR Serverless"
      region             = "us-east-1"
      application_id     = "00abcdefghijklmn"
      execution_role_arn = "arn:aws:iam::123456789012:role/estuary-emr-execution"
      bucket             = "acme-iceberg-staging"
      bucket_path        = "staging"

      credentials = {
        auth_type = "UseCatalogAuth"
      }
    }

    sync_schedule = {
      sync_frequency         = "5m"
      timezone               = "America/New_York"
      fast_sync_start_time   = "09:00"
      fast_sync_stop_time    = "17:00"
      fast_sync_enabled_days = "M-F"
    }

    glue_optimizers = {
      execution_role_arn      = "arn:aws:iam::123456789012:role/estuary-glue-optimizer"
      enable_compaction       = true
      enable_retention        = true
      snapshot_retention_days = 7
    }
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/users"
      resource = { table = "users" }
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

With `tenant = "acmeCo/"` set on the provider, this creates the materialization `acmeCo/lake/materialize-iceberg`.

A Snowflake Open Catalog (Apache Polaris) authenticated with OAuth client credentials, following a capture so its new collections are materialized as they're added, with each source schema as namespace. Snowflake resolves unquoted identifiers as upper case, so tables are created upper case. The EMR jobs authenticate with an access key and read the catalog credential from AWS Systems Manager:

```hcl
variable "catalog_credential" {
  type      = string
  sensitive = true
}

variable "emr_secret_access_key" {
  type      = string
  sensitive = true
}

resource "estuary_materialization_iceberg" "open_catalog" {
  name = "snowflake/open-catalog"

  source = {
    capture            = "acmeCo/aurora/source-amazon-aurora-postgres"
    fields_recommended = "1"
  }

  target_naming = {
    strategy = "matchSourceStructure"
  }

  config = {
    url         = "https://123456789.snowflakecomputing.com/polaris/api/catalog"
    warehouse   = "analytics_catalog"
    namespace   = "estuary"
    hard_delete = true

    credentials = {
      auth_type         = "OAuth 2.0 Client Credentials"
      oauth2_server_uri = "v1/oauth/tokens"
      credential        = var.catalog_credential
      scope             = "PRINCIPAL_ROLE:estuary"
    }

    compute = {
      compute_type           = "AWS EMR Serverless"
      region                 = "us-east-1"
      application_id         = "00abcdefghijklmn"
      execution_role_arn     = "arn:aws:iam::123456789012:role/estuary-emr-execution"
      bucket                 = "acme-iceberg-staging"
      systems_manager_prefix = "/estuary/"

      credentials = {
        auth_type             = "AWSAccessKey"
        aws_access_key_id     = "AKIAXXXXXXXXXXXXXXXX"
        aws_secret_access_key = var.emr_secret_access_key
      }
    }

    advanced = {
      table_identifier_case = "uppercase"
      field_name_case       = "uppercase"
    }
  }

  bindings = [
    {
      source = "acmeCo/aurora/public/orders"
      resource = {
        table     = "orders"
        namespace = "public"
      }
    },
  ]
}
```

An S3 table bucket reached through the S3 Tables REST endpoint with AWS SigV4, with lowercase columns for Athena, more Spark executors for the merge jobs, and a dbt Cloud job triggered when new data is materialized:

```hcl
variable "catalog_secret_access_key" {
  type      = string
  sensitive = true
}

variable "dbt_api_key" {
  type      = string
  sensitive = true
}

resource "estuary_materialization_iceberg" "table_bucket" {
  name = "s3tables"

  config = {
    url       = "https://s3tables.us-east-1.amazonaws.com/iceberg"
    warehouse = "arn:aws:s3tables:us-east-1:123456789012:bucket/acme-tables"
    namespace = "analytics"

    credentials = {
      auth_type             = "AWS SigV4"
      aws_access_key_id     = "AKIAXXXXXXXXXXXXXXXX"
      aws_secret_access_key = var.catalog_secret_access_key
      aws_region            = "us-east-1"
    }

    compute = {
      compute_type       = "AWS EMR Serverless"
      region             = "us-east-1"
      application_id     = "00abcdefghijklmn"
      execution_role_arn = "arn:aws:iam::123456789012:role/estuary-emr-execution"
      bucket             = "acme-iceberg-staging"

      credentials = {
        auth_type = "UseCatalogAuth"
      }

      spark_job_properties = [
        {
          key   = "spark.dynamicAllocation.maxExecutors"
          value = "20"
        },
      ]
    }

    dbt_job_trigger = {
      access_url = "https://cloud.getdbt.com"
      account_id = "67890"
      job_id     = "12345"
      api_key    = var.dbt_api_key
    }

    advanced = {
      field_name_case = "lowercase"
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
> `aws_secret_access_key`, `credential` and `api_key` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

> [!IMPORTANT]
> The connector needs an EMR Serverless application with the Spark runtime, an execution role for its jobs, and an S3 bucket to stage data files, besides the catalog. See the [connector prerequisites](https://docs.estuary.dev/reference/Connectors/materialization-connectors/apache-iceberg/#prerequisites) for the setup and the IAM policies.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Materialization name (e.g., `lake`). The provider `tenant` is added in front and `/materialize-iceberg` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the materialization was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the materialization. |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/materialize-iceberg:v1`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `source` | Object | No | Settings of the bindings Estuary adds, optionally following a capture. See [`source`](#source). |
| `target_naming` | Object | No | How Estuary names the namespaces and tables of the bindings it adds. A change only names the bindings added from then on: existing ones keep their schema and table. See [`target_naming`](#target_naming). |
| `on_incompatible_schema_change` | String | No | What an automated change, such as a capture discovery, does when it's incompatible with a materialized table: `backfill` (default) backfills the affected bindings, `disableBinding` disables them, `disableTask` disables the materialization, and `abort` stops the automated change. Bindings can override it. See [Schema Evolution](https://docs.estuary.dev/concepts/advanced/evolutions/#what-do-schema-evolutions-do). |
| `config` | Object | Yes | Catalog, compute and default namespace of the materialization. See [`config`](#config). |
| `bindings` | List of Objects | Yes | Materialized collections, one per binding. See [`bindings`](#bindings). |
| `backfill` | Object | No | Backfill control of the whole materialization. See [`backfill`](#backfill). |
| `shards` | Dynamic | No | Shard settings of the materialization, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `source.capture`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |

### `source`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `capture` | String | No | Full name of a capture whose bindings the materialization follows (e.g., `acmeCo/aurora/source-amazon-aurora-postgres`). The provider `tenant` isn't added to it. Each collection added to the capture gets a binding here, and bindings removed from the capture are kept. |
| `delta_updates` | Bool | No | Has no effect: this connector always merges changes into one row per key. |
| `fields_recommended` | String | No | `fields.recommended` of added bindings: `"0"`, `"1"`, `"2"`, … or `"unlimited"`, see [`bindings.fields`](#bindingsfields). Defaults to `"unlimited"`. |

> [!TIP]
> Bindings Estuary adds through `capture` get only these settings and [`target_naming`](#target_naming). Declare a binding to give it its own settings, see [Bindings](../../../docs/guides/bindings.md).

### `target_naming`

The namespace of a binding plays the role of its schema.

| Attribute | Type | Required | Description |
|---|---|---|---|
| `strategy` | String | Yes | How added bindings name their namespace and table from their collection (e.g., `acmeCo/aurora/public/orders`): `matchSourceStructure` keeps the source schema as namespace and the table (`public.orders`), `singleSchema` puts every table in the namespace `schema` (`<schema>.orders`), and `prefixTableNames` puts every table in the namespace `schema` with the source schema as table prefix (`<schema>.public_orders`). |
| `schema` | String | No | Namespace of every table, with `singleSchema` and `prefixTableNames`. |
| `skip_common_defaults` | Bool | No | With `prefixTableNames`, doesn't prefix the common default schemas `public` and `dbo` (`orders` instead of `public_orders`). Defaults to `false`. |
| `schema_template` | String | No | With `matchSourceStructure`, template of the namespace name around `{{schema}}` (e.g., `raw_{{schema}}`). |
| `table_template` | String | No | Template of the table name around `{{table}}` (e.g., `{{table}}_v2`). |

> [!TIP]
> Changing `target_naming` only affects the bindings added afterwards, never the existing ones. See [Target Resource Naming Conventions](https://docs.estuary.dev/concepts/materialization/#target-resource-naming-conventions). The resulting names are then cased by `config.advanced.table_identifier_case`.

### `config`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `url` | String | Yes | Base URL of the Iceberg REST catalog: `https://glue.<region>.amazonaws.com/iceberg` for AWS Glue, `https://s3tables.<region>.amazonaws.com/iceberg` for S3 Tables, or the URL of another REST catalog. The plan rejects credentials in it, such as `user:password@` or a password or token option, as state and logs would hold them in plain text. Credentials found in the live value show as `***`. |
| `warehouse` | String | Yes | Warehouse to connect to: the AWS account ID without hyphens for AWS Glue (e.g., `123456789012`), or the table bucket ARN for S3 Tables (e.g., `arn:aws:s3tables:us-east-1:123456789012:bucket/<bucket>`). |
| `namespace` | String | Yes | Namespace of the materialized tables, unless a binding sets its own. It can't contain dots. |
| `base_location` | String | No | S3 location of the table data and metadata (e.g., `s3://<bucket>/<prefix>/`). Required with AWS Glue, optional with other catalogs. |
| `hard_delete` | Bool | No | Deletes from the table the documents deleted in the source. Otherwise, the `_meta/op` column marks deleted rows (soft delete). Defaults to `false`. |
| `credentials` | Object | No | How the connector authenticates with the catalog. See [`config.credentials`](#configcredentials). Set it, or the deprecated `catalog_authentication`, for the materialization to connect. |
| `compute` | Object | Yes | EMR Serverless application that merges the staged data into the tables. See [`config.compute`](#configcompute). |
| `sync_schedule` | Object | No | When and how often transactions run. See [`config.sync_schedule`](#configsync_schedule). |
| `dbt_job_trigger` | Object | No | Triggers a dbt Cloud job when new data is materialized. See [`config.dbt_job_trigger`](#configdbt_job_trigger). |
| `advanced` | Object | No | Options for advanced users, rarely needed. See [`config.advanced`](#configadvanced). |
| `glue_optimizers` | Object | No | AWS Glue managed table optimizers, registered on each table. Only with AWS Glue as the catalog. See [`config.glue_optimizers`](#configglue_optimizers). |
| `catalog_authentication` | Object | No | Deprecated form of `credentials`, still accepted for existing materializations. Ignored when `credentials` is set. See [`config.catalog_authentication`](#configcatalog_authentication). |

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `auth_type` | String | Yes | Authentication method: `OAuth 2.0 Client Credentials`, `AWS SigV4` or `AWS IAM`. AWS Glue and S3 Tables catalogs need `AWS SigV4` or `AWS IAM`. Only the attributes of the selected method are used. |
| `oauth2_server_uri` | String | With `OAuth 2.0 Client Credentials` | OAuth 2.0 token endpoint: a path relative to `url`, `v1/oauth/tokens` (the dashboard default) for most catalogs, or a full http(s) URL when the token endpoint is on another host (e.g., `https://login.dremio.cloud/oauth/token`). The connector rejects an empty value. The plan rejects credentials in it, such as `user:password@` or a password or token option, as state and logs would hold them in plain text. Credentials found in the live value show as `***`. |
| `credential` | String | With `OAuth 2.0 Client Credentials` | Catalog credential: `<client_id>:<client_secret>` for OAuth client credentials, or `<token>` for Bearer authentication. Write-only, see the note above. |
| `credential_hash` | String | Read-only | Salted hash of `credential`, used to detect when it changes. |
| `scope` | String | No | Authorization scope requested with `OAuth 2.0 Client Credentials` (e.g., `PRINCIPAL_ROLE:<role>`). |
| `aws_access_key_id` | String | With `AWS SigV4` | AWS access key ID of the IAM user the connector authenticates as. |
| `aws_secret_access_key` | String | With `AWS SigV4` | AWS secret access key of the IAM user. Write-only, see the note above. |
| `aws_secret_access_key_hash` | String | Read-only | Salted hash of `aws_secret_access_key`, used to detect when it changes. |
| `aws_region` | String | With `AWS SigV4` and `AWS IAM` | AWS region of the catalog (e.g., `us-east-1`). |
| `aws_role_arn` | String | With `AWS IAM` | ARN of the IAM role Estuary assumes to connect, see the [AWS IAM guide](https://docs.estuary.dev/guides/iam-auth/aws/). |

### `config.compute`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `compute_type` | String | Yes | Compute backend running the merge jobs. Only `AWS EMR Serverless` is supported. |
| `region` | String | Yes | AWS region of the EMR Serverless application and the staging bucket (e.g., `us-east-1`). |
| `application_id` | String | Yes | ID of the EMR Serverless application, which needs the Spark runtime. |
| `execution_role_arn` | String | Yes | ARN of the IAM role the EMR Serverless jobs run as. |
| `bucket` | String | Yes | S3 bucket where data files are staged before they're merged into tables. |
| `bucket_path` | String | No | Prefix of the staged data files in `bucket`. |
| `systems_manager_prefix` | String | No | AWS Systems Manager path, starting and ending with `/` (e.g., `/estuary/`), where the connector stores the catalog credential for the EMR jobs. Required with `OAuth 2.0 Client Credentials` catalog authentication. |
| `credentials` | Object | No | How the connector authenticates with EMR and the staging bucket. See [`config.compute.credentials`](#configcomputecredentials). Set it, or the deprecated `aws_access_key_id` and `aws_secret_access_key`, for the materialization to connect. |
| `spark_job_properties` | List of Objects | No | Spark properties overriding the EMR Serverless defaults of the merge jobs. See [`config.compute.spark_job_properties`](#configcomputespark_job_properties). |
| `aws_access_key_id` | String | No | Deprecated form of `credentials` with `auth_type = "AWSAccessKey"`, still accepted for existing materializations. |
| `aws_secret_access_key` | String | No | Deprecated form of `credentials` with `auth_type = "AWSAccessKey"`, still accepted for existing materializations. Write-only, see the note above. |
| `aws_secret_access_key_hash` | String | Read-only | Salted hash of `aws_secret_access_key`, used to detect when it changes. |

### `config.compute.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `auth_type` | String | Yes | Authentication method: `AWSAccessKey`, or `UseCatalogAuth` to reuse the catalog credentials, which then must use `AWS SigV4` or `AWS IAM`. Only the attributes of the selected method are used. |
| `aws_access_key_id` | String | With `AWSAccessKey` | AWS access key ID of the IAM user that runs the EMR jobs and writes to the staging bucket. |
| `aws_secret_access_key` | String | With `AWSAccessKey` | AWS secret access key of the IAM user. Write-only, see the note above. |
| `aws_secret_access_key_hash` | String | Read-only | Salted hash of `aws_secret_access_key`, used to detect when it changes. |

### `config.compute.spark_job_properties`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `key` | String | Yes | Spark property to override: `spark.executor.instances`, `spark.dynamicAllocation.initialExecutors` or `spark.dynamicAllocation.maxExecutors`. |
| `value` | String | Yes | Value of the property, letters and digits only (e.g., `8`). |

See the [EMR Serverless Spark defaults](https://docs.aws.amazon.com/emr/latest/EMR-Serverless-UserGuide/jobs-spark.html#spark-defaults) for the values these properties override.

### `config.sync_schedule`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `sync_frequency` | String | No | How often transactions run once the materialization is caught up: `0s`, `30s`, `1m`, `2m30s`, `5m`, `15m`, `20m`, `30m`, `1h`, `2h` or `4h`. It applies all day, every day, unless the fast sync window and days limit it. Defaults to `30m`. |
| `timezone` | String | No | Timezone of the fast sync window and days, as an IANA name (e.g., `America/New_York`) or a `+HH:MM` offset. |
| `fast_sync_start_time` | String | No | Time of day, as `HH:MM` (e.g., `09:00`), when transactions start running at `sync_frequency`. Before it they run every 4 hours. |
| `fast_sync_stop_time` | String | No | Time of day, as `HH:MM` (e.g., `17:00`), when transactions stop running at `sync_frequency`. After it they run every 4 hours. |
| `fast_sync_enabled_days` | String | No | Days when `sync_frequency` applies, such as `M-F`, `M,W,F` or `Su-T,Th-S`. On other days transactions run every 4 hours. Defaults to every day. |

> [!TIP]
> The schedule only applies once the materialization is caught up: backfills run as fast as possible. Committed data is merged by EMR jobs, so a lower `sync_frequency` gives fresher tables but runs more jobs. See [Materialization Sync Schedule](https://docs.estuary.dev/reference/materialization-sync-schedule/).

### `config.dbt_job_trigger`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `job_id` | String | Yes | ID of the dbt job to trigger. |
| `account_id` | String | Yes | ID of the dbt account. |
| `api_key` | String | Yes | dbt API key used to trigger the job. Write-only, see the note above. |
| `api_key_hash` | String | Read-only | Salted hash of `api_key`, used to detect when it changes. |
| `access_url` | String | No | Access URL of the dbt account, found in its Account Settings (e.g., `https://cloud.getdbt.com`). It must start with `https://`. The job is only triggered when it's set. The plan rejects credentials in it, such as `user:password@` or a password or token option, as state and logs would hold them in plain text. Credentials found in the live value show as `***`. |
| `cause` | String | No | Message shown as the cause of each triggered run. Defaults to `Estuary`. |
| `mode` | String | No | What happens when the job is already running: `skip` doesn't trigger it (default), `replace` cancels the running job and triggers a new one, and `ignore` triggers a new one anyway. |
| `interval` | String | No | Minimum time between two triggers, which only happen after new data is materialized. Defaults to `30m`. |

### `config.advanced`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `table_identifier_case` | String | No | Casing of the namespace and table names of created tables: `lowercase` (default), `uppercase`, for catalogs resolving unquoted identifiers as upper case such as Snowflake's, or `preserve`. Existing tables are never renamed. |
| `field_name_case` | String | No | Casing of the names of created columns: `preserve` (default), `lowercase`, needed to query S3 Table Buckets with Athena, or `uppercase`. Existing columns are matched case-insensitively and never renamed. |
| `feature_flags` | String | No | Connector settings for edge cases and experimental features, see [Feature Flags](https://docs.estuary.dev/guides/advanced-usage/feature-flags/). Intended for Estuary internal use: change them only as directed by Estuary support. |
| `lowercase_column_names` | Bool | No | Deprecated form of `field_name_case = "lowercase"`, still accepted for existing materializations. It conflicts with `field_name_case` `uppercase` or `preserve`. |

### `config.glue_optimizers`

Only with AWS Glue as the catalog. See [Glue Table Optimizers](https://docs.estuary.dev/reference/Connectors/materialization-connectors/apache-iceberg/#glue-table-optimizers).

| Attribute | Type | Required | Description |
|---|---|---|---|
| `execution_role_arn` | String | No | ARN of the IAM role Glue assumes to optimize tables. It needs read and write access to the table data in S3 and access to the Glue catalog. Required when any optimizer is enabled. |
| `enable_compaction` | Bool | No | Compacts small data files to speed up queries. Defaults to `false`. |
| `enable_retention` | Bool | No | Removes old table snapshots to cut storage costs. Defaults to `false`. |
| `snapshot_retention_days` | Number | No | Days snapshots are kept, with `enable_retention`. Defaults to Glue's `5`. |
| `number_of_snapshots_to_retain` | Number | No | Snapshots always kept, whatever their age, with `enable_retention`. Defaults to Glue's `1`. |
| `enable_orphan_file_deletion` | Bool | No | Deletes files no table snapshot references. Defaults to `false`. |
| `orphan_file_retention_days` | Number | No | Days orphan files are kept before deletion, with `enable_orphan_file_deletion`. Defaults to Glue's `3`. |

### `config.catalog_authentication`

Deprecated form of [`config.credentials`](#configcredentials), still accepted for existing materializations. Use `credentials` for new ones.

| Attribute | Type | Required | Description |
|---|---|---|---|
| `catalog_auth_type` | String | Yes | Authentication method: `OAuth 2.0 Client Credentials` or `AWS SigV4`. Only the attributes of the selected method are used. |
| `oauth2_server_uri` | String | With `OAuth 2.0 Client Credentials` | OAuth 2.0 token endpoint: a path relative to `url`, `v1/oauth/tokens` for most catalogs, or a full http(s) URL. The connector rejects an empty value. The plan rejects credentials in it, such as `user:password@` or a password or token option, as state and logs would hold them in plain text. Credentials found in the live value show as `***`. |
| `credential` | String | With `OAuth 2.0 Client Credentials` | Catalog credential: `<client_id>:<client_secret>`, or `<token>` for Bearer authentication. Write-only, see the note above. |
| `credential_hash` | String | Read-only | Salted hash of `credential`, used to detect when it changes. |
| `scope` | String | No | Authorization scope requested with `OAuth 2.0 Client Credentials`. |
| `aws_access_key_id` | String | With `AWS SigV4` | AWS access key ID of the IAM user the connector authenticates as. |
| `aws_secret_access_key` | String | With `AWS SigV4` | AWS secret access key of the IAM user. Write-only, see the note above. |
| `aws_secret_access_key_hash` | String | Read-only | Salted hash of `aws_secret_access_key`, used to detect when it changes. |
| `region` | String | With `AWS SigV4` | AWS region of the catalog (e.g., `us-east-1`). |
| `signing_name` | String | No | Ignored by the connector, which derives the SigV4 signing name from `url`. Kept so existing materializations still decode. |

### `backfill`

Backfill control of the whole materialization, as the dashboard's **Backfill** button under **Source Collections → Advanced options**. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|---|---|---|---|
| `trigger` | Number | No | Raise it to backfill every enabled binding of the materialization, those Estuary adds through `source.capture` included: each table is dropped, created again and materialized again from its collection. Disabled bindings are skipped. Starts at 1 and only goes up. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `source` | String | Yes | Estuary collection the binding materializes (e.g., `acmeCo/mongodb/shop/orders`). The provider `tenant` isn't added to it. Changing it replaces the binding, see [Bindings](../../../docs/guides/bindings.md). |
| `not_before` | String | No | Materializes only the documents published at or after this RFC 3339 time (e.g., `2026-01-01T00:00:00Z`). It's only a filter: changing it doesn't reprocess the documents already read. See [Time Travel](https://docs.estuary.dev/reference/time-travel/). |
| `not_after` | String | No | Materializes only the documents published before this RFC 3339 time. Like `not_before`, it's only a filter. |
| `resource` | Object | Yes | Iceberg table the collection is materialized into. See [`bindings.resource`](#bindingsresource). |
| `fields` | Object | No | Collection fields materialized as columns. Omit it to keep the binding's live selection, such as one set in the dashboard: the provider keeps it in state without showing a diff, until you set `fields` yourself. A new binding without `fields` gets Estuary's default, `recommended = "unlimited"` (`recommended: true` in the spec). See [`bindings.fields`](#bindingsfields). |
| `disable` | Bool | No | Stops materializing the collection while keeping the binding. Defaults to `false`. |
| `priority` | Number | No | Processing order among bindings: when priorities differ, all ready documents of a higher-priority binding are processed before any of a lower one. Defaults to `0` and can be negative, such as `-1` for a binding with a large backlog. See [Processing order](https://docs.estuary.dev/concepts/materialization/#processing-order). |
| `backfill` | Object | No | Backfill control of the binding. See [`bindings.backfill`](#bindingsbackfill). |
| `on_incompatible_schema_change` | String | No | Overrides the materialization's `on_incompatible_schema_change` for this binding: `backfill`, `disableBinding`, `disableTask` or `abort`. |
| `name` | String | Read-only | Collection of the binding, shown in every plan so you can tell which binding changes. |

### `bindings.backfill`

Backfill control of the binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Raise it to backfill the binding: drop the table, create it again and materialize the whole collection again. Starts at 1 and only goes up. Can't be raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `table` | String | Yes | Iceberg table to materialize into. The connector creates it when missing, with the casing of `config.advanced.table_identifier_case`. Changing it on an existing binding moves it to a new table, which the connector backfills from the start of the collection, leaving the old table in place. |
| `namespace` | String | No | Namespace of the table, overriding `config.namespace`. Omitted, it keeps the live one, and a new binding gets the one `target_naming` names, as in the dashboard, or the endpoint default without `target_naming`. Changing it on an existing binding moves it to a new table, which the connector backfills from the start of the collection, leaving the old table in place. |
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
