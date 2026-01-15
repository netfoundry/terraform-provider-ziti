# Author: Vinay Lakshmaiah
# Date:   12-Jan-2026

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

resource "ziti_identity_updb" "test_identity_updb" {
  name                 = "test_identity_updb"
  updb_username        = "test_user"
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