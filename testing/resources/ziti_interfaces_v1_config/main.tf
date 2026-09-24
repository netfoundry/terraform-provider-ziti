# Author: Vinay Lakshmaiah
# Date:   24-Sep-2026

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

provider "ziti" {
  //env variables ZITI_API_USERNAME, ZITI_API_PASSWORD and ZITI_API_HOST should be set.
}

resource "ziti_interfaces_v1_config" "test_interfaces_v1_config" {
  name       = "test.interfaces.v1"
  interfaces = ["eth0", "eth1"]
}
