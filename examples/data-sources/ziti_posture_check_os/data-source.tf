data "ziti_posture_check_os" "test_posture_check_os" {
  name = "test_os"
}

output "ziti_check_os" {
  value = data.ziti_posture_check_os.test_posture_check_os
}