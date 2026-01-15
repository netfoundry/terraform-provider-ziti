resource "ziti_identity_updb" "test_identity_updb" {
  name                 = "test_identity_updb"
  updb_username        = "test_user"
  role_attributes      = ["test"]
  default_hosting_cost = 65
  is_admin             = true
  tags = {
    value = "test"
  }
  app_data = {
    "property1" = "test1"
    "property2" = "test2"
  }
}

output "ziti_identity_token" {
  value     = ziti_identity_updb.test_identity_updb.enrollment_token
  sensitive = true
}