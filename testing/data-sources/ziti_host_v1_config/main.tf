# Author: Vinay Lakshmaiah
# Date:   19-Mar-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

provider "ziti" {
}

data "ziti_host_v1_config" "test_host_v1_data" {
  name = "test.host.v1"
}

output "ziti_intercept_config" {
  value = data.ziti_host_v1_config.test_host_v1_data
}