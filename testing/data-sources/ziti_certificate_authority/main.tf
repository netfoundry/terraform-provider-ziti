# Author: Vinay Lakshmaiah
# Date:  24-Dec-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

provider "ziti" {
}

data "ziti_certificate_authority" "test_certificate_authority_data" {
  name = "test_certificate_authority"
}

output "ziti_ca" {
  value = data.ziti_certificate_authority.test_certificate_authority_data
}