# Author: Vinay Lakshmaiah
# Date:   26-Nov-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

provider "ziti" {
}

data "ziti_posture_check_domains" "test_posture_check_domains" {
  name = "test_domains"
}

output "ziti_check_domains" {
  value = data.ziti_posture_check_domains.test_posture_check_domains
}