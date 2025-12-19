data "ziti_posture_check_mac_addresses" "test_posture_check_mac_addresses" {
  name = "test_mac_addresses"
}

output "ziti_check_mac" {
  value = data.ziti_posture_check_mac_addresses.test_posture_check_mac_addresses
}