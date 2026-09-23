# Author: Vinay Lakshmaiah
# Date:   23-Sep-2026

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

resource "ziti_proxy_v1_config" "test_proxy_v1_config" {
  name      = "test.proxy.v1"
  port      = 8000
  protocols = ["tcp"]
  binding   = "transport"
}
