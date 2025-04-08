# Author: Vinay Lakshmaiah
# Date:  04-Mar-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

# using provider values
# provider "ziti" {
#   username = var.ziti_username
#   password = var.ziti_password
#   host     = var.ziti_host
# }

## using env values
provider "ziti" {
//env variables ZITI_API_USERNAME, ZITI_API_PASSWORD and ZITI_API_HOST should be set.
}

resource "ziti_edge_router" "test_edge_router" {
  name     = "test_edge_router"
  tags = {
    cost = "test"
  }
  role_attributes = ["test1"]
  app_data = {
    "property1" = "test1"
    "property2" = "test2"
  }
  cost = 65
  is_tunnelerenabled = false
  no_traversal = true
}