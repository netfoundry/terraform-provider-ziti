data "ziti_edge_router_policy" "test1_data" {
  name = "test1"
}

output "ziti_erp" {
  value = data.ziti_edge_router_policy.test1_data
}