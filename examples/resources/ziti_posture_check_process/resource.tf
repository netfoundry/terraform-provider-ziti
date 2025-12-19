resource "ziti_posture_check_process" "test_posture_check_process" {
  name            = "test_process"
  role_attributes = ["test"]
  process = {
    path               = "/usr/bin"
    os_type            = "Linux"
    hashes             = ["test"]
    signer_fingerprint = "test"
  }
}