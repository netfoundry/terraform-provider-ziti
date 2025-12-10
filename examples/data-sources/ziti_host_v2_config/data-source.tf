data "ziti_host_v2_config" "test_host_v2_data" {
  name = "test.host.v2"
}

output "ziti_v2_host_config" {
  value = data.ziti_host_v2_config.test_host_v2_data
}