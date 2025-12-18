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

resource "ziti_posture_check_domains" "test_posture_check_domains" {
  name            = "test_domains"
  role_attributes = ["test"]
  domains         = ["test.com"]
}