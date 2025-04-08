resource "ziti_service_edge_router_policy" "test_service_er_policy" {
  name            = "test_service_er_policy"
  edgerouterroles = ["#test"]
  serviceroles    = ["#test"]
  semantic        = "AnyOf"
  tags = {
    cost = "test"
  }
}