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