data "ziti_interfaces_v1_config" "test_v1_interfaces_data" {
  name = "test.interfaces.v1"
}

output "ziti_interfaces_config" {
  value = data.ziti_interfaces_v1_config.test_v1_interfaces_data
}
