# Author: Vinay Lakshmaiah
# Date:   12-Mar-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

provider "ziti" {
}

data "ziti_edge_router_policy" "test1_data" {
  name = "test1"
}

output "ziti_erp" {
  value = data.ziti_edge_router_policy.test1_data
}