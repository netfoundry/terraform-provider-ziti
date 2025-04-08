data "ziti_intercept_v1_config" "test_v1_intercept_data" {
  name = "test.intercept.v1"
}

output "ziti_intercept_config" {
  value = data.ziti_intercept_v1_config.test_v1_intercept_data
}