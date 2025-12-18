resource "ziti_posture_check_domains" "test_posture_check_domains" {
  name            = "test_domains"
  role_attributes = ["test"]
  domains         = ["test.com"]
}