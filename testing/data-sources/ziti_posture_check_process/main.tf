# Author: Vinay Lakshmaiah
# Date:   15-Dec-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

provider "ziti" {
}

data "ziti_posture_check_multi_process" "test_posture_check_multi_process" {
  name = "test_multi_process"
}

output "ziti_check_multi_process" {
  value = data.ziti_posture_check_multi_process.test_posture_check_multi_process
}