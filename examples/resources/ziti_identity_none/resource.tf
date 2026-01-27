resource "ziti_identity_none" "test_identity_none" {
  name                 = "test_identity_none"
  role_attributes      = ["test"]
  default_hosting_cost = 35
  is_admin             = false
  tags = {
    value = "test"
  }
  app_data = {
    "property1" = "test1"
    "property2" = "test2"
  }
}