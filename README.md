# Estuary Terraform Provider

<p align="center">
  <a href="https://estuary.dev"><img src="https://storage.googleapis.com/estuary-marketing-strapi-uploads/uploads//estuary_logo_a373037876/estuary_logo_a373037876.png" width="200" alt="Estuary"></a>
</p>
<p align="center">
    <b>Build millisecond-latency, scalable, future-proof data pipelines in minutes</b>
</p>
<div align="center">

[![Estuary's Flow Repo](https://img.shields.io/github/stars/estuary/flow?label=Estuary)](https://github.com/estuary/flow)
[![Subscribe to Estuary's YouTube Channel](https://img.shields.io/youtube/channel/subscribers/UCJ9JIjh7uaUdjcFR6xTkJXQ?label=Subscribe)](https://www.youtube.com/@estuarydev?sub_confirmation=1)
[![Join Estuary's Slack](https://img.shields.io/badge/Slack-Estuary-blue)](https://go.estuary.dev/slack)
[![Follow Estuary on LinkedIn](https://img.shields.io/badge/Follow_on-LinkedIn-lightblue)](https://www.linkedin.com/company/estuary-tech/)

</div>

<div align="center">

![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)
![Terraform Plugin Framework](https://img.shields.io/badge/terraform--plugin--framework-v1.19.0-844FBA?logo=terraform&logoColor=white)
![yaml.v3](https://img.shields.io/badge/yaml.v3-v3.0.1-lightgrey)
</div>

We are pleased to present the Estuary Terraform provider, a dedicated provider for managing Estuary objects using Infrastructure as Code (IaC). With it, you can manage your connectors, both captures (sources) and materializations.

Estuary is an ETL data pipeline platform that combines batch and real-time data. With Estuary, you can choose from hundreds of in-house low- and no-code connectors, transform and aggregate data, and power truly real-time applications.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Provider Configuration](#provider-configuration)
- [Resources](#resources)

## Prerequisites

| Requirement | Minimum Version |
|---|---|
| Terraform | >= 1.11 |
| Go (for local development) | >= 1.27 |
| `flowctl` CLI | Latest, available on `PATH` or set through `flowctl_path` |
| Estuary account | [Sign up for free](https://dashboard.estuary.dev) |

> [!WARNING]
> For now, `flowctl` is the only backend the provider supports for acting on the Estuary platform, so it must be installed on the host where Terraform runs.

Install `flowctl` by following the [official docs](https://docs.estuary.dev/concepts/flowctl/).

---

## Installation

Add the provider to your `terraform` block:

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

---

## Provider Configuration

Configure the provider once per Terraform root module. All attributes can be sourced from environment variables, which is the recommended approach for CI/CD pipelines and remote servers.

```hcl
provider "estuary" {
  token        = var.estuary_token
  tenant       = "acmeCo/"
  flowctl_path = "flowctl"
  backend      = "flowctl"
  data_plane   = "ops/dp/public/aws-eu-west-1-c2"
}
```

### Provider Attributes

| Attribute | Type | Required | Environment Variable | Description |
|---|---|---|---|---|
| `token` | String | Yes | `FLOW_AUTH_TOKEN` | Estuary Refresh or Access token, for CI/CD and remote environment that won't require temporary credentials we recommend using Refresh Tokens that can be generated in the Estuary > Admin > CLI-API [page](https://dashboard.estuary.dev/admin/api). Treat it as a secret — never hard-code in `.tf` files. |
| `tenant` | String | No | `ESTUARY_TENANT` | Default tenant prefix prepended to all resource names (e.g., `acmeCo/`). Enables consistent naming without repeating the org prefix in every resource. |
| `flowctl_path` | String | No | — | Absolute or relative path to the `flowctl` binary. Defaults to `flowctl` (must be on `PATH`). Useful only when the backend is `flowctl`. |
| `backend` | String | No | `ESTUARY_ BACKEND` | Target estuary backend engine. This parameter defines how the Terraform Provider will reach to Estuary. Available Backends: `flowctl`. | 
| `data_plane` | String | No | `ESTUARY_DATA_PLANE` | Target data plane for the managed connectors (e.g., `ops/dp/public/aws-eu-west-1-c2`). Leave unset for standard Estuary-managed deployments. |

> **Tip:** Use `FLOW_AUTH_TOKEN`, `ESTUARY_TENANT` and `ESTUARY_DATA_PLANE` as environment variables in your CI/CD system rather than storing tokens in variable files. See [Multi-Environment Best Practices](#multi-environment--multi-tenant-best-practices).

---

## Resources

Each resource manages one Estuary connector. See its documentation for examples and the full list of attributes.

| Resource | Type | Connector |
|---|---|---|
| [`estuary_source_mongodb`](internal/resources/capture/mongodb/README.md) | Capture | [MongoDB](https://docs.estuary.dev/reference/Connectors/capture-connectors/MongoDB/) |
