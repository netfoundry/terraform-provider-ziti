# Author: Vinay Lakshmaiah
# Date:   17-Mar-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

provider "ziti" {
}

data "ziti_service_edge_router_policy" "test_service_er_policy_data" {
  name = "test_service_er_policy"
}

output "ziti_serp" {
  value = data.ziti_service_edge_router_policy.test_service_er_policy_data
}