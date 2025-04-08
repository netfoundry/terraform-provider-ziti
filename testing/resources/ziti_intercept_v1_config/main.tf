# Author: Vinay Lakshmaiah
# Date:   05-Mar-2025

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
resource "ziti_intercept_v1_config" "test_intercept_v1_config" {
  name     = "test.intercept.v1"
  addresses      = ["test.com"]
  dial_options = {
    "connect_timeout_seconds" = "10"
    "identity" = "test"
  }
  port_ranges = [
    {
      low  = 80
      high = 443
    }
  ]
  protocols = ["tcp", "udp"]
  source_ip = "10.10.10.10"
}