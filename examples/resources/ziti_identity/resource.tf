resource "ziti_identity" "test1" {
  name            = "test1"
  role_attributes = ["test2"]
  tags = {
    value = "test"
  }
  app_data = {
    "property1" = "test1"
    "property2" = "test2"
  }
  default_hosting_cost = 65
  is_admin             = true
}