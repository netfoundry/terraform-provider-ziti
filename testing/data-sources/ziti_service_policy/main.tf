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

data "ziti_service_policy" "test_service_policy_data" {
  name = "test_service_policy"
}

output "ziti_sp" {
  value = data.ziti_service_policy.test_service_policy_data
}