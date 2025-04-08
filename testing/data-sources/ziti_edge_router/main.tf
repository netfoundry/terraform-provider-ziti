# Author: Vinay Lakshmaiah
# Date:  18-Mar-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

provider "ziti" {
}

data "ziti_edge_router" "test_edge_router_data" {
  name = "test_edge_router"
}

output "ziti_er" {
  value = data.ziti_edge_router.test_edge_router_data
}