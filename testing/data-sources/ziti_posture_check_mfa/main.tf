# Author: Vinay Lakshmaiah
# Date:   03-Dec-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

provider "ziti" {
}

data "ziti_posture_check_mfa" "test_posture_check_mfa" {
  name = "test_mfa"
}

output "ziti_check_mfa" {
  value = data.ziti_posture_check_mfa.test_posture_check_mfa
}