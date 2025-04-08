# Author: Vinay Lakshmaiah
# Date:   06-Mar-2025

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
  name     = "test.intercept.v1_service"
  addresses      = ["test.com"]
  port_ranges = [
    {
      low  = 80
      high = 443
    }
  ]
  protocols = ["tcp", "udp"]
  source_ip = "10.10.10.10"
}

resource "ziti_service" "test_service" {
  name    = "test_service"
  configs = [ziti_intercept_v1_config.test_intercept_v1_config.id]
}