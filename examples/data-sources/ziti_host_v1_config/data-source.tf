data "ziti_host_v1_config" "test_host_v1_data" {
  name = "test.host.v1"
}

output "ziti_intercept_config" {
  value = data.ziti_host_v1_config.test_host_v1_data
}