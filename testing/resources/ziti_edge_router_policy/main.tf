# Author: Vinay Lakshmaiah
# Date:   18-Feb-2025

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

resource "ziti_edge_router_policy" "test1" {
  name            = "test1"
  edgerouterroles = ["#test1"]
  semantic        = "AllOf"
  identityroles   = ["#test1"]
}