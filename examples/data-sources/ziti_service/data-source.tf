data "ziti_service" "test_service_data" {
  name = "test_service"
}

output "ziti_srv" {
  value = data.ziti_service.test_service_data
}