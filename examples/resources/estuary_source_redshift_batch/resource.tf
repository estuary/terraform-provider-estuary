variable "redshift_password" {
  type      = string
  sensitive = true
}

variable "ssh_private_key" {
  type      = string
  sensitive = true
}

# Through an SSH bastion host, with a table and the result of a custom query.
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
