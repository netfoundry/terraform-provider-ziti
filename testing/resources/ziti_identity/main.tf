# Author: Vinay Lakshmaiah
# Date:   27-Feb-2025

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

resource "ziti_identity" "test1" {
  name            = "test1"
  role_attributes = ["test2"]
  tags = {
    value = "test"
  }
  app_data = {
    "property1" = "test1"
    "property2" = "test2"
  }
  default_hosting_cost = 65
  is_admin = true
}