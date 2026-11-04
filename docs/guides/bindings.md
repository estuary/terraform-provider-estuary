---
page_title: "Bindings"
description: |-
  How the provider manages the bindings of captures and materializations, and their field selection.
---

# Bindings

A binding links one resource, such as a MongoDB collection or a PostgreSQL table, to one Estuary collection: the `target` of a capture binding, or the `source` of a materialization binding. The collection identifies the binding, and the read-only `name` shows it in plans. Changing a binding's collection removes the binding and adds a new one.

**Bindings you don't declare.** Estuary adds bindings through discovery (`auto_discover`) on a capture and `source.capture` on a materialization, and the dashboard can add them too. `keep_unmanaged_bindings` decides what Terraform does with them:

| `keep_unmanaged_bindings` | Terraform manages | Undeclared bindings |
|---|---|---|
| `true` (default) | The declared bindings | Kept and not shown in plans. |
| `false` | Every binding | Removed by the next apply, after a plan warning listing them. Can't be used with `auto_discover` or `source.capture`, as each apply would remove what Estuary adds back. |

**Default capture target.** A capture binding without `target` keeps its current collection. A new one gets the collection the connector's discovery names, such as `acmeCo/mongodb/shop/users` for `shop.users` in `acmeCo/mongodb/source-mongodb`. That discovery runs during `terraform plan`, so the source must be reachable. Set `target` to skip it.

**Default destination schema.** A materialization binding without its schema (`schema`, or `namespace` for Iceberg) keeps its current one. A new one gets the schema `target_naming` names, as in the dashboard, else the endpoint default such as `config.schema`. Changing `target_naming` only names bindings added from then on.

**Moving a binding to another table.** Changing an existing binding's schema or table makes the connector materialize it into a new table and leave the old one in place. Raise the binding's `backfill.trigger` to fill the new table from the start of its collection. The apply fails if the new table already exists in the destination.

**Bindings Estuary adds.** Creating a capture with `auto_discover.add_new_bindings = true` adds a binding for every discovered resource you didn't declare, and later discoveries keep adding new ones. A materialization with `source.capture` gets a binding for every collection of that capture you didn't declare. Those materialization bindings get only `source.delta_updates`, `source.fields_recommended` and `target_naming`: declare a binding to give it its own settings.

**Removing a binding.** With `add_new_bindings = true` or `source.capture`, Estuary may add a removed binding back. Set `disable = true` instead to stop it for good.

**Taking over a binding.** Declaring a binding that already exists in Estuary updates it in place. The plan shows it as added, and a warning lists the fields the apply changes.

```hcl
# Manage every binding, without discovery
keep_unmanaged_bindings = false

bindings = [
  { resource = { database = "shop", collection = "users" } },
  { resource = { database = "shop", collection = "orders" } },
]
```

```hcl
# Let discovery add new collections, managing only the declared ones
auto_discover = {
  add_new_bindings = true
}

bindings = [
  { resource = { database = "shop", collection = "users" } },
  {
    resource = { database = "shop", collection = "sessions" }
    disable  = true # stop capturing it; removing it would let discovery add it back
  },
]
```

```hcl
# Follow a capture, declaring only the bindings that need their own settings
resource "estuary_materialization_postgres" "shop" {
  # ...
  source = {
    capture            = "acmeCo/mongodb/source-mongodb"
    fields_recommended = "unlimited"
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/orders"
      resource = { table = "orders" }
      fields   = { recommended = "unlimited", require = ["flow_document"] }
    },
  ]
}
```

## Every Live Binding

A capture resource only holds the bindings you declare. Its data source, such as `data "estuary_source_mongodb"`, lists every live binding of a capture in `live_bindings`, including those discovery or the dashboard added:

```hcl
data "estuary_source_mongodb" "shop" {
  name = estuary_source_mongodb.shop.name # or the name of a capture managed elsewhere
}

resource "estuary_materialization_postgres" "shop" {
  # ...
  bindings = [
    for binding in data.estuary_source_mongodb.shop.live_bindings : {
      source   = binding.target
      resource = { table = binding.resource.collection }
    } if binding.disable != true
  ]
}
```

- Empty live values are left out, so test optional fields like `binding.disable != true`.
- When the capture has changes, the data source is read after the capture's apply, so everything built from it shows as known after apply.
- Bindings that discovery adds later show up on the next plan.
- Reading fails when the capture doesn't exist.
- Terraform reads every declared data source, so declare one only where something uses it.

## Field Selection

A materialization binding's `fields` chooses which collection fields it materializes. Omit it to keep the binding's live selection. A new binding without `fields` gets Estuary's default, `recommended = "unlimited"`.

| Attribute | Description |
|---|---|
| `recommended` | Required when `fields` is set. Depth of the fields selected by default: `"0"` selects only the root document and required fields such as the collection key, `"1"` the top-level fields, `"2"` the second level too, and so on. `"unlimited"` selects every field. |
| `require` | Fields always materialized. |
| `exclude` | Fields never materialized. |
| `group_by` | Fields keying the table instead of the collection key. Only scalar fields (boolean, integer or string) can be keys. Changing them backfills the binding. |

A materialization's `source.fields_recommended` sets `recommended` for the bindings Estuary adds.

```hcl
{
  source   = "acmeCo/mongodb/shop/orders"
  resource = { table = "orders_slim" }

  fields = {
    recommended = "0"
    require     = ["customer_id", "status", "total"]
  }
}
```

> **Note:** `flow_document` holds the full source document. With standard updates it's always materialized, as the connector reads it back to merge updates, unless the connector's `no_flow_document` advanced option is set. With `delta_updates = true` it's only selected with `recommended = "0"`: add it to `require` to keep it at other depths.
