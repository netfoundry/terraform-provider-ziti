# Author: Vinay Lakshmaiah
# Date:   05-Dec-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

provider "ziti" {
}

data "ziti_posture_check_os" "test_posture_check_os" {
  name = "test_os"
}

output "ziti_check_os" {
  value = data.ziti_posture_check_os.test_posture_check_os
}