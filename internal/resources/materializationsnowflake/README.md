# estuary_materialization_snowflake

[Back to the provider documentation](../../../README.md)

This resource manages a [Snowflake](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Snowflake/) materialization connector. Each binding materializes one Estuary collection into a Snowflake table, which the connector creates. Standard bindings stage each transaction in a Snowflake internal stage and merge it into the table, one row per key. [Delta updates](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Snowflake/#delta-updates) bindings skip that merge and, with key-pair (`jwt`) authentication, load through [Snowpipe Streaming](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Snowflake/#performance-considerations).

| Default image | Connector name |
|---|---|
| `ghcr.io/estuary/materialize-snowflake:v4` | `materialize-snowflake` |

## Table of Contents

- [Example Usage](#example-usage)
- [Attributes](#attributes)
  - [`source`](#source)
  - [`target_naming`](#target_naming)
  - [`config`](#config)
  - [`config.credentials`](#configcredentials)
  - [`config.sync_schedule`](#configsync_schedule)
  - [`config.dbt_job_trigger`](#configdbt_job_trigger)
  - [`config.advanced`](#configadvanced)
  - [`backfill`](#backfill)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
  - [`bindings.fields`](#bindingsfields)

## Example Usage

A materialization with key-pair authentication, syncing every 15 minutes on weekday business hours, with one table in another schema clustered by date and one delta updates table:

```hcl
variable "snowflake_private_key" {
  type      = string
  sensitive = true
}

resource "estuary_materialization_snowflake" "warehouse" {
  name = "warehouse"

  config = {
    host           = "orgname-accountname.snowflakecomputing.com"
    database       = "ESTUARY_DB"
    schema         = "ESTUARY_SCHEMA"
    warehouse      = "ESTUARY_WH"
    role           = "ESTUARY_ROLE"
    timestamp_type = "TIMESTAMP_LTZ"

    credentials = {
      auth_type   = "jwt"
      user        = "ESTUARY_USER"
      private_key = var.snowflake_private_key
    }

    sync_schedule = {
      sync_frequency         = "15m"
      timezone               = "America/New_York"
      fast_sync_start_time   = "09:00"
      fast_sync_stop_time    = "17:00"
      fast_sync_enabled_days = "M-F"
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
        table      = "orders"
        schema     = "SALES"
        clustering = "order_date, customer_id"
      }

      fields = {
        recommended = "1"
        exclude     = ["internal_notes"]
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

With `tenant = "acmeCo/"` set on the provider, this creates the materialization `acmeCo/warehouse/materialize-snowflake`.

A near real-time materialization following a capture, so its new collections are materialized as they're added, as delta updates through Snowpipe Streaming, with hard deletes and a dbt Cloud job triggered after new data lands:

```hcl
variable "dbt_api_key" {
  type      = string
  sensitive = true
}

resource "estuary_materialization_snowflake" "streaming" {
  name = "streaming"

  source = {
    capture            = "acmeCo/aurora/source-amazon-aurora-postgres"
    delta_updates      = true
    fields_recommended = "1"
  }

  target_naming = {
    strategy = "matchSourceStructure"
  }

  config = {
    host           = "orgname-accountname.snowflakecomputing.com"
    database       = "ESTUARY_DB"
    schema         = "RAW"
    warehouse      = "ESTUARY_WH"
    timestamp_type = "TIMESTAMP_TZ"
    hard_delete    = true

    credentials = {
      auth_type   = "jwt"
      user        = "ESTUARY_USER"
      private_key = var.snowflake_private_key
    }

    sync_schedule = {
      sync_frequency = "0s"
    }

    dbt_job_trigger = {
      job_id     = "12345"
      account_id = "67890"
      api_key    = var.dbt_api_key
      access_url = "https://cloud.getdbt.com"
      mode       = "skip"
      interval   = "1h"
    }
  }

  bindings = [
    {
      source   = "acmeCo/aurora/public/orders"
      resource = { table = "orders", delta_updates = true }
    },
  ]
}
```

> [!NOTE]
> `private_key`, `password` and `api_key` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Materialization name (e.g., `warehouse`). The provider `tenant` is added in front and `/materialize-snowflake` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the materialization was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the materialization. |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/materialize-snowflake:v4`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `source` | Object | No | Settings of the bindings Estuary adds, optionally following a capture. See [`source`](#source). |
| `target_naming` | Object | No | How Estuary names the tables of the bindings it adds. A change only names the bindings added from then on: existing ones keep their schema and table. See [`target_naming`](#target_naming). |
| `on_incompatible_schema_change` | String | No | What an automated change, such as a capture discovery, does when it's incompatible with a materialized table: `backfill` (default) backfills the affected bindings, `disableBinding` disables them, `disableTask` disables the materialization, and `abort` stops the automated change. Bindings can override it. See [Schema Evolution](https://docs.estuary.dev/concepts/advanced/evolutions/#what-do-schema-evolutions-do). |
| `config` | Object | Yes | Account, authentication, default schema and schedule of the materialization. See [`config`](#config). |
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
| `host` | String | Yes | Snowflake account URL, without the protocol and ending in `.snowflakecomputing.com` (e.g., `orgname-accountname.snowflakecomputing.com`). The plan rejects credentials in it, such as `user:password@` or a password or token option, as state and logs would hold them in plain text. Credentials found in the live value show as `***`. |
| `database` | String | Yes | Snowflake database to materialize into. |
| `schema` | String | Yes | Schema of the bound tables, unless a binding sets its own, and of the materialization metadata tables. |
| `warehouse` | String | No | Snowflake virtual warehouse that runs the queries. Defaults to the user's default warehouse. |
| `role` | String | No | Role the user performs actions with. |
| `account` | String | No | Snowflake account identifier. Optional and hidden in the dashboard. |
| `timestamp_type` | String | Yes | How timestamp columns are stored: `TIMESTAMP_LTZ`, `TIMESTAMP_TZ`, `TIMESTAMP_NTZ (discard TZ)` or `TIMESTAMP_NTZ (normalize to UTC)`. See [Timestamp Data Type Mapping](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Snowflake/#timestamp-data-type-mapping). |
| `hard_delete` | Bool | No | Deletes rows from the destination when they're deleted in the source. Otherwise, the `_meta/op` column marks deleted rows (soft delete). Defaults to `false`. |
| `credentials` | Object | Yes | How the connector authenticates with Snowflake. See [`config.credentials`](#configcredentials). |
| `sync_schedule` | Object | No | When and how often transactions run. See [`config.sync_schedule`](#configsync_schedule). |
| `dbt_job_trigger` | Object | No | Triggers a dbt Cloud job when new data is materialized. See [`config.dbt_job_trigger`](#configdbt_job_trigger). |
| `advanced` | Object | No | Options for advanced users, rarely needed. See [`config.advanced`](#configadvanced). |

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `auth_type` | String | Yes | Authentication method: `jwt` (key-pair) or the deprecated `user_password`. Only the attributes of the selected method are used. |
| `user` | String | Yes | Snowflake user login name. |
| `private_key` | String | With `jwt` | PEM-encoded PKCS#8 private key used to sign the JWT. Write-only, see the note above. |
| `private_key_hash` | String | Read-only | Salted hash of `private_key`, used to detect when it changes. |
| `password` | String | With `user_password` | Password of `user`. Write-only, see the note above. |
| `password_hash` | String | Read-only | Salted hash of `password`, used to detect when it changes. |

> [!TIP]
> Use `jwt`: Snowflake is phasing out password sign-in, and only key-pair authentication loads delta updates bindings through Snowpipe Streaming. See [Key-pair Authentication](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Snowflake/#key-pair-authentication) to create the key and assign it to the user.

### `config.sync_schedule`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `sync_frequency` | String | No | How often transactions run once the materialization is caught up: `0s`, `30s`, `1m`, `2m30s`, `5m`, `15m`, `20m`, `30m`, `1h`, `2h` or `4h`. It applies all day, every day, unless the fast sync window and days limit it. Defaults to `30m`. |
| `timezone` | String | No | Timezone of the fast sync window and days, as an IANA name (e.g., `America/New_York`) or a `+HH:MM` offset. |
| `fast_sync_start_time` | String | No | Time of day, as `HH:MM` (e.g., `09:00`), when transactions start running at `sync_frequency`. Before it they run every 4 hours. |
| `fast_sync_stop_time` | String | No | Time of day, as `HH:MM` (e.g., `17:00`), when transactions stop running at `sync_frequency`. After it they run every 4 hours. |
| `fast_sync_enabled_days` | String | No | Days when `sync_frequency` applies, such as `M-F`, `M,W,F` or `Su-T,Th-S`. On other days transactions run every 4 hours. Defaults to every day. |

> [!TIP]
> The schedule only applies once the materialization is caught up: backfills run as fast as possible. A longer `sync_frequency` resumes the warehouse less often, which lowers Snowflake costs, and `0s` gives the lowest latency, such as for delta updates bindings loading through Snowpipe Streaming. A changed configuration only takes effect once the outstanding transactions complete, which can take up to twice `sync_frequency`. See [Materialization Sync Schedule](https://docs.estuary.dev/reference/materialization-sync-schedule/).

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
| `disable_field_truncation` | Bool | No | Disables truncation of materialized fields. May result in errors for documents with extremely large values or complex nested structures. |
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
| `trigger` | Number | No | Raise it to backfill the binding: truncate the table and materialize the whole collection again. Starts at 1 and only goes up. Can't be raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |

> [!TIP]
> A backfill truncates the table, keeping its clustering and other table settings. It drops and creates the table again instead when the same publication changes a field type incompatibly or the collection key. See [Schema changes during backfill](https://docs.estuary.dev/reference/backfilling-data/#schema-changes-during-backfill).

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `table` | String | Yes | Snowflake table to materialize into. Changing it on an existing binding moves it to a new table, which the connector backfills from the start of the collection, leaving the old table in place. |
| `schema` | String | No | Schema of the table, overriding `config.schema`. Omitted, it keeps the live one, and a new binding gets the one `target_naming` names, as in the dashboard, or the endpoint default without `target_naming`. Changing it on an existing binding moves it to a new table, which the connector backfills from the start of the collection, leaving the old table in place. |
| `delta_updates` | Bool | No | Appends the changes of each transaction instead of merging them into one row per key. With `jwt` credentials, delta updates bindings load through Snowpipe Streaming. Defaults to `false`, see [Delta Updates](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Snowflake/#delta-updates). Switching it from `true` to `false` requires a backfill: raise `backfill.trigger`, or the materialization's, in the same change, or the plan fails. |
| `clustering` | String | No | Comma-separated materialized column names used as the clustering key for [Snowflake Automatic Clustering](https://docs.estuary.dev/reference/Connectors/materialization-connectors/Snowflake/#clustering-keys) (e.g., `date, user_id`); expressions aren't accepted. It's applied each time the materialization restarts, is published or backfills. Empty drops the table's clustering key, one set outside Estuary included. Recommended only for large tables queried with selective filters on these columns, as it consumes additional Snowflake credits. |

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
