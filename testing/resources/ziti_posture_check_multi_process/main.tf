# Author: Vinay Lakshmaiah
# Date:   17-Dec-2025

terraform {
  required_providers {
    ziti = {
      source = "netfoundry/ziti"
    }
  }
}

provider "ziti" {
}

resource "ziti_posture_check_multi_process" "test_posture_check_multi_process" {
  name              = "test_multi_process"
  role_attributes   = ["test"]
  semantic = "AnyOf"
  processes = [
    {
      path    = "/usr/bin"
      os_type = "Linux"
      hashes  = ["test"]
      signer_fingerprints = ["test"]
    },
    {
      path    = "/usr/bin"
      os_type = "macOS"
      hashes  = ["test"]
      signer_fingerprints = ["test"]
    }
  ]
}