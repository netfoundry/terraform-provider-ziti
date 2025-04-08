# Author: Vinay Lakshmaiah
# Date:   03-Mar-2025

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

resource "ziti_service_policy" "test_service1" {
  name     = "test_service_policy"
  identityroles      = ["#test"]
  serviceroles       = ["#test"]
  posturecheckroles = ["#test"]
  semantic = "AnyOf"
  type     = "Dial"
  tags = {
    cost = "test"
  }
}