resource "ziti_proxy_v1_config" "test_proxy_v1_config" {
  name      = "test.proxy.v1"
  port      = 8000
  protocols = ["tcp"]
  binding   = "transport"
}
