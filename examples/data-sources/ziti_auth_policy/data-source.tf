data "ziti_auth_policy" "test_auth_policy_data" {
  name = "test_auth_policy"
}

output "ziti_auth" {
  value = data.ziti_auth_policy.test_auth_policy_data
}