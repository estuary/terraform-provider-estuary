# estuary_source_mysql_batch and estuary_source_redshift_batch

[Back to the provider documentation](../../../README.md)

These resources manage the batch SQL capture connectors for [MySQL](https://docs.estuary.dev/reference/Connectors/capture-connectors/MySQL/mysql-batch/) and [Amazon Redshift](https://docs.estuary.dev/reference/Connectors/capture-connectors/redshift-batch/). Each binding polls one table, view or custom query on a schedule and captures its rows into an Estuary collection. A binding with cursor columns only reads the rows past the last values it saw; one without reads the whole table at each poll.

| Resource | Source | Default image | Connector name |
|---|---|---|---|
| `estuary_source_mysql_batch` | MySQL | `ghcr.io/estuary/source-mysql-batch:v1` | `source-mysql-batch` |
| `estuary_source_redshift_batch` | Amazon Redshift | `ghcr.io/estuary/source-redshift-batch:v1` | `source-redshift-batch` |

Both take the same attributes, except that Redshift requires `config.database` and each has its own `config.advanced` TLS options. To capture MySQL changes as they happen, use the change data capture of [`estuary_source_mysql`](../sourcemysql/README.md) instead, which reads the binary log.

## Table of Contents

- [Example Usage](#example-usage)
- [Importing](#importing)
- [Attributes](#attributes)
  - [`auto_discover`](#auto_discover)
  - [`backfill`](#backfill)
  - [`config`](#config)
  - [`config.network_tunnel`](#confignetwork_tunnel)
  - [`config.advanced`](#configadvanced)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
- [Data Sources](#data-sources)

## Example Usage

A MySQL capture of an incremental table and a small table read in full every night, with auto discovery adding every new table of the `shop` schema:

```hcl
variable "mysql_password" {
  type      = string
  sensitive = true
}

resource "estuary_source_mysql_batch" "shop" {
  name = "shop"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    address  = "mysql.example.com:3306"
    user     = "flow_capture"
    password = var.mysql_password

    advanced = {
      poll             = "6h"
      discover_schemas = ["shop"]
      sslmode          = "verify_identity"
    }
  }

  bindings = [
    {
      resource = {
        name   = "orders"
        schema = "shop"
        table  = "orders"
        cursor = ["updated_at"]
        poll   = "15m"
      }
    },
    {
      resource = {
        name   = "countries"
        schema = "shop"
        table  = "countries"
        poll   = "daily at 02:00Z"
      }
      target = "acmeCo/shop/countries"
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the capture `acmeCo/shop/source-mysql-batch`. The `orders` binding sets no `target`, so its data lands in the collection the connector discovery names, such as `acmeCo/shop/orders`.

A Redshift capture through an SSH bastion host, with a table and the result of a custom query:

```hcl
variable "redshift_password" {
  type      = string
  sensitive = true
}

variable "ssh_private_key" {
  type      = string
  sensitive = true
}

resource "estuary_source_redshift_batch" "warehouse" {
  name = "warehouse"

  config = {
    address  = "acme.abc123.us-east-1.redshift.amazonaws.com:5439"
    user     = "flow_capture"
    password = var.redshift_password
    database = "analytics"

    network_tunnel = {
      ssh_forwarding = {
        ssh_endpoint = "ssh://ec2-user@bastion.example.com:22"
        private_key  = var.ssh_private_key
        known_hosts  = "bastion.example.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample"
      }
    }
  }

  bindings = [
    {
      resource = {
        name   = "orders"
        schema = "public"
        table  = "orders"
        cursor = ["updated_at"]
      }
    },
    {
      resource = {
        name     = "revenue_by_day"
        template = "SELECT day, SUM(total) AS revenue FROM public.orders GROUP BY day"
      }
      target = "acmeCo/warehouse/revenue_by_day"
    },
  ]
}
```

> [!NOTE]
> `password`, `private_key`, `ssl_server_ca`, `ssl_client_cert` and `ssl_client_key` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Importing

```bash
terraform import estuary_source_mysql_batch.shop acmeCo/shop/source-mysql-batch
```

```bash
terraform import estuary_source_redshift_batch.warehouse acmeCo/warehouse/source-redshift-batch
```

Secrets and bindings aren't imported, see [Existing Pipelines](../../../docs/guides/existing-pipelines.md). A configuration that matches the live endpoint and leaves `password` out plans no changes, and applies keep the password Estuary holds. Once the configuration sets a secret or changes `config`, the apply publishes the endpoint with the secrets the configuration sets and no others, so set all of them in that change.

`config.network_tunnel.ssh_forwarding` requires its `private_key`, so a capture behind an SSH tunnel always takes that second path: the first plan after its import shows the `password_hash` and `private_key_hash` attributes, and the apply publishes the endpoint with the configured secrets and leaves the bindings as they are.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name (e.g., `shop`). The provider `tenant` is added in front and the connector name at the end, such as `/source-mysql-batch`, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the capture was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the capture. |
| `data_plane` | String | No | Data plane the capture runs in (e.g., `ops/dp/public/example-data-plane`), defaulting to the provider `data_plane` when the capture is created. Estuary can't move a live capture, so setting another data plane than it runs in fails the plan. See [Use Several Data Planes](../../../docs/guides/multiple-environments.md#use-several-data-planes). |
| `image` | String | No | Connector image. Defaults to the resource's image in the table above, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `auto_discover` | Object | No | Periodically checks the database for new tables and schema changes. See [`auto_discover`](#auto_discover). |
| `config` | Object | Yes | Connection settings of the database. See [`config`](#config). |
| `bindings` | Set of Objects | Yes | Polled tables, views and queries, one per binding. Their order doesn't matter, as Estuary identifies a binding by its resource path, so plans show only the bindings added, removed or changed. See [`bindings`](#bindings). |
| `shards` | Dynamic | No | Shard settings of the capture, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../docs/guides/task-settings.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `auto_discover`. Defaults to `true`. A provider control Estuary never sees. See [Bindings](../../../docs/guides/bindings.md). |
| `backfill` | Object | No | Backfill control of the whole capture. See [`backfill`](#backfill). |

### `auto_discover`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `add_new_bindings` | Bool | No | Adds a binding for each table found in the database that isn't declared, when the capture is created and on each later discovery. Terraform doesn't manage those bindings; declare a binding to manage it. |
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
| `address` | String | Yes | Host of the database server, as `host` or `host:port` (e.g., `mysql.example.com:3306` or `acme.abc123.us-east-1.redshift.amazonaws.com:5439`). The plan rejects credentials in it, such as `user:password@` or a password option, as state and logs would hold them in plain text: set them in `user` and `password`. Credentials found in the live value show as `***`. |
| `user` | String | Yes | Database user to connect as. The connector's own default is `flow_capture`. |
| `password` | String | To create the capture | Password of the database user. The connector requires it; an imported capture can leave it out to keep the password Estuary holds. Write-only, see the note above. |
| `password_hash` | String | Read-only | Salted hash of `password`, used to detect when it changes. |
| `database` | String | Yes, Redshift only | Database to capture from. The connector's own default is `dev`. `estuary_source_mysql_batch` doesn't have it: see `advanced.dbname`. |
| `network_tunnel` | Object | No | Connects through an SSH server acting as a bastion host. See [`config.network_tunnel`](#confignetwork_tunnel). |
| `advanced` | Object | No | Options for advanced users, rarely needed. See [`config.advanced`](#configadvanced). |

> [!TIP]
> The user only needs `SELECT` on the tables it captures. See [Creating a Capture User](https://docs.estuary.dev/reference/Connectors/capture-connectors/MySQL/mysql-batch/#creating-a-capture-user) for MySQL and [for Redshift](https://docs.estuary.dev/reference/Connectors/capture-connectors/redshift-batch/#creating-a-capture-user).

### `config.network_tunnel`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `ssh_forwarding` | Object | No | SSH server tunneling the connection to the database. |
| `ssh_forwarding.ssh_endpoint` | String | Yes | Endpoint of the SSH server, in the form `ssh://user@hostname[:port]`. The plan rejects a password in it, such as `user:password@`, as state and logs would hold it in plain text: authenticate with `private_key`. A password found in the live value shows as `***`. |
| `ssh_forwarding.private_key` | String | Yes | Private key used to connect to the SSH server. Write-only, see the note above. |
| `ssh_forwarding.private_key_hash` | String | Read-only | Salted hash of `private_key`, used to detect when it changes. |
| `ssh_forwarding.known_hosts` | String | No | Host keys of the SSH server the connector verifies it against, in OpenSSH `known_hosts` format, one per line, as `ssh-keyscan` prints them (e.g., `[bastion.example.com]:2222 ssh-ed25519 AAAA...`). |
| `ssh_forwarding.skip_host_key_verification` | Bool | No | Connects without verifying the host key of the SSH server. The dashboard hides it: prefer `known_hosts`. |

### `config.advanced`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `poll` | String | No | Default polling schedule of the bindings: a Go duration such as `5m` or `6h`, or a daily time in UTC such as `daily at 12:34Z`. Defaults to `24h`. |
| `discover_schemas` | List of Strings | No | Schemas whose tables are discovered. Unset, the tables of every schema are discovered. |
| `discover_views` | Bool | No | Also discovers views. Otherwise only tables are discovered. |
| `source_tag` | String | No | Value added as the `tag` property of the source metadata of each document. |
| `feature_flags` | String | No | Connector settings for edge cases and experimental features, see [Feature Flags](https://docs.estuary.dev/guides/advanced-usage/feature-flags/). Intended for Estuary internal use: change them only as directed by Estuary support. |

`estuary_source_mysql_batch` also has:

| Attribute | Type | Required | Description |
|---|---|---|---|
| `dbname` | String | No | Database to connect to. It rarely matters, as the connector discovers and captures from every database its user can access. |
| `sslmode` | String | No | TLS of the connection: `disabled` doesn't encrypt it, `preferred` encrypts it when the server supports TLS, `required` (default) encrypts it without verifying the server certificate, `verify_ca` also verifies the certificate against `ssl_server_ca`, and `verify_identity` verifies the hostname as well. See [TLS and Certificate Verification](https://docs.estuary.dev/reference/Connectors/capture-connectors/MySQL/mysql-batch/#tls-and-certificate-verification). |
| `ssl_server_ca` | String | With `verify_ca` | PEM-encoded certificate authority the server certificate is verified against. With `verify_identity`, the connector otherwise trusts public and managed database certificate authorities. Write-only, see the note above. |
| `ssl_server_ca_hash` | String | Read-only | Salted hash of `ssl_server_ca`, used to detect when it changes. |
| `ssl_client_cert` | String | No | PEM-encoded client certificate presented to the server with `ssl_client_key`, for mutual TLS. Write-only, see the note above. |
| `ssl_client_cert_hash` | String | Read-only | Salted hash of `ssl_client_cert`, used to detect when it changes. |
| `ssl_client_key` | String | No | PEM-encoded private key of `ssl_client_cert`. Write-only, see the note above. |
| `ssl_client_key_hash` | String | Read-only | Salted hash of `ssl_client_key`, used to detect when it changes. |

`estuary_source_redshift_batch` also has:

| Attribute | Type | Required | Description |
|---|---|---|---|
| `sslmode` | String | No | Overrides the `sslmode` connection parameter: `disable`, `allow`, `prefer`, `require`, `verify-ca` or `verify-full`. |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `resource` | Object | Yes | Table, view or query the binding polls. See [`bindings.resource`](#bindingsresource). |
| `target` | String | No | Estuary collection the data is captured into. A binding without it keeps its current collection. A new one gets the collection the connector discovery names, which `terraform plan` runs against the source (e.g., `acmeCo/shop/orders`), see [Bindings](../../../docs/guides/bindings.md). |
| `disable` | Bool | No | Stops capturing the table while keeping the binding in the capture. |
| `backfill` | Object | No | Backfill control of the binding. See [`bindings.backfill`](#bindingsbackfill). |
| `name` | String | Read-only | Collection of the binding, shown in every plan so you can tell which binding changes. |
| `resource_path` | List of String | Read-only | Resource path Estuary identifies the binding by, as in the dashboard. See [Bindings](../../../docs/guides/bindings.md). |

### `bindings.backfill`

Backfill control of the binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Set or raise it to backfill the binding: run its query again from the start, without its stored cursor. Setting it on a binding Estuary doesn't have yet, when the capture is created, or in the first apply after an import, backfills nothing, while setting it on an existing binding Terraform takes over backfills it. At least 1, can't be lowered, and can't be set or raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |
| `strategy` | String | No | Overrides the capture's `backfill.strategy` for this binding: `incremental` captures again into the existing collection, `dataflowReset` resets the collection instead, backfilling its materializations and truncating their tables. Changing it alone publishes nothing. |

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Unique name of the binding in the capture, which discovery sets to the table name. |
| `schema` | String | Unless `template` is set | Schema of the captured table. |
| `table` | String | Unless `template` is set | Name of the captured table. |
| `cursor` | List of Strings | No | Columns whose last values are kept between polls, so each poll only reads the rows after them (e.g., an `updated_at` column or an incrementing ID). Unset, each poll reads the whole table again. |
| `poll` | String | No | Polling schedule of the binding, overriding `config.advanced.poll`. Same format. |
| `template` | String | No | Query template the connector renders and runs instead of its own, to capture the result of a custom query. |

> [!TIP]
> See [Query Behaviors](https://docs.estuary.dev/reference/Connectors/capture-connectors/MySQL/mysql-batch/#query-behaviors) for how cursors and full refreshes work, and [Query Templates](https://docs.estuary.dev/reference/Connectors/capture-connectors/MySQL/mysql-batch/#query-templates) for the template syntax. The Redshift page has the same sections.

## Data Sources

`data "estuary_source_mysql_batch"` and `data "estuary_source_redshift_batch"` read every live binding of a capture by name: the declared ones, those discovery (`auto_discover`) adds and those added in the dashboard. Terraform doesn't manage live bindings, so the resources neither store them nor show them in plans: read them here, for a capture of this configuration or one managed elsewhere:

```hcl
# A capture of this configuration
data "estuary_source_mysql_batch" "source" {
  name = estuary_source_mysql_batch.source.name
}

# A capture managed elsewhere
data "estuary_source_redshift_batch" "other" {
  name = "warehouse"
}
```

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name, as in the resource: the provider `tenant` is added in front and the connector name at the end, unless they're already there. |
| `live_bindings` | List of Objects | Read-only | Every live binding of the capture, with the attributes of [`bindings`](#bindings). |

> [!NOTE]
> See [Bindings](../../../docs/guides/bindings.md) for when the data source is read and how to use it.

