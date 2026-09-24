resource "ziti_interfaces_v1_config" "test_interfaces_v1_config" {
  name       = "test.interfaces.v1"
  interfaces = ["eth0", "eth1"]
}
