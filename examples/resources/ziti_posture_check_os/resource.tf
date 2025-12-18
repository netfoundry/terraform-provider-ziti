resource "ziti_posture_check_os" "test_posture_check_os" {
  name            = "test_os"
  role_attributes = ["test"]
  operating_systems = [
    {
      type     = "Linux"
      versions = ["1.0.1"]
    }
  ]
}