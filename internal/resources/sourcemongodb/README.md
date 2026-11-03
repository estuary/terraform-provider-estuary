# estuary_source_mongodb

[Back to the provider documentation](../../../README.md)

This resource manages a [MongoDB](https://docs.estuary.dev/reference/Connectors/capture-connectors/MongoDB/) capture connector. Each binding captures one MongoDB collection into an Estuary collection, using change streams by default or batch polling for views, time series collections and deployments without change streams.

## Table of Contents

- [Example Usage](#example-usage)
- [Attributes](#attributes)
  - [`auto_discover`](#auto_discover)
  - [`backfill`](#backfill)
  - [`config`](#config)
  - [`config.network_tunnel`](#confignetwork_tunnel)
  - [`config.advanced`](#configadvanced)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
- [Data Source](#data-source)

## Example Usage

A change stream capture of two collections, with auto discovery adding every new collection of the `shop` database:

```hcl
variable "mongodb_password" {
  type      = string
  sensitive = true
}

resource "estuary_source_mongodb" "shop" {
  name = "mongodb"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    address  = "mongodb+srv://cluster0.123456789.mongodb.net?authSource=admin"
    user     = "flow_capture"
    password = var.mongodb_password
    database = "shop"
  }

  bindings = [
    {
      resource = {
        database   = "shop"
        collection = "users"
      }
    },
    {
      resource = {
        database   = "shop"
        collection = "orders"
      }
      target = "acmeCo/shop/orders"
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the capture `acmeCo/mongodb/source-mongodb`. The `users` binding sets no `target`, so its data lands in the collection the connector discovery names, such as `acmeCo/mongodb/shop/users`.

Batch capture of a view and a time series collection, alongside a change stream collection:

```hcl
resource "estuary_source_mongodb" "metrics" {
  name = "mongodb/metrics"

  config = {
    address                 = "mongodb://123.4.567.89:27017?authSource=admin"
    user                    = "flow_capture"
    password                = var.mongodb_password
    database                = "shop,metrics"
    batch_and_change_stream = true
    poll_schedule           = "6h"
  }

  bindings = [
    {
      resource = {
        database      = "shop"
        collection    = "user_view"
        capture_mode  = "Batch Snapshot"
        cursor_field  = "_id"
        poll_schedule = "1h"
      }
    },
    {
      resource = {
        database      = "metrics"
        collection    = "readings"
        capture_mode  = "Batch Incremental"
        cursor_field  = "ts"
        poll_schedule = "daily at 02:00Z"
      }
    },
    {
      resource = {
        database     = "shop"
        collection   = "events"
        capture_mode = "Change Stream Incremental"
      }
    },
  ]
}
```

Connecting through an SSH bastion host, with advanced change stream options:

```hcl
variable "ssh_private_key" {
  type      = string
  sensitive = true
}

resource "estuary_source_mongodb" "private" {
  name = "mongodb/private"

  config = {
    address  = "mongodb://123.4.567.89:27017?authSource=admin"
    user     = "flow_capture"
    password = var.mongodb_password

    network_tunnel = {
      ssh_forwarding = {
        ssh_endpoint = "ssh://ec2-user@ec2-123-4-567-89.compute-1.amazonaws.com:22"
        private_key  = var.ssh_private_key
      }
    }

    advanced = {
      max_await_time      = "10s"
      disable_pre_images  = true
      exclude_collections = "shop:sessions,shop:carts"
      skip_backfills      = "shop:events"
    }
  }

  bindings = [
    {
      resource = {
        database   = "shop"
        collection = "events"
      }
    },
  ]
}
```

> [!NOTE]
> `password` and `private_key` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name (e.g., `mongodb`). The provider `tenant` is added in front and `/source-mongodb` at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the capture was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the capture. |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/source-mongodb:v3`, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `auto_discover` | Object | No | Periodically checks the deployment for new collections and schema changes. See [`auto_discover`](#auto_discover). |
| `config` | Object | Yes | Connection and discovery settings of the MongoDB deployment. See [`config`](#config). |
| `bindings` | List of Objects | Yes | Captured collections, one per binding. See [`bindings`](#bindings). |
| `shards` | Dynamic | No | Shard settings of the capture, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `auto_discover`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |
| `backfill` | Object | No | Backfill control of the whole capture. See [`backfill`](#backfill). |

### `auto_discover`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `add_new_bindings` | Bool | No | Adds a binding for each collection found in the deployment that isn't declared, when the capture is created and on each later discovery. Terraform doesn't manage those bindings; declare a binding to manage it. |
| `evolve_incompatible_collections` | Bool | No | Automatically updates the target collections when the source schema changes in a way they can't accept. |

### `backfill`

Backfill control of the whole capture, as the dashboard's **Backfill** button and **Backfill mode**. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Raise it to backfill every enabled binding of the capture, including those Terraform doesn't manage. Disabled bindings are skipped. Starts at 1 and only goes up. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |
| `strategy` | String | No | Default strategy of the bindings, which a binding's `backfill.strategy` overrides: `incremental` (default) captures again into the existing collection, `dataflowReset` resets the collection instead, backfilling its materializations and truncating their tables. Changing it alone publishes nothing. |

### `config`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `address` | String | Yes | Connection URI of the deployment, holding only the host and connection options (e.g., `mongodb+srv://cluster0.123456789.mongodb.net?authSource=admin`). A `mongodb://` host can include its port (e.g., `mongodb://123.4.567.89:27017`), a `mongodb+srv://` host can't. The plan rejects credentials in it, such as `user:password@` or a password option, as state and logs would hold them in plain text: set them in `user` and `password`. Credentials found in the live value, such as an address pasted in the dashboard, show as `***`, and the next apply replaces them with the configured value. |
| `user` | String | Yes | Database user to connect as. |
| `password` | String | Yes | Password of the database user. Write-only, see the note above. |
| `password_hash` | String | Read-only | Salted hash of `password`, used to detect when it changes. |
| `database` | String | No | Comma-separated list of databases to discover. If unset, every available database is discovered. |
| `batch_and_change_stream` | Bool | No | Also discovers the collections that can only be batch captured, such as views and time series collections, when the deployment supports change streams. Every collection is batch captured when the deployment doesn't support change streams. |
| `poll_schedule` | String | No | Default polling schedule of batch collections: a Go duration such as `5m` or `6h`, or a daily time in UTC such as `daily at 12:34Z`. Defaults to `24h`. |
| `network_tunnel` | Object | No | Connects through an SSH server acting as a bastion host. See [`config.network_tunnel`](#confignetwork_tunnel). |
| `advanced` | Object | No | Options for advanced users, rarely needed. See [`config.advanced`](#configadvanced). |

### `config.network_tunnel`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `ssh_forwarding` | Object | No | SSH server tunneling the connection to the deployment. |
| `ssh_forwarding.ssh_endpoint` | String | Yes | Endpoint of the SSH server, in the form `ssh://user@hostname[:port]`. The plan rejects a password in it, such as `user:password@`, as state and logs would hold it in plain text: authenticate with `private_key`. A password found in the live value shows as `***`. |
| `ssh_forwarding.private_key` | String | Yes | Private key used to connect to the SSH server. Write-only, see the note above. |
| `ssh_forwarding.private_key_hash` | String | Read-only | Salted hash of `private_key`, used to detect when it changes. |

### `config.advanced`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `max_await_time` | String | No | Maximum time to wait for new change stream events before returning an empty batch, as a Go duration such as `10s`. Defaults to `1s`. |
| `disable_pre_images` | Bool | No | Stops requesting pre-images, even when the deployment supports them and they are enabled for the collections. |
| `exclusive_collection_filter` | Bool | No | Adds a pipeline filter to the database change streams so they only match events of enabled bindings. Use it only with a small number of bindings. Cannot be set with `exclude_collections`. |
| `exclude_collections` | String | No | Comma-separated list of collections to exclude from the database change streams, each formatted as `database:collection`. Cannot be set with `exclusive_collection_filter`, nor list a collection with an enabled binding. |
| `skip_backfills` | String | No | Comma-separated list of collections not to backfill, each formatted as `database:collection`. Use `*:*` to skip the backfill of every change stream collection. Only applies to Change Stream Incremental bindings: naming another binding fails validation, while `*:*` skips only the change stream ones. |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `resource` | Object | Yes | MongoDB collection the binding captures. See [`bindings.resource`](#bindingsresource). |
| `target` | String | No | Estuary collection the data is captured into. A binding without it keeps its current collection. A new one gets the collection the connector discovery names, which `terraform plan` runs against the source (e.g., `acmeCo/mongodb/shop/users`), see [Bindings](../../../docs/guides/bindings.md). |
| `disable` | Bool | No | Stops capturing the collection while keeping the binding in the capture. |
| `backfill` | Object | No | Backfill control of the binding. See [`bindings.backfill`](#bindingsbackfill). |
| `name` | String | Read-only | Collection of the binding, shown in every plan so you can tell which binding changes. |

### `bindings.backfill`

Backfill control of the binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Raise it to backfill the binding: capture the whole collection again from the start. Starts at 1 and only goes up. Can't be raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |
| `strategy` | String | No | Overrides the capture's `backfill.strategy` for this binding: `incremental` captures again into the existing collection, `dataflowReset` resets the collection instead, backfilling its materializations and truncating their tables. Changing it alone publishes nothing. |

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `database` | String | Yes | Database of the captured collection. |
| `collection` | String | Yes | Name of the captured collection. |
| `capture_mode` | String | No | How the collection is captured: `Change Stream Incremental` (default), `Batch Snapshot` or `Batch Incremental`. |
| `cursor_field` | String | No | Field used as the cursor of batch capture modes, which require it. It should be indexed. With `Batch Incremental`, every document must have it and its values must strictly increase. |
| `poll_schedule` | String | No | Polling schedule of a batch binding, overriding `config.poll_schedule`. Same format, defaults to `config.poll_schedule`. |

## Data Source

`data "estuary_source_mongodb"` reads every live binding of a capture by name: the declared ones, those discovery (`auto_discover`) adds and those added in the dashboard. Terraform doesn't manage live bindings, so the resource neither stores them nor shows them in plans: read them here, for a capture of this configuration or one managed elsewhere:

```hcl
# A capture of this configuration
data "estuary_source_mongodb" "source" {
  name = estuary_source_mongodb.source.name
}

# A capture managed elsewhere
data "estuary_source_mongodb" "other" {
  name = "mongodb"
}
```

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name, as in the resource: the provider `tenant` is added in front and `/source-mongodb` at the end, unless they're already there. |
| `live_bindings` | List of Objects | Read-only | Every live binding of the capture, with the attributes of [`bindings`](#bindings). |

> [!NOTE]
> See [Bindings](../../../docs/guides/bindings.md) for when the data source is read and how to use it.
