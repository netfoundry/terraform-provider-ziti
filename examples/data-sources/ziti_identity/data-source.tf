data "ziti_identity" "test_identity_data" {
  name = "test1"
}

output "ziti_iden" {
  value = data.ziti_identity.test_identity_data
}