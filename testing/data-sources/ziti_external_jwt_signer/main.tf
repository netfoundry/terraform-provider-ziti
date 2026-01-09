# Author: Vinay Lakshmaiah
# Date:  30-Dec-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

provider "ziti" {
}

data "ziti_external_jwt_signer" "test_external_jwt_signer_data" {
  name = "test_external_jwt_signer"
}

output "ziti_jwt_signer" {
  value = data.ziti_external_jwt_signer.test_external_jwt_signer_data
}