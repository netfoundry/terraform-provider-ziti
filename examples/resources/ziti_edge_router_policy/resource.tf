resource "ziti_edge_router_policy" "test1" {
  name            = "test1"
  edgerouterroles = ["#test1"]
  semantic        = "AllOf"
  identityroles   = ["#test1"]
}