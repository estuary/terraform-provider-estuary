# MySQL Captures

[Back to the provider documentation](../../../README.md)

These resources manage MySQL capture connectors on Amazon. Each binding captures one table into an Estuary collection, using change data capture through the MySQL binary log: an initial backfill of the table contents, followed by its inserts, updates and deletes as they happen.

They're configured the same way. Pick the resource matching your database:

| Resource | Database | Default image | Connector name |
|---|---|---|---|
| `estuary_source_aurora_mysql` | [Amazon Aurora for MySQL](https://docs.estuary.dev/reference/Connectors/capture-connectors/MySQL/#amazon-aurora) | `ghcr.io/estuary/source-amazon-aurora-mysql:v3` | `source-amazon-aurora-mysql` |
| `estuary_source_rds_mysql` | [Amazon RDS for MySQL](https://docs.estuary.dev/reference/Connectors/capture-connectors/MySQL/amazon-rds-mysql/) | `ghcr.io/estuary/source-amazon-rds-mysql:v3` | `source-amazon-rds-mysql` |

## Table of Contents

- [Example Usage](#example-usage)
- [Attributes](#attributes)
  - [`auto_discover`](#auto_discover)
  - [`backfill`](#backfill)
  - [`config`](#config)
  - [`config.credentials`](#configcredentials)
  - [`config.discovery_filters`](#configdiscovery_filters)
  - [`config.network_tunnel`](#confignetwork_tunnel)
  - [`config.advanced`](#configadvanced)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
- [Data Source](#data-source)

## Example Usage

An Amazon Aurora capture of two tables with password authentication, with auto discovery adding every new table of the `shop` database:

```hcl
variable "mysql_password" {
  type      = string
  sensitive = true
}

resource "estuary_source_aurora_mysql" "shop" {
  name = "aurora"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    address      = "shop.cluster-123456789.us-east-1.rds.amazonaws.com:3306"
    user         = "flow_capture"
    history_mode = false

    credentials = {
      auth_type = "UserPassword"
      password  = var.mysql_password
    }

    discovery_filters = {
      include_schemas = ["shop"]
    }
  }

  bindings = [
    {
      resource = {
        namespace = "shop"
        stream    = "users"
      }
    },
    {
      resource = {
        namespace = "shop"
        stream    = "orders"
      }
      target = "acmeCo/shop/orders"
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the capture `acmeCo/aurora/source-amazon-aurora-mysql`. The `users` binding sets no `target`, so its data lands in the collection the connector discovery names, such as `acmeCo/aurora/shop/users`.

An Amazon RDS capture with AWS IAM authentication, capturing every change event without reducing them, with a high priority table and a table captured without a backfill:

```hcl
resource "estuary_source_rds_mysql" "events" {
  name = "rds/events"

  config = {
    address      = "events.123456789.us-east-1.rds.amazonaws.com:3306"
    user         = "flow_capture"
    timezone     = "America/New_York"
    history_mode = true

    credentials = {
      auth_type    = "AWSIAM"
      aws_region   = "us-east-1"
      aws_role_arn = "arn:aws:iam::123456789012:role/estuary-rds-capture"
    }
  }

  bindings = [
    {
      resource = {
        namespace = "events"
        stream    = "accounts"
        priority  = 10
      }
    },
    {
      resource = {
        namespace = "events"
        stream    = "audit_log"
      }
      backfill = {
        mode = "Only Changes"
      }
    },
  ]
}
```

Connecting through an SSH bastion host, verifying the server certificate:

```hcl
variable "ssh_private_key" {
  type      = string
  sensitive = true
}

resource "estuary_source_aurora_mysql" "private" {
  name = "aurora/private"

  config = {
    address      = "private.cluster-123456789.us-east-1.rds.amazonaws.com:3306"
    user         = "flow_capture"
    history_mode = false

    credentials = {
      auth_type = "UserPassword"
      password  = var.mysql_password
    }

    network_tunnel = {
      ssh_forwarding = {
        ssh_endpoint = "ssh://ec2-user@ec2-123-4-567-89.compute-1.amazonaws.com:22"
        private_key  = var.ssh_private_key
      }
    }

    advanced = {
      sslmode        = "verify_identity"
      skip_backfills = "shop.events"
      source_tag     = "aurora-private"
    }
  }

  bindings = [
    {
      resource = {
        namespace = "shop"
        stream    = "payments"
      }
    },
  ]
}
```

> [!NOTE]
> `password`, `private_key`, `ssl_server_ca`, `ssl_client_cert` and `ssl_client_key` are [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attributes: Terraform never stores them in state or shows them in plans, and keeps only a salted hash of each to detect changes. See [Secrets](../../../docs/guides/secrets.md).

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name (e.g., `aurora`). The provider `tenant` is added in front and the connector name of the resource (e.g., `/source-amazon-aurora-mysql`) at the end, unless they're already there. |
| `catalog_name` | String | Read-only | Catalog name the capture was created or imported under, with the tenant and connector name. Refreshes and deletions address it, so a provider `tenant` change replaces the capture. |
| `image` | String | No | Connector image. Defaults to the default image of the resource, see the table above, and only its tag can change. See [Connector Versions](../../../docs/guides/task-settings.md#connector-versions). |
| `auto_discover` | Object | No | Periodically checks the database for new tables and schema changes. See [`auto_discover`](#auto_discover). |
| `config` | Object | Yes | Connection, authentication and replication settings of the database. See [`config`](#config). |
| `bindings` | List of Objects | Yes | Captured tables, one per binding. See [`bindings`](#bindings). |
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
| `trigger` | Number | No | Raise it to backfill every enabled binding of the capture, including those Terraform doesn't manage. Disabled bindings are skipped. Starts at 1 and only goes up. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |
| `strategy` | String | No | Default strategy of the bindings, which a binding's `backfill.strategy` overrides: `incremental` (default) captures again into the existing collection, `dataflowReset` resets the collection instead, backfilling its materializations and truncating their tables. Changing it alone publishes nothing. |

### `config`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `address` | String | Yes | Host or `host:port` of the Aurora cluster's writer endpoint or of the RDS instance endpoint (e.g., `shop.cluster-123456789.us-east-1.rds.amazonaws.com:3306`). The port defaults to `3306`. The plan rejects credentials in it, such as `user:password@` or a password or token option, as state and logs would hold them in plain text. Credentials found in the live value show as `***`. |
| `user` | String | Yes | Database user to authenticate as, usually `flow_capture`. |
| `credentials` | Object | No | How the user authenticates. See [`config.credentials`](#configcredentials). Set it, or the deprecated `password`; the connector rejects a configuration with neither. |
| `password` | String | No | Deprecated form of `credentials` with `auth_type = "UserPassword"`, still accepted for existing captures. At most 32 characters. Write-only, see the note above. |
| `password_hash` | String | Read-only | Salted hash of `password`, used to detect when it changes. |
| `timezone` | String | No | Time zone of `DATETIME` columns, as an IANA name (e.g., `America/New_York`) or a `+HH:MM` offset. Leave it unset to use the database's `time_zone` system variable, which it overrides when set. |
| `history_mode` | Bool | Yes | Captures every change event as its own document instead of reducing them to the latest state of each row. |
| `discovery_filters` | Object | No | Restricts which tables discovery finds. See [`config.discovery_filters`](#configdiscovery_filters). |
| `network_tunnel` | Object | No | Connects through an SSH server acting as a bastion host. See [`config.network_tunnel`](#confignetwork_tunnel). |
| `advanced` | Object | No | Options for advanced users, rarely needed. See [`config.advanced`](#configadvanced). |

> [!IMPORTANT]
> On Aurora, point `address` at the cluster's writer endpoint. Aurora reader instances don't keep their own binary log, so the capture can't run against them.

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `auth_type` | String | Yes | Authentication method: `UserPassword`, `AWSIAM`, `GCPIAM` or `AzureIAM`. Only the attributes of the selected method are used. |
| `password` | String | With `UserPassword` | Password of the database user, at most 32 characters (a MySQL replication limit). Write-only, see the note above. |
| `password_hash` | String | Read-only | Salted hash of `password`, used to detect when it changes. |
| `aws_region` | String | With `AWSIAM` | AWS region of the cluster or instance (e.g., `us-east-1`). |
| `aws_role_arn` | String | With `AWSIAM` | ARN of the IAM role Estuary assumes to connect, see the [AWS IAM guide](https://docs.estuary.dev/guides/iam-auth/aws/). |
| `gcp_service_account_to_impersonate` | String | With `GCPIAM` | Email of the Google Cloud service account used for IAM authentication. |
| `gcp_workload_identity_pool_audience` | String | With `GCPIAM` | Audience of the workload identity pool, in the form `//iam.googleapis.com/projects/123456789012/locations/global/workloadIdentityPools/<pool>/providers/<provider>`. |
| `azure_client_id` | String | With `AzureIAM` | Client ID of the Azure app registration used for Microsoft Entra authentication. |
| `azure_tenant_id` | String | With `AzureIAM` | Azure tenant ID of the app registration. |

> [!TIP]
> With `AWSIAM`, the database user must authenticate through the `AWSAuthenticationPlugin` and IAM database authentication be enabled on the cluster or instance, see [AWS IAM](https://docs.estuary.dev/reference/Connectors/capture-connectors/MySQL/#aws-iam) for Aurora and [IAM Authentication](https://docs.estuary.dev/reference/Connectors/capture-connectors/MySQL/amazon-rds-mysql/#iam-authentication) for RDS. A fresh authentication token is generated for each connection, so there's no password to rotate.
>
> IAM authentication (`AWSIAM`, `GCPIAM` or `AzureIAM`) sends its token as the password, so it requires an encrypted connection: `advanced.sslmode` must be unset or one of `required`, `verify_ca` and `verify_identity`.

### `config.discovery_filters`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `include_schemas` | List of Strings | No | Only discovers tables in these databases. Combined with `advanced.discover_schemas`. |
| `exclude_schemas` | List of Strings | No | Never discovers tables in these databases. |
| `table_patterns` | List of Strings | No | Only discovers tables matching at least one of these glob patterns, using `*` and `?` as wildcards. A pattern with a `.` matches `database.table` (e.g., `shop.orders_*`), a pattern without one matches the table name in any database. |

> [!TIP]
> With auto discovery enabled, a table these filters stop matching is deactivated the next time discovery runs.

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
| `sslmode` | String | No | Whether connections use TLS and how strictly the server certificate is verified: `disabled`, `preferred`, `required`, `verify_ca` or `verify_identity`. Defaults to `required`. |
| `ssl_server_ca` | String | No | PEM-encoded CA certificate the server certificate must chain to. Required with `verify_ca`. Optional with `verify_identity`, which otherwise trusts public certificate authorities, the Amazon RDS CAs and Google Cloud SQL's shared CA. Rejected with other modes. Write-only, see the note above. |
| `ssl_server_ca_hash` | String | Read-only | Salted hash of `ssl_server_ca`, used to detect when it changes. |
| `ssl_client_cert` | String | No | PEM-encoded client certificate, for mutual TLS. Set it together with `ssl_client_key`, and not with `disabled`. Write-only, see the note above. |
| `ssl_client_cert_hash` | String | Read-only | Salted hash of `ssl_client_cert`, used to detect when it changes. |
| `ssl_client_key` | String | No | PEM-encoded private key of `ssl_client_cert`. Write-only, see the note above. |
| `ssl_client_key_hash` | String | Read-only | Salted hash of `ssl_client_key`, used to detect when it changes. |
| `skip_backfills` | String | No | Comma-separated list of tables not to backfill, each as `database.table`. |
| `backfill_chunk_size` | Number | No | Number of rows fetched in a single backfill query. Defaults to `50000`. |
| `skip_binlog_retention_check` | Bool | No | Skips the startup check for a dangerously short binary log retention. Only set it if you understand the risk of losing changes. |
| `node_id` | Number | No | Replication node ID of the capture, an unsigned 32-bit integer unique within the replication cluster. Derived from the capture name when unset. |
| `dbname` | String | No | Database to connect to, which doesn't limit what is captured. Defaults to `mysql`. |
| `statement_timeout` | String | No | Statement timeout of the connector's queries: `30s`, `1m`, `5m` or `30m`. No timeout when unset. |
| `discover_schemas` | List of Strings | No | Only discovers tables in these databases. Prefer `discovery_filters.include_schemas`. |
| `rediscovery_interval` | String | No | How often the running capture checks for schema changes and new tables, as a number and one unit (`ms`, `s`, `m` or `h`) from `1m` to `8760h`, such as `15m` or `1h`. Defaults to `15m`. |
| `source_tag` | String | No | Value added as the `tag` property of each document's source metadata. |
| `feature_flags` | String | No | Connector settings for edge cases and experimental features, see [Feature Flags](https://docs.estuary.dev/guides/advanced-usage/feature-flags/). Intended for Estuary internal use: change them only as directed by Estuary support. |
| `watermarks_table` | String | No | Deprecated. Only excludes the named table from discovery, kept for existing captures. Defaults to `flow.watermarks`. |
| `heartbeat_interval` | String | No | Deprecated and has no effect, kept for existing captures. Defaults to `60s`. |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `resource` | Object | Yes | Table the binding captures. See [`bindings.resource`](#bindingsresource). |
| `target` | String | No | Estuary collection the data is captured into. A binding without it keeps its current collection. A new one gets the collection the connector discovery names, which `terraform plan` runs against the source (e.g., `acmeCo/aurora/shop/users`), see [Bindings](../../../docs/guides/bindings.md). |
| `disable` | Bool | No | Stops capturing the table while keeping the binding in the capture. |
| `backfill` | Object | No | Backfill control of the binding. See [`bindings.backfill`](#bindingsbackfill). |
| `name` | String | Read-only | Collection of the binding, shown in every plan so you can tell which binding changes. |

### `bindings.backfill`

Backfill control of the binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Raise it to backfill the binding: capture the whole table again from the start. Starts at 1 and only goes up. Can't be raised on a disabled binding. See [Backfill Control](../../../docs/guides/backfills.md#backfill-control). |
| `strategy` | String | No | Overrides the capture's `backfill.strategy` for this binding: `incremental` captures again into the existing collection, `dataflowReset` resets the collection instead, backfilling its materializations and truncating their tables. Changing it alone publishes nothing. |
| `mode` | String | No | How the table is backfilled: `Normal` emits every change right away, `Precise` holds back changes to rows the backfill hasn't reached yet, `Only Changes` skips the backfill, and `Without Primary Key` backfills tables without a primary key. Unset lets the connector choose. Changing it requires raising `trigger`, or the capture's, in the same change. See [Backfill Modes](https://docs.estuary.dev/reference/backfilling-data/#resource-configuration-backfill-modes). |

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `namespace` | String | Yes | Database of the captured table. |
| `stream` | String | Yes | Name of the captured table. |
| `priority` | Number | No | Backfill priority. Bindings with the highest priority are backfilled completely before the others. Defaults to `0`, negative values backfill after the others. |
| `primary_key` | List of Strings | No | Columns used to order the backfill and merge changes, overriding the collection key. Rarely needed. |
| `advanced` | Object | No | Rarely needed options of the binding. |
| `advanced.additional_backfill_filter` | String | No | SQL filter clause applied to every backfill query of the table, so the rows it excludes are never backfilled. It can't be used with the `Precise` mode. Setting or changing it requires a backfill of the binding. Contact Estuary support before using it. |

## Data Source

`data "estuary_source_aurora_mysql"` and `data "estuary_source_rds_mysql"` read every live binding of a capture by name: the declared ones, those discovery (`auto_discover`) adds and those added in the dashboard. Terraform doesn't manage live bindings, so the resource neither stores them nor shows them in plans: read them here, for a capture of this configuration or one managed elsewhere:

```hcl
# A capture of this configuration
data "estuary_source_aurora_mysql" "source" {
  name = estuary_source_aurora_mysql.source.name
}

# A capture managed elsewhere
data "estuary_source_aurora_mysql" "other" {
  name = "aurora"
}
```

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name, as in the resource: the provider `tenant` is added in front and the connector name of the resource at the end, unless they're already there. |
| `live_bindings` | List of Objects | Read-only | Every live binding of the capture, with the attributes of [`bindings`](#bindings). |

> [!NOTE]
> See [Bindings](../../../docs/guides/bindings.md) for when the data source is read and how to use it.
