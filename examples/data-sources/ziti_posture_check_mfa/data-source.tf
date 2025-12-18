data "ziti_posture_check_mfa" "test_posture_check_mfa" {
  name = "test_mfa"
}

output "ziti_check_mfa" {
  value = data.ziti_posture_check_mfa.test_posture_check_mfa
}