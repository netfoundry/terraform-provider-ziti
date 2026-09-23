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

data "ziti_proxy_v1_config" "test_v1_proxy_data" {
  name = "test.proxy.v1"
}

output "ziti_proxy_config" {
  value = data.ziti_proxy_v1_config.test_v1_proxy_data
}
