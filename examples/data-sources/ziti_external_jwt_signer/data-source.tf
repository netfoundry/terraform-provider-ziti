data "ziti_external_jwt_signer" "test_external_jwt_signer_data" {
  name = "test_external_jwt_signer"
}

output "ziti_jwt_signer" {
  value = data.ziti_external_jwt_signer.test_external_jwt_signer_data
}