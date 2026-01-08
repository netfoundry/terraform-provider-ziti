data "ziti_external_jwt_signer" "test_external_jwt_signer_data" {
  name = "test_external_jwt_signer"
}

resource "ziti_auth_policy" "test_auth_policy" {
  name      = "test_auth_policy"
  primary   = {
    cert = {
      allowed             = true
      allow_expired_certs = true
    }
    ext_jwt = {
      allowed         = true
      allowed_signers = [data.ziti_external_jwt_signer.test_external_jwt_signer_data.id]
    }
    updb = {
      allowed                  = true
      lockout_duration_minutes = 10
    }
  }
  secondary = {
    require_totp = true
    jwt_signer   = data.ziti_external_jwt_signer.test_external_jwt_signer_data.id
  }
  tags = {
    cost = "test"
  }
}