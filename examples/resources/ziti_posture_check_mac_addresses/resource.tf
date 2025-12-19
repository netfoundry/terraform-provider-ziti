resource "ziti_posture_check_mac_addresses" "test_posture_check_mac_addresses" {
  name            = "test_mac_addresses"
  role_attributes = ["test"]
  mac_addresses   = ["00:1a:2b:3c:4d:5e"]
}