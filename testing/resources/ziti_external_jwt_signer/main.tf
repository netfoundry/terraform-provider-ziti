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
//env variables ZITI_API_USERNAME, ZITI_API_PASSWORD and ZITI_API_HOST should be set.
}

resource "ziti_external_jwt_signer" "test_external_jwt_signer" {
  name              = "test_external_jwt_signer"
  scopes            = ["test"]
  audience          = "test"
  issuer            = "test1"
  enabled           = false
  kid               = "test1"
  external_auth_url = ""
  target_token      = "ACCESS"
  cert_pem          = "-----BEGIN CERTIFICATE-----\naaaaaaaaa\nbbbbbbb\n-----END CERTIFICATE-----\n"
  tags = {
    cost = "test"
  }
}