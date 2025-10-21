resource "ziti_edge_router" "test_edge_router" {
  name = "test_edge_router"
  tags = {
    cost = "test"
  }
  role_attributes = ["test1"]
  app_data = {
    "property1" = "test1"
    "property2" = "test2"
  }
  cost               = 65
  is_tunnelerenabled = false
  no_traversal       = true
}

output "ziti_router_token" {
  value     = ziti_edge_router.test_edge_router.enrollment_token
  sensitive = true
}