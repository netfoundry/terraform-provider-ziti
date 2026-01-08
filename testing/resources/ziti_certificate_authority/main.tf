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
//env variables ZITI_API_USERNAME, ZITI_API_PASSWORD and ZITI_API_HOST should be set.
}

resource "ziti_certificate_authority" "test_certificate_authority" {
  name                         = "ziti_certificate_authority"
  identityroles                = ["test"]
  is_autoca_enrollment_enabled = true
  cert_pem                     = "-----BEGIN CERTIFICATE-----\naaaaaaaaa\nbbbbbbb\n-----END CERTIFICATE-----\n"
  external_id_claim = {
    location        = "COMMON_NAME"
    matcher         = "ALL"
    parser          = "NONE"
    matchercriteria = "test"
    parsercriteria  = "test"
    index           = 10
  }
  tags = {
    cost = "test"
  }
}