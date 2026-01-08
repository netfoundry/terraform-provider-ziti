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