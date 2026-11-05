# PostgreSQL Materializations

[Back to the provider documentation](../../../README.md)

These resources manage [PostgreSQL](https://docs.estuary.dev/reference/Connectors/materialization-connectors/PostgreSQL/) materialization connectors. Each binding materializes one Estuary collection into a PostgreSQL table, which the connector creates unless it created it before.

They're configured the same way. Pick the resource matching your database:

| Resource | Database | Default image | Connector name |
|---|---|---|---|
| `estuary_materialization_postgres` | PostgreSQL, Google Cloud SQL and other PostgreSQL-compatible databases | `ghcr.io/estuary/materialize-postgres:v5` | `materialize-postgres` |
| `estuary_materialization_aurora_postgres` | Amazon Aurora for PostgreSQL | `ghcr.io/estuary/materialize-amazon-aurora-postgres:v5` | `materialize-amazon-aurora-postgres` |
| `estuary_materialization_rds_postgres` | [Amazon RDS for PostgreSQL](https://docs.estuary.dev/reference/Connectors/materialization-connectors/PostgreSQL/amazon-rds-postgres/) | `ghcr.io/estuary/materialize-amazon-rds-postgres:v5` | `materialize-amazon-rds-postgres` |

## Table of Contents

- [Example Usage](#example-usage)
- [Attributes](#attributes)
  - [`source`](#source)
  - [`target_naming`](#target_naming)
  - [`config`](#config)
  - [`config.credentials`](#configcredentials)
  - [`config.dbt_job_trigger`](#configdbt_job_trigger)
  - [`config.network_tunnel`](#confignetwork_tunnel)
  - [`config.advanced`](#configadvanced)
  - [`backfill`](#backfill)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
  - [`bindings.fields`](#bindingsfields)

## Example Usage

A materialization of three collections with password authentication: one into another schema with a reduced field selection keyed by other fields, and one with delta updates that skips the documents published before 2026:

```hcl
variable "postgres_password" {
  type      = string
  sensitive = true
}

resource "estuary_materialization_postgres" "analytics" {
  name = "analytics"

  config = {
    address  = "123.4.567.89:5432"
    user     = "flow_materialize"
    database = "analytics"
    schema   = "estuary"

    credentials = {
      auth_type = "UserPassword"
      password  = var.postgres_password
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
        table  = "orders"
        schema = "sales"
      }

      fields = {
        recommended = "1"
        exclude     = ["internal_notes"]
        group_by    = ["customer_id", "order_id"]
      }
    },
    {
      source     = "acmeCo/mongodb/shop/events"
      not_before = "2026-01-01T00:00:00Z"
      resource = {
        table         = "events"
        delta_updates = true
      }
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the materialization `acmeCo/analytics/materialize-postgres`.

An Amazon Aurora database with AWS IAM authentication and hard deletes, following a capture so its new collections are materialized as they're added, into tables named after their source schema and table. An incompatible schema change disables the affected binding instead of backfilling it, and one table materializes every field and gets an index:

```hcl
resource "estuary_materialization_aurora_postgres" "replica" {
  name = "aurora/replica"

  source = {
    capture            = "acmeCo/aurora/source-amazon-aurora-postgres"
    delta_updates      = false
    fields_recommended = "1"
  }

  target_naming = {
    strategy = "matchSourceStructure"
  }

  on_incompatible_schema_change = "disableBinding"

  config = {
    address     = "replica.cluster-123456789.us-east-1.rds.amazonaws.com:5432"
    user        = "flow_materialize"
    database    = "replica"
    hard_delete = true

    credentials = {
      auth_type    = "AWSIAM"
      aws_region   = "us-east-1"
      aws_role_arn = "arn:aws:iam::123456789012:role/estuary-materialize"
    }
  }

  bindings = [
    {
      source = "acmeCo/aurora/public/orders"
      resource = {
        table                       = "orders"
        schema                      = "public"
        additional_table_create_sql = "CREATE INDEX orders_customer_id ON public.orders (customer_id);"
      }

      fields = { recommended = "unlimited" }
    },
  ]
}
```

An Amazon RDS database reached through an SSH bastion host, triggering a dbt Cloud job when new data is materialized:

```hcl
variable "ssh_private_key" {
  type      = string
  sensitive = true
}

variable "dbt_api_key" {
  type      = string
  sensitive = true
}

resource "estuary_materialization_rds_postgres" "private" {
  name = "warehouse"

  config = {
    address  = "warehouse.123456789.us-east-1.rds.amazonaws.com:5432"
    user     = "flow_materialize"
    database = "warehouse"

    credentials = {
      auth_type = "UserPassword"
      password  = var.postgres_password
    }

    network_tunnel = {
      ssh_forwarding = {
        ssh_endpoint = "ssh://ec2-user@ec2-123-4-567-89.compute-1.amazonaws.com:22"
        private_key  = var.ssh_private_key
      }
    }

    dbt_job_trigger = {
      access_url = "https://cloud.getdbt.com"
      account_id = "67890"
      job_id     = "12345"
      api_key    = var.dbt_api_key
      mode       = "skip"
      interval   = "1h"
    }

    advanced = {
      sslmode = "require"
    }
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/orders"
      resource = { table = "orders" }
      backfill = { trigger = 1 }
    },
  ]
}
```

> [!NOTE]
> `password`, `private_key` and `api_key` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Materialization name (e.g., `analytics`). The provider `tenant` is added in front and the connector name of the resource (e.g., `/materialize-postgres`) at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the materialization was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the materialization. |
| `image` | String | No | Connector image. Defaults to the default image of the resource, see the table above, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `source` | Object | No | Settings of the bindings Estuary adds, optionally following a capture. See [`source`](#source). |
| `target_naming` | Object | No | How Estuary names the tables of the bindings it adds. A change only names the bindings added from then on: existing ones keep their schema and table. See [`target_naming`](#target_naming). |
| `on_incompatible_schema_change` | String | No | What an automated change, such as a capture discovery, does when it's incompatible with a materialized table: `backfill` (default) backfills the affected bindings, `disableBinding` disables them, `disableTask` disables the materialization, and `abort` stops the automated change. Bindings can override it. See [Schema Evolution](https://docs.estuary.dev/concepts/advanced/evolutions/#what-do-schema-evolutions-do). |
| `config` | Object | Yes | Connection, authentication and default schema of the database. See [`config`](#config). |
| `bindings` | List of Objects | Yes | Materialized collections, one per binding. See [`bindings`](#bindings). |
| `backfill` | Object | No | Backfill control of the whole materialization. See [`backfill`](#backfill). |
| `shards` | Dynamic | No | Shard settings of the materialization, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `source.capture`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |

### `source`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `capture` | String | No | Full name of a capture whose bindings the materialization follows (e.g., `acmeCo/aurora/source-amazon-aurora-postgres`). The provider `tenant` isn't added to it. Each collection added to the capture gets a binding here, and bindings removed from the capture are kept. |
| `delta_updates` | Bool | No | Whether added bindings use delta updates. Defaults to `false`. |
| `fields_recommended` | String | No | `fields.recommended` of added bindings: `"0"`, `"1"`, `"2"`, … or `"unlimited"`, see [`bindings.fields`](#bindingsfields). Defaults to `"unlimited"`. |

> [!TIP]
> Bindings Estuary adds through `capture` get only these settings and [`target_naming`](#target_naming). Declare a binding to give it its own settings, see [Bindings](../../../docs/guides/bindings.md).

### `target_naming`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `strategy` | String | Yes | How added bindings name their schema and table from their collection (e.g., `acmeCo/aurora/public/orders`): `matchSourceStructure` keeps the source schema and table (`public.orders`), `singleSchema` puts every table in `schema` (`<schema>.orders`), and `prefixTableNames` puts every table in `schema` with the source schema as table prefix (`<schema>.public_orders`). |
| `schema` | String | No | Schema of every table, with `singleSchema` and `prefixTableNames`. |
| `skip_common_defaults` | Bool | No | With `prefixTableNames`, doesn't prefix the common default schemas `public` and `dbo` (`orders` instead of `public_orders`). Defaults to `false`. |
| `schema_template` | String | No | With `matchSourceStructure`, template of the schema name around `{{schema}}` (e.g., `raw_{{schema}}`). |
| `table_template` | String | No | Template of the table name around `{{table}}` (e.g., `{{table}}_v2`). |

> [!TIP]
> Changing `target_naming` only affects the bindings added afterwards, never the existing ones. See [Target Resource Naming Conventions](https://docs.estuary.dev/concepts/materialization/#target-resource-naming-conventions).

### `config`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `address` | String | Yes | Host or `host:port` of the database, as shown in the RDS console for Aurora and RDS (e.g., `123.4.567.89:5432`). The port defaults to `5432`. The plan rejects credentials in it, such as `user:password@` or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as `***`. |
| `user` | String | Yes | Database user to connect as. |
| `database` | String | No | Logical database to materialize to. |
| `schema` | String | No | Schema of the materialized tables and of the connector's metadata tables, unless a binding sets its own. Defaults to `public`. |
| `hard_delete` | Bool | No | Deletes from the table the documents deleted in the source. Otherwise, the `_meta/op` column marks deleted rows (soft delete). Defaults to `false`. |
| `credentials` | Object | No | How the user authenticates. See [`config.credentials`](#configcredentials). Set it, or the deprecated `password`, for the materialization to connect. |
| `password` | String | No | Deprecated form of `credentials` with `auth_type = "UserPassword"`, still accepted for existing materializations. Write-only, see the note above. |
| `password_hash` | String | Read-only | Salted hash of `password`, used to detect when it changes. |
| `dbt_job_trigger` | Object | No | Triggers a dbt Cloud job when new data is materialized. See [`config.dbt_job_trigger`](#configdbt_job_trigger). |
| `network_tunnel` | Object | No | Connects through an SSH server acting as a bastion host. See [`config.network_tunnel`](#confignetwork_tunnel). |
| `advanced` | Object | No | Options for advanced users, rarely needed. See [`config.advanced`](#configadvanced). |

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `auth_type` | String | Yes | Authentication method: `UserPassword`, `AWSIAM`, `GCPIAM` or `AzureIAM`. Only the attributes of the selected method are used. |
| `password` | String | With `UserPassword` | Password of the database user. Write-only, see the note above. |
| `password_hash` | String | Read-only | Salted hash of `password`, used to detect when it changes. |
| `aws_region` | String | With `AWSIAM` | AWS region of the database (e.g., `us-east-1`). |
| `aws_role_arn` | String | With `AWSIAM` | ARN of the IAM role Estuary assumes to connect, see the [AWS IAM guide](https://docs.estuary.dev/guides/iam-auth/aws/). |
| `gcp_service_account_to_impersonate` | String | With `GCPIAM` | Email of the Google Cloud service account used for IAM authentication. |
| `gcp_workload_identity_pool_audience` | String | With `GCPIAM` | Audience of the workload identity pool, in the form `//iam.googleapis.com/projects/123456789012/locations/global/workloadIdentityPools/<pool>/providers/<provider>`. |
| `azure_client_id` | String | With `AzureIAM` | Client ID of the Azure app registration used for Microsoft Entra authentication. |
| `azure_tenant_id` | String | With `AzureIAM` | Azure tenant ID of the app registration. |

> [!TIP]
> With `GCPIAM` or `AzureIAM`, connections use TLS (`sslmode = require`) unless `advanced.sslmode` says otherwise.

### `config.dbt_job_trigger`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `job_id` | String | Yes | ID of the dbt job to trigger. |
| `account_id` | String | Yes | ID of the dbt account. |
| `api_key` | String | Yes | dbt API key used to trigger the job. Write-only, see the note above. |
| `api_key_hash` | String | Read-only | Salted hash of `api_key`, used to detect when it changes. |
| `access_url` | String | No | Access URL of the dbt account, found in its Account Settings (e.g., `https://cloud.getdbt.com`). It must start with `https://`. The job is only triggered when it's set. The plan rejects credentials in it, such as `user:password@` or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as `***`. |
| `cause` | String | No | Message shown as the cause of each triggered run. Defaults to `Estuary`. |
| `mode` | String | No | What happens when the job is already running: `skip` doesn't trigger it (default), `replace` cancels the running job and triggers a new one, and `ignore` triggers a new one anyway. |
| `interval` | String | No | Minimum time between two triggers, which only happen after new data is materialized. Defaults to `30m`. |

### `config.network_tunnel`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `ssh_forwarding` | Object | No | SSH server tunneling the connection to the database. |
| `ssh_forwarding.ssh_endpoint` | String | Yes | Endpoint of the SSH server, in the form `ssh://user@hostname[:port]`. The plan rejects a password in it, such as `user:password@`, as state and logs would hold it in plain text: authenticate with `private_key`. A password found in the live value shows as `***`. |
| `ssh_forwarding.private_key` | String | Yes | Private key used to connect to the SSH server. Write-only, see the note above. |
| `ssh_forwarding.private_key_hash` | String | Read-only | Salted hash of `private_key`, used to detect when it changes. |

### `config.advanced`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `sslmode` | String | No | Overrides the PostgreSQL `sslmode` connection parameter: `disable`, `allow`, `prefer`, `require`, `verify-ca` or `verify-full`. |
| `no_flow_document` | Bool | No | Stops requiring the `flow_document` field in bindings with standard updates. Defaults to `false`, see [Field Selection](../../../docs/guides/bindings.md#field-selection). |
| `feature_flags` | String | No | Connector settings for edge cases and experimental features, see [Feature Flags](https://docs.estuary.dev/guides/advanced-usage/feature-flags/). Intended for Estuary internal use: change them only as directed by Estuary support. |

### `backfill`

Backfill control of the whole materialization, as the dashboard's **Backfill** button under **Source Collections → Advanced options**. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|---|---|---|---|
| `trigger` | Number | No | Raise it to backfill every enabled binding of the materialization, those Estuary adds through `source.capture` included: each table is truncated and materialized again from its collection. Disabled bindings are skipped. Starts at 1 and only goes up. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `source` | String | Yes | Estuary collection the binding materializes (e.g., `acmeCo/mongodb/shop/orders`). The provider `tenant` isn't added to it. Changing it replaces the binding, see [Bindings](../../../docs/guides/bindings.md). |
| `not_before` | String | No | Materializes only the documents published at or after this RFC 3339 time (e.g., `2026-01-01T00:00:00Z`). It's only a filter: changing it doesn't reprocess the documents already read. See [Time Travel](https://docs.estuary.dev/reference/time-travel/). |
| `not_after` | String | No | Materializes only the documents published before this RFC 3339 time. Like `not_before`, it's only a filter. |
| `resource` | Object | Yes | Table the collection is materialized into. See [`bindings.resource`](#bindingsresource). |
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
| `trigger` | Number | No | Raise it to backfill the binding: materialize the whole collection again. Starts at 1 and only goes up. Can't be raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `table` | String | Yes | Table to materialize into. The connector creates it, unless it created it before. Changing it on an existing binding moves it to a new table, which the connector backfills from the start of the collection, leaving the old table in place. |
| `schema` | String | No | Schema of the table, overriding `config.schema`. Omitted, it keeps the live one, and a new binding gets the one `target_naming` names, as in the dashboard, or the endpoint default without `target_naming`. Changing it on an existing binding moves it to a new table, which the connector backfills from the start of the collection, leaving the old table in place. |
| `delta_updates` | Bool | No | Appends the changes of each transaction instead of merging them into one row per key. Defaults to `false`, see [Delta Updates](https://docs.estuary.dev/concepts/materialization/#delta-updates). Switching it from `true` to `false` requires a backfill: raise `backfill.trigger`, or the materialization's, in the same change, or the plan fails. |
| `additional_table_create_sql` | String | No | SQL statements run in the same transaction that creates the table, such as a `CREATE INDEX`. They only run when the connector creates the table: a backfill truncates an existing table without running them, so run them yourself on it. Don't re-create the table in them. |

> [!TIP]
> Switching a binding from delta updates back to standard updates requires raising its `backfill.trigger`, or the materialization's, in the same change: the plan fails otherwise. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

### `bindings.fields`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `recommended` | String | Yes | Depth of the fields selected by default: `"0"` selects only the root document and the required fields, such as the collection key, `"1"` the top-level fields, `"2"` the second-level fields too, and so on, and `"unlimited"` every field. |
| `require` | Set of Strings | No | Fields always materialized, whatever `recommended` selects. |
| `exclude` | List of Strings | No | Fields never materialized. |
| `group_by` | List of Strings | No | Fields keying the table instead of the collection key. Only fields with a scalar type (boolean, integer or string) can be keys, and changing them backfills the binding. See [Group-by keys](https://docs.estuary.dev/guides/customize-materialization-fields/#group-by-keys). |

See [Field Selection](../../../docs/guides/bindings.md#field-selection) for examples and how `flow_document` is selected.
