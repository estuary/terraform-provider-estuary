# MySQL Materializations

[Back to the provider documentation](../../../README.md)

These resources manage [MySQL](https://docs.estuary.dev/reference/Connectors/materialization-connectors/MySQL/) materialization connectors. Each binding materializes one Estuary collection into a MySQL table, which the connector creates unless it created it before.

They're configured the same way. Pick the resource matching your database:

| Resource | Database | Default image | Connector name |
|---|---|---|---|
| `estuary_materialization_mysql` | MySQL, Google Cloud SQL, Azure Database for MySQL and other MySQL databases | `ghcr.io/estuary/materialize-mysql:v2` | `materialize-mysql` |
| `estuary_materialization_aurora_mysql` | [Amazon Aurora for MySQL](https://docs.estuary.dev/reference/Connectors/materialization-connectors/MySQL/#mysql-on-managed-cloud-platforms) | `ghcr.io/estuary/materialize-amazon-aurora-mysql:v2` | `materialize-amazon-aurora-mysql` |
| `estuary_materialization_rds_mysql` | [Amazon RDS for MySQL](https://docs.estuary.dev/reference/Connectors/materialization-connectors/MySQL/amazon-rds-mysql/) | `ghcr.io/estuary/materialize-amazon-rds-mysql:v2` | `materialize-amazon-rds-mysql` |

## Table of Contents

- [Example Usage](#example-usage)
- [Attributes](#attributes)
  - [`source`](#source)
  - [`target_naming`](#target_naming)
  - [`config`](#config)
  - [`config.dbt_job_trigger`](#configdbt_job_trigger)
  - [`config.network_tunnel`](#confignetwork_tunnel)
  - [`config.advanced`](#configadvanced)
  - [`backfill`](#backfill)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
  - [`bindings.fields`](#bindingsfields)

## Example Usage

A materialization of three collections: one with a reduced field selection keyed by other fields, and one with delta updates that skips the documents published before 2026:

```hcl
variable "mysql_password" {
  type      = string
  sensitive = true
}

resource "estuary_materialization_mysql" "analytics" {
  name = "analytics"

  config = {
    address  = "123.4.567.89:3306"
    user     = "flow_materialize"
    password = var.mysql_password
    database = "analytics"
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/users"
      resource = { table = "users" }
    },
    {
      source   = "acmeCo/mongodb/shop/orders"
      resource = { table = "orders" }

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

With `tenant = "acmeCo/"` set on the provider, this creates the materialization `acmeCo/analytics/materialize-mysql`.

An Amazon Aurora database with hard deletes over a verified TLS connection, following a capture so its new collections are materialized as they're added. Datetimes are materialized in UTC, an incompatible schema change disables the affected binding instead of backfilling it, and one table materializes every field and gets an index:

```hcl
variable "mysql_server_ca" {
  type = string
}

resource "estuary_materialization_aurora_mysql" "replica" {
  name = "aurora/replica"

  source = {
    capture            = "acmeCo/aurora/source-amazon-aurora-mysql"
    delta_updates      = false
    fields_recommended = "1"
  }

  on_incompatible_schema_change = "disableBinding"

  config = {
    address     = "replica.cluster-123456789.us-east-1.rds.amazonaws.com:3306"
    user        = "flow_materialize"
    password    = var.mysql_password
    database    = "replica"
    timezone    = "+00:00"
    hard_delete = true

    advanced = {
      sslmode       = "verify_identity"
      ssl_server_ca = var.mysql_server_ca
    }
  }

  bindings = [
    {
      source = "acmeCo/aurora/shop/orders"
      resource = {
        table                       = "orders"
        additional_table_create_sql = "CREATE INDEX orders_customer_id ON orders (customer_id);"
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

resource "estuary_materialization_rds_mysql" "private" {
  name = "warehouse"

  config = {
    address  = "warehouse.123456789.us-east-1.rds.amazonaws.com:3306"
    user     = "flow_materialize"
    password = var.mysql_password
    database = "warehouse"

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
> `password`, `ssl_server_ca`, `ssl_client_cert`, `ssl_client_key`, `private_key` and `api_key` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

> [!TIP]
> The connector loads data with `LOAD DATA LOCAL INFILE`, so the `local_infile` global variable must be enabled on the database: `SET GLOBAL local_infile = true`.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Materialization name (e.g., `analytics`). The provider `tenant` is added in front and the connector name of the resource (e.g., `/materialize-mysql`) at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the materialization was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the materialization. |
| `image` | String | No | Connector image. Defaults to the default image of the resource, see the table above, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `source` | Object | No | Settings of the bindings Estuary adds, optionally following a capture. See [`source`](#source). |
| `target_naming` | Object | No | How Estuary names the tables of the bindings it adds. A change only names the bindings added from then on: existing ones keep their schema and table. See [`target_naming`](#target_naming). |
| `on_incompatible_schema_change` | String | No | What an automated change, such as a capture discovery, does when it's incompatible with a materialized table: `backfill` (default) backfills the affected bindings, `disableBinding` disables them, `disableTask` disables the materialization, and `abort` stops the automated change. Bindings can override it. See [Schema Evolution](https://docs.estuary.dev/concepts/advanced/evolutions/#what-do-schema-evolutions-do). |
| `config` | Object | Yes | Connection and authentication of the database. See [`config`](#config). |
| `bindings` | List of Objects | Yes | Materialized collections, one per binding. See [`bindings`](#bindings). |
| `backfill` | Object | No | Backfill control of the whole materialization. See [`backfill`](#backfill). |
| `shards` | Dynamic | No | Shard settings of the materialization, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `source.capture`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |

### `source`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `capture` | String | No | Full name of a capture whose bindings the materialization follows (e.g., `acmeCo/aurora/source-amazon-aurora-mysql`). The provider `tenant` isn't added to it. Each collection added to the capture gets a binding here, and bindings removed from the capture are kept. |
| `delta_updates` | Bool | No | Whether added bindings use delta updates. Defaults to `false`. |
| `fields_recommended` | String | No | `fields.recommended` of added bindings: `"0"`, `"1"`, `"2"`, … or `"unlimited"`, see [`bindings.fields`](#bindingsfields). Defaults to `"unlimited"`. |

> [!TIP]
> Bindings Estuary adds through `capture` get only these settings and [`target_naming`](#target_naming). Declare a binding to give it its own settings, see [Bindings](../../../docs/guides/bindings.md).

### `target_naming`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `strategy` | String | Yes | Naming convention of added bindings: `matchSourceStructure`, `singleSchema` or `prefixTableNames`. |
| `schema` | String | No | Schema of every table, with `singleSchema` and `prefixTableNames`. |
| `skip_common_defaults` | Bool | No | With `prefixTableNames`, doesn't prefix the common default schemas `public` and `dbo`. Defaults to `false`. |
| `schema_template` | String | No | With `matchSourceStructure`, template of the schema name around `{{schema}}` (e.g., `raw_{{schema}}`). |
| `table_template` | String | No | Template of the table name around `{{table}}` (e.g., `{{table}}_v2`). |

> [!WARNING]
> MySQL tables have no schema, and Estuary only offers [Target Resource Naming Conventions](https://docs.estuary.dev/concepts/materialization/#target-resource-naming-conventions) for destinations with schemas, so the dashboard doesn't show them for MySQL. Without them, Estuary [prefixes the table name with the source schema](https://docs.estuary.dev/concepts/materialization/#schema-and-table-names-in-destinations): the collection `acmeCo/aurora/shop/orders` gets the table `shop_orders`. Name tables with `bindings.resource.table` instead.

### `config`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `address` | String | Yes | Host or `host:port` of the database, as shown in the RDS console for Aurora and RDS (e.g., `123.4.567.89:3306`). The port defaults to `3306`. The plan rejects credentials in it, such as `user:password@` or a password or token option, as state and logs would hold them in plain text. Credentials found in the live value show as `***`. |
| `user` | String | Yes | Database user to connect as. It needs permission to create tables. |
| `password` | String | Yes | Password of the database user. Write-only, see the note above. |
| `password_hash` | String | Read-only | Salted hash of `password`, used to detect when it changes. |
| `database` | String | Yes | Logical database to materialize to. |
| `timezone` | String | No | Time zone of the materialized datetime columns: an IANA time zone name (e.g., `America/New_York`) or a `+HH:MM` offset. Leave it unset to use the database `time_zone` system variable, which it overrides when both are set. Required when `time_zone` is `SYSTEM`, as the connector can't read the time zone then. |
| `hard_delete` | Bool | No | Deletes from the table the documents deleted in the source. Otherwise, the `_meta/op` column marks deleted rows (soft delete). Defaults to `false`. |
| `dbt_job_trigger` | Object | No | Triggers a dbt Cloud job when new data is materialized. See [`config.dbt_job_trigger`](#configdbt_job_trigger). |
| `network_tunnel` | Object | No | Connects through an SSH server acting as a bastion host. See [`config.network_tunnel`](#confignetwork_tunnel). |
| `advanced` | Object | No | Options for advanced users, rarely needed. See [`config.advanced`](#configadvanced). |

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
| `sslmode` | String | No | TLS of the connection: `disabled` doesn't encrypt it, `preferred` encrypts it when the server supports TLS, `required` encrypts it without verifying the server certificate, `verify_ca` also verifies the certificate against `ssl_server_ca`, and `verify_identity` verifies the hostname as well. |
| `ssl_server_ca` | String | No | PEM certificate authority the server certificate is verified against. Required with `verify_ca` and `verify_identity`. Write-only, see the note above. |
| `ssl_server_ca_hash` | String | Read-only | Salted hash of `ssl_server_ca`, used to detect when it changes. |
| `ssl_client_cert` | String | No | PEM client certificate presented to the server, with `ssl_client_key`. Only used with `verify_ca` and `verify_identity`. Write-only, see the note above. |
| `ssl_client_cert_hash` | String | Read-only | Salted hash of `ssl_client_cert`, used to detect when it changes. |
| `ssl_client_key` | String | No | PEM private key of `ssl_client_cert`. Write-only, see the note above. |
| `ssl_client_key_hash` | String | Read-only | Salted hash of `ssl_client_key`, used to detect when it changes. |
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
| `delta_updates` | Bool | No | Appends the changes of each transaction instead of merging them into one row per key. Defaults to `false`, see [Delta Updates](https://docs.estuary.dev/concepts/materialization/#delta-updates). Switching it from `true` to `false` requires a backfill: raise `backfill.trigger`, or the materialization's, in the same change, or the plan fails. |
| `additional_table_create_sql` | String | No | SQL statements run after the connector creates the table, such as a `CREATE INDEX`. They only run when the connector creates the table: a backfill truncates an existing table without running them, so run them yourself on it. Don't re-create the table in them. |

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
