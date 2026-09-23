data "ziti_proxy_v1_config" "test_v1_proxy_data" {
  name = "test.proxy.v1"
}

output "ziti_proxy_config" {
  value = data.ziti_proxy_v1_config.test_v1_proxy_data
}
