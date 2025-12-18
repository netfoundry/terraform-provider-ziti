resource "ziti_posture_check_mfa" "test_posture_check_mfa" {
  name             = "test_mfa"
  role_attributes  = ["test"]
  prompt_on_unlock = true
  prompt_on_wake   = true
  timeout_seconds  = -1
}