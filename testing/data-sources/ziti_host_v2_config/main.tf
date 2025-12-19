# Author: Vinay Lakshmaiah
# Date:   10-Dec-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

data "ziti_host_v2_config" "test_host_v2_data" {
  name = "test.host.v2"
}

output "ziti_v2_host_config" {
  value = data.ziti_host_v2_config.test_host_v2_data
}