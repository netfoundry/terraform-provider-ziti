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
  //env variables ZITI_API_USERNAME, ZITI_API_PASSWORD and ZITI_API_HOST should be set.
}

data "ziti_intercept_v1_config" "test_v1_intercept_data" {
  name = "test.intercept.v1"
}

output "ziti_intercept_config" {
  value = data.ziti_intercept_v1_config.test_v1_intercept_data
}