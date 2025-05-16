resource "ziti_service_policy" "test_service_policy" {
  name              = "test_service_policy"
  identityroles     = ["#test"]
  serviceroles      = ["#test"]
  posturecheckroles = ["#test"]
  semantic          = "AnyOf"
  type              = "Dial"
  tags = {
    cost = "test"
  }
}