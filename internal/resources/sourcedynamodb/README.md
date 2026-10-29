# estuary_source_dynamodb

[Back to the provider documentation](../../../README.md)

This resource manages an [Amazon DynamoDB](https://docs.estuary.dev/reference/Connectors/capture-connectors/amazon-dynamodb/) capture connector. Each binding captures one DynamoDB table into an Estuary collection, backfilling the table with parallel scans while reading its changes from DynamoDB Streams.

## Table of Contents

- [Example Usage](#example-usage)
- [Attributes](#attributes)
  - [`auto_discover`](#auto_discover)
  - [`backfill`](#backfill)
  - [`config`](#config)
  - [`config.credentials`](#configcredentials)
  - [`config.advanced`](#configadvanced)
  - [`bindings`](#bindings)
  - [`bindings.backfill`](#bindingsbackfill)
  - [`bindings.resource`](#bindingsresource)
- [Data Source](#data-source)

## Example Usage

A capture of two tables authenticated with an access key, with auto discovery adding every new table of the region:

```hcl
variable "aws_secret_access_key" {
  type      = string
  sensitive = true
}

resource "estuary_source_dynamodb" "shop" {
  name = "dynamodb"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    region = "us-east-1"

    credentials = {
      auth_type             = "AWSAccessKey"
      aws_access_key_id     = "AKIAXXXXXXXXXXXXXXXX"
      aws_secret_access_key = var.aws_secret_access_key
    }
  }

  bindings = [
    {
      resource = {
        table = "orders"
      }
    },
    {
      resource = {
        table = "customers"
      }
      target = "acmeCo/shop/customers"
    },
  ]
}
```

With `tenant = "acmeCo/"` set on the provider, this creates the capture `acmeCo/dynamodb/source-dynamodb`. The `orders` binding sets no `target`, so its data lands in the collection the connector discovery names, such as `acmeCo/dynamodb/orders`.

Authenticating with an IAM role, limiting the read capacity the backfill of a large table consumes:

```hcl
resource "estuary_source_dynamodb" "events" {
  name = "dynamodb/events"

  config = {
    region = "eu-west-1"

    credentials = {
      auth_type    = "AWSIAM"
      aws_role_arn = "arn:aws:iam::123456789012:role/estuary-dynamodb"
      aws_region   = "eu-west-1"
    }
  }

  bindings = [
    {
      resource = {
        table          = "events"
        rcu_allocation = 1000
      }
    },
  ]
}
```

Capturing from a DynamoDB-compatible API, such as DynamoDB Local, with advanced backfill and polling options:

```hcl
resource "estuary_source_dynamodb" "local" {
  name = "dynamodb/local"

  config = {
    region = "us-east-1"

    credentials = {
      auth_type             = "AWSAccessKey"
      aws_access_key_id     = "local"
      aws_secret_access_key = var.aws_secret_access_key
    }

    advanced = {
      endpoint          = "https://dynamodb.example.com"
      backfill_segments = 4
      scan_limit        = 500
      idle_backoff_max  = "1m"
    }
  }

  bindings = [
    {
      resource = {
        table = "orders"
      }
    },
  ]
}
```

> [!NOTE]
> `aws_secret_access_key` is a [write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) attribute: Terraform sends it to Estuary but never stores it in the state or shows it in plans. To still tell you when the secret changes, the provider keeps a read-only salted hash of it (`aws_secret_access_key_hash`) and warns in the plan when the value no longer matches. Write-only attributes require Terraform 1.11 or later.

> [!IMPORTANT]
> Every captured table needs DynamoDB Streams enabled with the **New and old images** view type (`NEW_AND_OLD_IMAGES`). Discovery skips the tables without it, and a binding of such a table fails when the capture starts.

## Attributes

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name (e.g., `dynamodb`). The provider `tenant` is added in front and `/source-dynamodb` at the end, unless they're already there. |
| `image` | String | No | Connector image. Defaults to `ghcr.io/estuary/source-dynamodb:v1`, and only its tag can change. See [Connector Versions](../../../README.md#connector-versions). |
| `auto_discover` | Object | No | Periodically checks the region for new tables and schema changes. See [`auto_discover`](#auto_discover). |
| `config` | Object | Yes | Region, authentication and advanced settings of the capture. See [`config`](#config). |
| `bindings` | List of Objects | Yes | Captured tables, one per binding. See [`bindings`](#bindings). |
| `live_bindings` | List of Objects | Read-only | Every live binding, the declared ones and those discovery (`auto_discover`) adds, with the attributes of [`bindings`](#bindings). Refreshed on each plan, and planned with the binding changes the apply publishes. See [Bindings Lifecycle](../../../README.md#bindings-lifecycle). |
| `shards` | Dynamic | No | Shard settings of the capture, with Estuary's own keys, e.g. `{ logLevel = "debug", disable = true }`. See [Shard Settings](../../../README.md#shard-settings). |
| `keep_unmanaged_bindings` | Bool | No | Leaves the live bindings your configuration doesn't declare, such as those Estuary or the dashboard adds, untouched. `false` manages every binding, removing undeclared ones. Can't be `false` with `auto_discover`. Defaults to `true`. A provider control Estuary never sees. See [Bindings Lifecycle](../../../README.md#bindings-lifecycle). |
| `backfill` | Object | No | Backfill control of the whole capture. See [`backfill`](#backfill). |

> [!WARNING]
> Changing the resulting catalog name, through `name` or the provider `tenant`, destroys the capture and creates a new one.

> [!WARNING]
> With the default `keep_unmanaged_bindings = true`, Terraform never removes bindings it doesn't manage: those Estuary or the dashboard adds, including the ones kept when you remove `auto_discover`, stay in the capture and aren't shown in plans. Set `keep_unmanaged_bindings = false` to manage every binding and remove the undeclared ones; the plan lists them before the apply.

### `auto_discover`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `add_new_bindings` | Bool | No | Adds a binding for each table found in the region that isn't declared, when the capture is created and on each later discovery. Terraform doesn't manage those bindings; declare a binding to manage it. |
| `evolve_incompatible_collections` | Bool | No | Automatically updates the target collections when the source schema changes in a way they can't accept. |

> [!TIP]
> With `add_new_bindings = true`, a binding you remove from your configuration may be added back by the next discovery. To stop capturing a table, set `disable = true` on its binding instead.

### `backfill`

Backfill control of the whole capture, as the dashboard's **Backfill** button and **Backfill mode**. See [Backfill Control](../../../README.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Raise it to backfill every enabled binding of the capture, including those Terraform doesn't manage. Disabled bindings are skipped, as in the dashboard: they keep their counter. **Not Estuary's backfill counter**, but a provider control Estuary never sees: each raise raises each binding's counter by one from its live value. Starts at 1 and only goes up. |
| `strategy` | String | No | Default strategy of the bindings, which a binding's `backfill.strategy` overrides: `incremental` (default) captures again into the existing collection, `dataflowReset` resets the collection instead, backfilling its materializations and truncating their tables. Changing it alone publishes nothing. |

### `config`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `region` | String | Yes | AWS region of the DynamoDB tables (e.g., `us-east-1`). |
| `credentials` | Object | Yes | AWS authentication of the capture. See [`config.credentials`](#configcredentials). |
| `advanced` | Object | No | Options for advanced users, rarely needed. See [`config.advanced`](#configadvanced). |

### `config.credentials`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `auth_type` | String | Yes | Authentication method: `AWSAccessKey` or `AWSIAM`. Only the attributes of the selected method are used. |
| `aws_access_key_id` | String | With `AWSAccessKey` | AWS access key ID of the IAM user the capture authenticates as. |
| `aws_secret_access_key` | String | With `AWSAccessKey` | AWS secret access key of the IAM user. Write-only, see the note above. |
| `aws_secret_access_key_hash` | String | Read-only | Salted hash of `aws_secret_access_key`, used to detect when it changes. |
| `aws_role_arn` | String | With `AWSIAM` | ARN of the IAM role Estuary assumes to connect, see the [AWS IAM guide](https://docs.estuary.dev/guides/iam-auth/aws/). |
| `aws_region` | String | With `AWSIAM` | AWS region to authenticate in (e.g., `us-east-1`). |

The IAM user or role needs `ListTables` and `DescribeTable` on every table of the region, and `DescribeStream`, `Scan`, `GetRecords` and `GetShardIterator` on the captured tables and their streams. See the [connector prerequisites](https://docs.estuary.dev/reference/Connectors/capture-connectors/amazon-dynamodb/#prerequisites) for example policies.

### `config.advanced`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `endpoint` | String | No | AWS endpoint URI to connect to, for both DynamoDB and DynamoDB Streams. Use it to capture from a DynamoDB-compatible API that AWS doesn't provide. |
| `backfill_segments` | Number | No | Number of segments the backfill scan of each table is split into. Defaults to `100`. Has no effect once the backfill of a table has started. |
| `scan_limit` | Number | No | Maximum number of items each backfill scan request evaluates. By default, it's unlimited, except for tables whose read capacity, from `rcu_allocation` or else provisioned, is 100 or less, which use `100`. |
| `idle_backoff_max` | String | No | Maximum wait between polls of an open stream shard whose reads keep returning no records, as a Go duration between `1s` and `10m`. Defaults to `1s`. Higher values reduce billed DynamoDB Streams requests on idle shards, but delay the capture of new records by up to this wait. |

### `bindings`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `resource` | Object | Yes | DynamoDB table the binding captures. See [`bindings.resource`](#bindingsresource). |
| `target` | String | No | Estuary collection the data is captured into. A binding without it keeps its current collection. A new one gets the collection the connector discovery names, which `terraform plan` runs against the source (e.g., `acmeCo/dynamodb/orders`), see [Bindings Lifecycle](../../../README.md#bindings-lifecycle). |
| `disable` | Bool | No | Stops capturing the table while keeping the binding in the capture. |
| `backfill` | Object | No | Backfill control of the binding. See [`bindings.backfill`](#bindingsbackfill). |
| `name` | String | Read-only | Collection of the binding, shown in every plan so you can tell which binding changes. |

### `bindings.backfill`

Backfill control of the binding. See [Backfill Control](../../../README.md#backfill-control).

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `trigger` | Number | No | Raise it to backfill the binding: capture the whole table again from the start. **Not Estuary's backfill counter**, but a provider control Estuary never sees: each raise raises Estuary's counter by one from its live value. Starts at 1 and only goes up. Raising it on a disabled binding is an error: set `disable = false` in the same change. |
| `strategy` | String | No | Overrides the capture's `backfill.strategy` for this binding: `incremental` captures again into the existing collection, `dataflowReset` resets the collection instead, backfilling its materializations and truncating their tables. Changing it alone publishes nothing. |

### `bindings.resource`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `table` | String | Yes | Name of the captured table. |
| `rcu_allocation` | Number | No | Read capacity units the backfill of the table tries to consume. Defaults to the provisioned read capacity of the table, and is unlimited for tables without one, such as on-demand tables. |

## Data Source

`data "estuary_source_dynamodb"` reads the live bindings of a capture by name without managing it, such as a capture managed in another configuration or outside Terraform:

```hcl
data "estuary_source_dynamodb" "source" {
  name = "dynamodb"
}
```

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Capture name, as in the resource: the provider `tenant` is added in front and `/source-dynamodb` at the end, unless they're already there. |
| `live_bindings` | List of Objects | Read-only | Every live binding of the capture, with the attributes of [`bindings`](#bindings). |

> [!WARNING]
> For a capture managed in the same configuration, use the resource's `live_bindings` instead. A data source referencing the resource is read at apply whenever the capture has changes, so everything built from it shows as known after apply. Reading fails when the capture doesn't exist, rather than listing no bindings.
