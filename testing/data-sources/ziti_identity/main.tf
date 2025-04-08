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

data "ziti_identity" "test_identity_data" {
  name = "test1"
}

output "ziti_iden" {
  value = data.ziti_identity.test_identity_data
}