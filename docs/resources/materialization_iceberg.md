---
page_title: "estuary_materialization_iceberg Resource - Estuary"
subcategory: "Materializations"
description: |-
  Materializes Estuary collections into Apache Iceberg tables of a REST catalog, merging changes into one row per key with Spark jobs on AWS EMR Serverless.
---

# estuary_materialization_iceberg (Resource)

Materializes Estuary collections into Apache Iceberg tables of a REST catalog, merging changes into one row per key with Spark jobs on AWS EMR Serverless.

See the [Bindings](https://registry.terraform.io/providers/estuary/estuary/latest/docs/guides/bindings), [Backfills](https://registry.terraform.io/providers/estuary/estuary/latest/docs/guides/backfills) and [Secrets](https://registry.terraform.io/providers/estuary/estuary/latest/docs/guides/secrets) guides for how the provider manages them.

## Example Usage

```terraform
variable "catalog_credential" {
  type      = string
  sensitive = true
}

variable "emr_secret_access_key" {
  type      = string
  sensitive = true
}

variable "catalog_secret_access_key" {
  type      = string
  sensitive = true
}

variable "dbt_api_key" {
  type      = string
  sensitive = true
}

# AWS Glue with IAM authentication shared by the EMR jobs, Glue optimizers and a sync schedule.
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

# Snowflake Open Catalog with OAuth client credentials, following a capture with upper-case identifiers.
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

# S3 Tables with AWS SigV4, lowercase columns, Spark overrides and a dbt Cloud job trigger.
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

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `bindings` (Attributes List) Materialized collections, one per binding. (see [below for nested schema](#nestedatt--bindings))
- `config` (Attributes) Connection, authentication and default settings of the destination. (see [below for nested schema](#nestedatt--config))
- `name` (String) Catalog name of the materialization (e.g., shop). The provider adds the tenant in front and the connector name at the end, unless they're already there. Changing the resulting catalog name, through name or the provider tenant, replaces the materialization.

### Optional

- `backfill` (Attributes) Backfill control of the whole materialization. (see [below for nested schema](#nestedatt--backfill))
- `image` (String) Connector image, defaulting to ghcr.io/estuary/materialize-iceberg:v1. Only the tag can change, e.g. ghcr.io/estuary/materialize-iceberg:dev.
- `keep_unmanaged_bindings` (Boolean) Keeps the live bindings the configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. False manages every binding, removing undeclared ones, and conflicts with source.capture. Defaults to true. A provider control Estuary never sees.
- `on_incompatible_schema_change` (String) What an automated change, such as a capture discovery, does when it's incompatible with a materialized table: 'backfill' (default) backfills the affected bindings, 'disableBinding' disables them, 'disableTask' disables the materialization, and 'abort' stops the automated change. Bindings can override it.
- `shards` (Dynamic) Shard settings of the materialization, with Estuary's own keys such as logLevel and disable.
- `source` (Attributes) Settings of the bindings Estuary adds, optionally following a capture. (see [below for nested schema](#nestedatt--source))
- `target_naming` (Attributes) How Estuary names the tables of the bindings it adds. A change only names the bindings added from then on: existing ones keep their schema and table. (see [below for nested schema](#nestedatt--target_naming))

### Read-Only

- `catalog_name` (String) Catalog name the materialization was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider tenant change replaces the materialization under the new tenant, with a plan warning, instead of managing whatever entry has its name there.

<a id="nestedatt--bindings"></a>
### Nested Schema for `bindings`

Required:

- `resource` (Attributes) Destination resource the collection is materialized into, such as a table. (see [below for nested schema](#nestedatt--bindings--resource))
- `source` (String) Estuary collection the binding materializes. The provider tenant is not added to it. Changing it keeps the binding, which Estuary identifies by its resource path.

Optional:

- `backfill` (Attributes) Backfill control of the binding. (see [below for nested schema](#nestedatt--bindings--backfill))
- `disable` (Boolean) Stops materializing the collection while keeping the binding. Defaults to false.
- `fields` (Attributes) Collection fields materialized as columns. Omitted, it keeps the live selection, and new bindings get Estuary's default recommended = "unlimited". (see [below for nested schema](#nestedatt--bindings--fields))
- `not_after` (String) Materializes only the documents published before this RFC 3339 time. It only filters, like not_before.
- `not_before` (String) Materializes only the documents published at or after this RFC 3339 time. It only filters: changing it does not reprocess documents already read.
- `on_incompatible_schema_change` (String) Overrides the on_incompatible_schema_change of the materialization for this binding: backfill, disableBinding, disableTask or abort.
- `priority` (Number) Processing order among bindings: all ready documents of a higher-priority binding are processed before any of a lower one. Defaults to 0 and can be negative.

Read-Only:

- `name` (String) Collection of the binding, its target in captures and its source in materializations. Plans always show it, identifying the changed binding.
- `resource_path` (List of String) Resource path Estuary identifies the binding by, such as its schema and table, as in the dashboard. A binding is the live one with its resource path, never another binding of its collection. Known at plan time when the plan validates the bindings it changes, else after the apply.

<a id="nestedatt--bindings--resource"></a>
### Nested Schema for `bindings.resource`

Required:

- `table` (String) Iceberg table to materialize into. The connector creates it when missing. Changing it on an existing binding moves it to a new table, which the connector backfills from the start of the collection, leaving the old table in place.

Optional:

- `additional_table_properties` (Map of String) Iceberg table properties set when the table is created (e.g., 'write.parquet.compression-codec' = 'zstd'). They can't be changed afterwards.
- `namespace` (String) Namespace of the table, overriding config.namespace. Omitted, it keeps the live one, and a new binding gets the one target_naming names, as in the dashboard, or the endpoint default without target_naming. Changing it on an existing binding moves it to a new table, which the connector backfills from the start of the collection, leaving the old table in place.


<a id="nestedatt--bindings--backfill"></a>
### Nested Schema for `bindings.backfill`

Optional:

- `trigger` (Number) Set or raise it to backfill the binding, or every enabled binding of the materialization: each binding is materialized again from its collection. A provider control Estuary never sees, not its backfill counter: each set or raise adds one to the live counter. Setting it when the binding or the materialization is added, or in the first apply after an import, backfills nothing. At least 1, can't be lowered, and can't be set or raised on a disabled binding.


<a id="nestedatt--bindings--fields"></a>
### Nested Schema for `bindings.fields`

Required:

- `recommended` (String) Depth of the fields selected by default: '0' selects only the root document and the required fields, such as the collection key, '1' the top-level fields, '2' the second-level fields too, and so on, and 'unlimited' every field.

Optional:

- `exclude` (List of String) Fields never materialized.
- `group_by` (List of String) Fields keying the table instead of the collection key. Only fields with a scalar type (boolean, integer or string) can be keys, and changing them backfills the binding.
- `require` (Set of String) Fields always materialized, whatever recommended selects.



<a id="nestedatt--config"></a>
### Nested Schema for `config`

Required:

- `compute` (Attributes) EMR Serverless application that merges the staged data into the tables. (see [below for nested schema](#nestedatt--config--compute))
- `namespace` (String) Namespace of the materialized tables, unless a binding sets its own. It can't contain dots.
- `url` (String) Base URL of the Iceberg REST catalog: 'https://glue.<region>.amazonaws.com/iceberg' for AWS Glue, 'https://s3tables.<region>.amazonaws.com/iceberg' for S3 Tables, or the URL of another REST catalog. The plan rejects credentials in it, such as user:password@ or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as ***.
- `warehouse` (String) Warehouse to connect to: the AWS account ID without hyphens for AWS Glue, or the table bucket ARN for S3 Tables.

Optional:

- `advanced` (Attributes) Options for advanced users, rarely needed. (see [below for nested schema](#nestedatt--config--advanced))
- `base_location` (String) S3 location of the table data and metadata (e.g., 's3://<bucket>/<prefix>/'). Required with AWS Glue, optional with other catalogs.
- `catalog_authentication` (Attributes) Deprecated form of credentials, still accepted for existing materializations. Ignored when credentials is set. (see [below for nested schema](#nestedatt--config--catalog_authentication))
- `credentials` (Attributes) How the connector authenticates with the catalog. Set it, or the deprecated catalog_authentication, for the materialization to connect. (see [below for nested schema](#nestedatt--config--credentials))
- `dbt_job_trigger` (Attributes) Triggers a dbt Cloud job when new data is materialized. (see [below for nested schema](#nestedatt--config--dbt_job_trigger))
- `glue_optimizers` (Attributes) AWS Glue managed table optimizers, registered on each table. Only with AWS Glue as the catalog. (see [below for nested schema](#nestedatt--config--glue_optimizers))
- `hard_delete` (Boolean) Deletes from the table the documents deleted in the source. Otherwise, the _meta/op column marks deleted rows (soft delete). Defaults to false.
- `sync_schedule` (Attributes) When and how often transactions run. (see [below for nested schema](#nestedatt--config--sync_schedule))

<a id="nestedatt--config--compute"></a>
### Nested Schema for `config.compute`

Required:

- `application_id` (String) ID of the EMR Serverless application, which needs the Spark runtime.
- `bucket` (String) S3 bucket where data files are staged before they're merged into tables.
- `compute_type` (String) Compute backend running the merge jobs. Only 'AWS EMR Serverless' is supported.
- `execution_role_arn` (String) ARN of the IAM role the EMR Serverless jobs run as.
- `region` (String) AWS region of the EMR Serverless application and the staging bucket (e.g., 'us-east-1').

Optional:

- `aws_access_key_id` (String) Deprecated form of credentials with auth_type 'AWSAccessKey', still accepted for existing materializations.
- `aws_secret_access_key` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Deprecated form of credentials with auth_type 'AWSAccessKey', still accepted for existing materializations. Write-only.
- `bucket_path` (String) Prefix of the staged data files in bucket.
- `credentials` (Attributes) How the connector authenticates with EMR and the staging bucket. Set it, or the deprecated aws_access_key_id and aws_secret_access_key, for the materialization to connect. (see [below for nested schema](#nestedatt--config--compute--credentials))
- `spark_job_properties` (Attributes List) Spark properties overriding the EMR Serverless defaults of the merge jobs. (see [below for nested schema](#nestedatt--config--compute--spark_job_properties))
- `systems_manager_prefix` (String) AWS Systems Manager path, starting and ending with '/' (e.g., '/estuary/'), where the connector stores the catalog credential for the EMR jobs. Required with 'OAuth 2.0 Client Credentials' catalog authentication.

Read-Only:

- `aws_secret_access_key_hash` (String, Sensitive) Salted hash of aws_secret_access_key, detecting its changes without storing it.

<a id="nestedatt--config--compute--credentials"></a>
### Nested Schema for `config.compute.credentials`

Required:

- `auth_type` (String) Authentication method: 'AWSAccessKey', or 'UseCatalogAuth' to reuse the catalog credentials, which then must use 'AWS SigV4' or 'AWS IAM'. Only the attributes of the selected method are used.

Optional:

- `aws_access_key_id` (String) AWS access key ID of the IAM user that runs the EMR jobs and writes to the staging bucket, with 'AWSAccessKey'.
- `aws_secret_access_key` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) AWS secret access key of the IAM user, with 'AWSAccessKey'. Write-only.

Read-Only:

- `aws_secret_access_key_hash` (String, Sensitive) Salted hash of aws_secret_access_key, detecting its changes without storing it.


<a id="nestedatt--config--compute--spark_job_properties"></a>
### Nested Schema for `config.compute.spark_job_properties`

Required:

- `key` (String) Spark property to override: 'spark.executor.instances', 'spark.dynamicAllocation.initialExecutors' or 'spark.dynamicAllocation.maxExecutors'.
- `value` (String) Value of the property, letters and digits only (e.g., '8').



<a id="nestedatt--config--advanced"></a>
### Nested Schema for `config.advanced`

Optional:

- `feature_flags` (String) Connector settings for edge cases and experimental features, see https://docs.estuary.dev/guides/advanced-usage/feature-flags/. Intended for Estuary internal use: change them only as directed by Estuary support.
- `field_name_case` (String) Casing of the names of created columns: 'preserve' (default), 'lowercase', needed to query S3 Table Buckets with Athena, or 'uppercase'. Existing columns are matched case-insensitively and never renamed.
- `lowercase_column_names` (Boolean) Deprecated form of field_name_case 'lowercase', still accepted for existing materializations. It conflicts with field_name_case 'uppercase' or 'preserve'.
- `table_identifier_case` (String) Casing of the namespace and table names of created tables: 'lowercase' (default), 'uppercase', for catalogs resolving unquoted identifiers as upper case such as Snowflake's, or 'preserve'. Existing tables are never renamed.


<a id="nestedatt--config--catalog_authentication"></a>
### Nested Schema for `config.catalog_authentication`

Required:

- `catalog_auth_type` (String) Authentication method: 'OAuth 2.0 Client Credentials' or 'AWS SigV4'. Only the attributes of the selected method are used.

Optional:

- `aws_access_key_id` (String) AWS access key ID of the IAM user the connector authenticates as, with 'AWS SigV4'.
- `aws_secret_access_key` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) AWS secret access key of the IAM user, with 'AWS SigV4'. Write-only.
- `credential` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Catalog credential, with 'OAuth 2.0 Client Credentials': '<client_id>:<client_secret>', or '<token>' for Bearer authentication. Write-only.
- `oauth2_server_uri` (String) OAuth 2.0 token endpoint, required with 'OAuth 2.0 Client Credentials': a path relative to url, 'v1/oauth/tokens' for most catalogs, or a full http(s) URL. The plan rejects credentials in it, such as user:password@ or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as ***.
- `region` (String) AWS region of the catalog (e.g., 'us-east-1'), with 'AWS SigV4'.
- `scope` (String) Authorization scope requested with 'OAuth 2.0 Client Credentials'.
- `signing_name` (String) Ignored by the connector, which derives the SigV4 signing name from url. Kept so existing materializations still decode.

Read-Only:

- `aws_secret_access_key_hash` (String, Sensitive) Salted hash of aws_secret_access_key, detecting its changes without storing it.
- `credential_hash` (String, Sensitive) Salted hash of credential, detecting its changes without storing it.


<a id="nestedatt--config--credentials"></a>
### Nested Schema for `config.credentials`

Required:

- `auth_type` (String) Authentication method: 'OAuth 2.0 Client Credentials', 'AWS SigV4' or 'AWS IAM'. AWS Glue and S3 Tables catalogs need 'AWS SigV4' or 'AWS IAM'. Only the attributes of the selected method are used.

Optional:

- `aws_access_key_id` (String) AWS access key ID of the IAM user the connector authenticates as, with 'AWS SigV4'.
- `aws_region` (String) AWS region of the catalog (e.g., 'us-east-1'), with 'AWS SigV4' and 'AWS IAM'.
- `aws_role_arn` (String) ARN of the IAM role Estuary assumes to connect, with 'AWS IAM'. See https://docs.estuary.dev/guides/iam-auth/aws/.
- `aws_secret_access_key` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) AWS secret access key of the IAM user, with 'AWS SigV4'. Write-only.
- `credential` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Catalog credential, with 'OAuth 2.0 Client Credentials': '<client_id>:<client_secret>' for OAuth client credentials, or '<token>' for Bearer authentication. Write-only.
- `oauth2_server_uri` (String) OAuth 2.0 token endpoint, required with 'OAuth 2.0 Client Credentials': a path relative to url, 'v1/oauth/tokens' (the dashboard default) for most catalogs, or a full http(s) URL when the token endpoint is on another host (e.g., 'https://login.dremio.cloud/oauth/token'). The plan rejects credentials in it, such as user:password@ or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as ***.
- `scope` (String) Authorization scope requested with 'OAuth 2.0 Client Credentials' (e.g., 'PRINCIPAL_ROLE:<role>').

Read-Only:

- `aws_secret_access_key_hash` (String, Sensitive) Salted hash of aws_secret_access_key, detecting its changes without storing it.
- `credential_hash` (String, Sensitive) Salted hash of credential, detecting its changes without storing it.


<a id="nestedatt--config--dbt_job_trigger"></a>
### Nested Schema for `config.dbt_job_trigger`

Required:

- `account_id` (String) ID of the dbt account.
- `api_key` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) dbt API key used to trigger the job. Write-only.
- `job_id` (String) ID of the dbt job to trigger.

Optional:

- `access_url` (String) Access URL of the dbt account, found in its Account Settings (e.g., https://cloud.getdbt.com). It must start with https://. The job is only triggered when it's set. The plan rejects credentials in it, such as user:password@ or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as ***.
- `cause` (String) Message shown as the cause of each triggered run. Defaults to 'Estuary'.
- `interval` (String) Minimum time between two triggers, which only happen after new data is materialized. Defaults to '30m'.
- `mode` (String) What happens when the job is already running: 'skip' doesn't trigger it (default), 'replace' cancels the running job and triggers a new one, and 'ignore' triggers a new one anyway.

Read-Only:

- `api_key_hash` (String, Sensitive) Salted hash of api_key, detecting its changes without storing it.


<a id="nestedatt--config--glue_optimizers"></a>
### Nested Schema for `config.glue_optimizers`

Optional:

- `enable_compaction` (Boolean) Compacts small data files to speed up queries. Defaults to false.
- `enable_orphan_file_deletion` (Boolean) Deletes files no table snapshot references. Defaults to false.
- `enable_retention` (Boolean) Removes old table snapshots to cut storage costs. Defaults to false.
- `execution_role_arn` (String) ARN of the IAM role Glue assumes to optimize tables. It needs read and write access to the table data in S3 and access to the Glue catalog. Required when any optimizer is enabled.
- `number_of_snapshots_to_retain` (Number) Snapshots always kept, whatever their age, with enable_retention. Defaults to Glue's 1.
- `orphan_file_retention_days` (Number) Days orphan files are kept before deletion, with enable_orphan_file_deletion. Defaults to Glue's 3.
- `snapshot_retention_days` (Number) Days snapshots are kept, with enable_retention. Defaults to Glue's 5.


<a id="nestedatt--config--sync_schedule"></a>
### Nested Schema for `config.sync_schedule`

Optional:

- `fast_sync_enabled_days` (String) Days when syncFrequency applies, such as 'M-F', 'M,W,F' or 'Su-T,Th-S'. On other days transactions run every 4 hours. Defaults to every day.
- `fast_sync_start_time` (String) Time of day, as HH:MM (e.g., '09:00'), when transactions start running at syncFrequency. Before it they run every 4 hours.
- `fast_sync_stop_time` (String) Time of day, as HH:MM (e.g., '17:00'), when transactions stop running at syncFrequency. After it they run every 4 hours.
- `sync_frequency` (String) How often transactions run once the materialization is caught up: '0s', '30s', '1m', '2m30s', '5m', '15m', '20m', '30m', '1h', '2h' or '4h'. It applies all day, every day, unless the fast sync window and days limit it. Defaults to '30m'.
- `timezone` (String) Timezone of the fast sync window and days, as an IANA name (e.g., 'America/New_York') or a +HH:MM offset.



<a id="nestedatt--backfill"></a>
### Nested Schema for `backfill`

Optional:

- `trigger` (Number) Set or raise it to backfill the binding, or every enabled binding of the materialization: each binding is materialized again from its collection. A provider control Estuary never sees, not its backfill counter: each set or raise adds one to the live counter. Setting it when the binding or the materialization is added, or in the first apply after an import, backfills nothing. At least 1, can't be lowered, and can't be set or raised on a disabled binding.


<a id="nestedatt--source"></a>
### Nested Schema for `source`

Optional:

- `capture` (String) Full name of a capture whose bindings the materialization follows. The provider tenant isn't added to it. Each collection added to the capture gets a binding here, and bindings removed from the capture are kept.
- `delta_updates` (Boolean) Whether added bindings use delta updates, for connectors offering both modes. Defaults to false.
- `fields_recommended` (String) fields.recommended of added bindings: '0', '1', '2', ... or 'unlimited', see bindings.fields. Defaults to 'unlimited'.


<a id="nestedatt--target_naming"></a>
### Nested Schema for `target_naming`

Required:

- `strategy` (String) How added bindings name their schema and table from their collection: 'matchSourceStructure' keeps the source schema and table, 'singleSchema' puts every table in schema, and 'prefixTableNames' puts every table in schema with the source schema as table prefix.

Optional:

- `schema` (String) Schema of every table, with 'singleSchema' and 'prefixTableNames'.
- `schema_template` (String) With 'matchSourceStructure', template of the schema name around {{schema}} (e.g., 'raw_{{schema}}').
- `skip_common_defaults` (Boolean) With 'prefixTableNames', doesn't prefix the common default schemas 'public' and 'dbo'. Defaults to false.
- `table_template` (String) Template of the table name around {{table}} (e.g., '{{table}}_v2').
