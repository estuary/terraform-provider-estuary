# Estuary Terraform Provider

<p align="center">
  <a href="https://estuary.dev"><img src="https://storage.googleapis.com/estuary-marketing-strapi-uploads/uploads//estuary_logo_a373037876/estuary_logo_a373037876.png" width="200" alt="Estuary"></a>
</p>
<p align="center">
    <b>Build millisecond-latency, scalable, future-proof data pipelines in minutes</b>
</p>

<div align="center">

![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)
![Terraform Plugin Framework](https://img.shields.io/badge/terraform--plugin--framework-v1.19.0-844FBA?logo=terraform&logoColor=white)
![yaml.v3](https://img.shields.io/badge/yaml.v3-v3.0.1-lightgrey)
</div>

We are pleased to present the Estuary Terraform provider, a dedicated provider for managing Estuary objects using Infrastructure as Code (IaC). With it, you can manage your connectors, both captures (sources) and materializations.

Estuary is an ETL data pipeline platform that combines batch and real-time data. With Estuary, you can choose from hundreds of in-house low- and no-code connectors, transform and aggregate data, and power truly real-time applications.

## Documentation

The provider documentation is available on the [Terraform Registry](https://registry.terraform.io/providers/estuary/estuary/latest/docs): There you can find the provider configuration, resources and data source, specifications, also the usage guides guides.

We recommend starting from [Getting Started](https://registry.terraform.io/providers/estuary/estuary/latest/docs/guides/getting-started).

## Quick Start

### Requirements

| Requirement | Minimum Version |
|---|---|
| Terraform | >= 1.11 |
| Go (for local development) | >= 1.27 |
| `flowctl` CLI | Latest, available on `PATH` or set through `flowctl_path` |
| Estuary account | [Sign up for free](https://dashboard.estuary.dev) |

> [!WARNING]
> Currently, `flowctl` is the only backend the provider supports for acting on the Estuary platform, so it must be installed on the host where Terraform runs.

Install `flowctl` by following the [official docs](https://docs.estuary.dev/concepts/flowctl/).

---

### Installation

```hcl
terraform {
  required_version = ">= 1.11"

  required_providers {
    estuary = {
      source  = "estuary/estuary"
      version = "~> 0.1"
    }
  }
}
```

Then run:

```bash
terraform init
```

## Connectors

Connectors configured the same way share one page, with examples and every attribute.

### Captures

| Connector | Resources |
|---|---|
| [Amazon DynamoDB](internal/resources/sourcedynamodb/README.md) | `estuary_source_dynamodb` |
| [MongoDB](internal/resources/sourcemongodb/README.md) | `estuary_source_mongodb` |
| [MySQL](internal/resources/sourcemysql/README.md) | `estuary_source_aurora_mysql`, `estuary_source_rds_mysql` |
| [PostgreSQL](internal/resources/sourcepostgres/README.md) | `estuary_source_aurora_postgres`, `estuary_source_rds_postgres` |

### Materializations

| Connector | Resources |
|---|---|
| [Apache Iceberg](internal/resources/materializationiceberg/README.md) | `estuary_materialization_iceberg` |
| [Apache Iceberg Tables in Amazon S3 (delta updates)](internal/resources/materializations3iceberg/README.md) | `estuary_materialization_s3_iceberg` |
| [Dekaf](internal/resources/materializationdekaf/README.md) | `estuary_materialization_dekaf_generic`, `estuary_materialization_dekaf_bytewax`, `estuary_materialization_dekaf_clickhouse`, `estuary_materialization_dekaf_imply_polaris`, `estuary_materialization_dekaf_materialize`, `estuary_materialization_dekaf_risingwave`, `estuary_materialization_dekaf_singlestore`, `estuary_materialization_dekaf_startree`, `estuary_materialization_dekaf_tinybird` |
| [HTTP Webhook](internal/resources/materializationwebhook/README.md) | `estuary_materialization_webhook` |
| [MySQL](internal/resources/materializationmysql/README.md) | `estuary_materialization_mysql`, `estuary_materialization_aurora_mysql`, `estuary_materialization_rds_mysql` |
| [PostgreSQL](internal/resources/materializationpostgres/README.md) | `estuary_materialization_postgres`, `estuary_materialization_aurora_postgres`, `estuary_materialization_rds_postgres` |
| [Snowflake](internal/resources/materializationsnowflake/README.md) | `estuary_materialization_snowflake` |

## Development Guide

- [Prerequisites](#prerequisites)
- [Local Setup](#local-setup)
- [Project Structure](#project-structure)
- [Adding a Connector](#adding-a-connector)
- [Testing](#testing)
- [Documentation](#documentation)
- [Dependency Scanning](#dependency-scanning)
- [Submitting Changes](#submitting-changes)

### Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| [Go](https://go.dev/dl/) | >= 1.27 | Build and test the provider |
| [Terraform](https://developer.hashicorp.com/terraform/install) | >= 1.11 | Run the provider locally |
| [`flowctl`](https://docs.estuary.dev/concepts/flowctl/) | Latest | Backend the provider calls to act on Estuary |
| [mise](https://mise.jdx.dev) | Latest | Installs the pinned Trivy release for dependency scans |

### Local Setup

**1. Clone and build**

```bash
git clone https://github.com/estuary/terraform-provider-estuary.git
cd terraform-provider-estuary
go build -o terraform-provider-estuary .
```

**2. Use the local build**

Add a `dev_overrides` block to `~/.terraformrc`, so Terraform loads your binary instead of the Registry release:

```hcl
provider_installation {
  dev_overrides {
    "estuary/estuary" = "/absolute/path/to/terraform-provider-estuary"
  }
  direct {}
}
```

**3. Run it**

With overrides on, skip `terraform init` and run `terraform plan` or `terraform apply` from any configuration. Rebuild after every code change.

> [!TIP]
> Set `TF_LOG=DEBUG` to see the provider's logs, including the `flowctl` commands it runs.

### Project Structure

```text
internal/
├── catalog/              # Connector specs: Go structs mirroring each connector's catalog YAML
│   ├── capture/          # Source connectors
│   ├── materialization/  # Destination connectors
│   ├── common/           # Shared blocks (auth, endpoint, SSL mode)
│   └── spectest/         # Shared test suites every spec runs
├── flowctl/              # flowctl client: catalog reads, discovery and publication
├── provider/             # Provider configuration and resource registry
└── resources/
    ├── builder/          # Generic resource and data source built from a spec
    └── <connector>/      # One package per connector, with its README
examples/                 # HCL samples rendered into the Registry docs
templates/                # tfplugindocs templates and guides
```

### Adding a Connector

The steps below use the HTTP Webhook materialization as the reference. Captures follow the same steps under `capture/`.

**1. Define the spec** in `internal/catalog/materialization/webhook/spec.go`. Each field maps to a key of the connector's config, and its tags drive the Terraform schema:

```go
const Image string = "ghcr.io/estuary/materialize-webhook:v1"
const ConnectorName string = "materialize-webhook"

type Config struct {
	Address string   `yaml:"address" credentials:"forbidden" description:"Base URL the requests are sent to."`
	Headers *Headers `yaml:"headers,omitempty" description:"HTTP headers of the requests."`
}
```

| Tag | Effect |
|---|---|
| `yaml` | Key in the connector config; `omitempty` makes the attribute optional |
| `description` | Attribute description in the schema and the Registry docs (required) |
| `secret:"true"` | Write-only attribute, never stored in state |
| `credentials:"forbidden"` | The plan rejects values with embedded credentials |

**2. Create the resource** in `internal/resources/materializationwebhook/resource.go`:

```go
var New func() resource.Resource = builder.NewMaterialization[webhook.Config, webhook.Resource](builder.Connector{
	TypeName:      "materialization_webhook",
	Image:         webhook.Image,
	ConnectorName: webhook.ConnectorName,
	Description:   "Sends the documents of Estuary collections to an HTTP endpoint of your server.",
})
```

Captures use `builder.NewCapture`, and also expose a data source through `builder.NewCaptureDataSource`.

**3. Register it** in `internal/provider/registry.go`, keeping the list sorted by Terraform type name:

```go
var resources []func() resource.Resource = []func() resource.Resource{
	// ...
	materializationwebhook.New,
}
```

**4. Add tests and samples.** Put sample catalog specs under `testdata/`, and run the shared suite against them (see [Testing](#testing)).

**5. Document it.** Add the example, the connector `README.md` and a row in the [Connectors](#connectors) table (see [Documentation](#documentation)).

### Testing

Every spec runs the shared conformance suite, plus its own checks on the samples in `testdata/`:

```go
func TestSpec(test *testing.T) {
	spectest.RunMaterializationSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}
```

Run the checks before pushing:

```bash
go vet ./...
go test ./...

# A single package or test
go test ./internal/catalog/materialization/webhook/ -run TestSpec -v
```

`internal/provider/registry_test.go` fails when any attribute is missing a description.

### Documentation

The documentation comes from three places:

| Source | Becomes |
|---|---|
| `internal/resources/<connector>/README.md` | The connector's page on [docs.estuary.dev](https://docs.estuary.dev) |
| Schema descriptions, `examples/` and `templates/` | The Registry pages under `docs/`, generated by [`tfplugindocs`](https://github.com/hashicorp/terraform-plugin-docs) |
| `templates/guides/` | The Registry guides under `docs/guides/` |

Each new resource needs `examples/resources/estuary_<type>/resource.tf`, and each data source `examples/data-sources/estuary_<type>/data-source.tf`. Attribute descriptions live in the spec's `description` tags and in the connector README's tables, so change both together.

Never edit `docs/` by hand. Regenerate it:

```bash
go generate ./...
```

### Submitting Changes

1. Branch off `main` as `<your-name>/<type>/<topic>`, such as `jdoe/feat/bigquery-materialization`.
2. Write commit messages as `<type>: <summary>`, where the type is `feat`, `fix` or `chore`.
3. Before opening the pull request, check that:
   - [ ] `go vet ./...` and `go test ./...` pass
   - [ ] `go generate ./...` ran, and new resources have examples and a README
4. Open the pull request against `main`.
