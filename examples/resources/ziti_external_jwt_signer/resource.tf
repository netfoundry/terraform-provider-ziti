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

  # Optional: JIT enrollment settings. Omit any of these to leave them
  # unmanaged by this resource (the controller's own default applies).
  enroll_to_token_enabled          = true
  enroll_to_cert_enabled           = false
  enroll_name_claims_selector      = "preferred_username"
  enroll_attribute_claims_selector = "groups"
  enroll_auth_policy_id            = "default"

  tags = {
    cost = "test"
  }
}