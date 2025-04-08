# Author: Vinay Lakshmaiah
# Date:   04-Mar-2025

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

resource "ziti_service_edge_router_policy" "test_service_er_policy" {
  name     = "test_service_er_policy"
  edgerouterroles      = ["#test"]
  serviceroles       = ["#test"]
  semantic = "AnyOf"
  tags = {
    cost = "test"
  }
}