# Author: Vinay Lakshmaiah
# Date:   14-Jan-2026

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

resource "ziti_identity_ca" "test_identity_ca" {
  name                 = "test_identity_ca"
  ottca                = data.ziti_certificate_authority.test_certificate_authority.id
  role_attributes      = ["test"]
  default_hosting_cost = 65
  is_admin             = true
  tags = {
    value = "test"
  }
  app_data = {
    "property1" = "test1"
    "property2" = "test2"
  }
}

data "ziti_certificate_authority" "test_certificate_authority" {
  name = "test_ca"
}