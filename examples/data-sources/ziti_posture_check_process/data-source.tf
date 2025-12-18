data "ziti_posture_check_process" "test_posture_check_process" {
  name = "test_process"
}

output "ziti_check_process" {
  value = data.ziti_posture_check_process.test_posture_check_process
}