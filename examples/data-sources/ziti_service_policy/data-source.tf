data "ziti_service_policy" "test_service_policy_data" {
  name = "test_service_policy"
}

output "ziti_sp" {
  value = data.ziti_service_policy.test_service_policy_data
}