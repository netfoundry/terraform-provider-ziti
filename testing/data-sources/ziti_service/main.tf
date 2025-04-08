# Author: Vinay Lakshmaiah
# Date:   18-Mar-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

provider "ziti" {
}

data "ziti_service" "test_service_data" {
  name = "test_service"
}

output "ziti_srv" {
  value = data.ziti_service.test_service_data
}