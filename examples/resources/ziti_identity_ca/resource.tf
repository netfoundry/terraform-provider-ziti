resource "ziti_identity_ca" "test_identity_ca" {
  name                 = "test_identity_ca"
  ottca                = data.ziti_certificate_authority.test_certificate_authority.id
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

data "ziti_certificate_authority" "test_certificate_authority" {
  name = "test_ca"
}

output "ziti_identity_token" {
  value     = ziti_identity_ca.test_identity_ca.enrollment_token
  sensitive = true
}