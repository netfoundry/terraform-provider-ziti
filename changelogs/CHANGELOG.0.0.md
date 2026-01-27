# v0.0.7

## What's New

* Added terraform provider code for host v2 config
* Added terraform provider code for OS Posture check
* Added terraform provider code for MFA Posture check
* Added terraform provider code for Windows domain check Posture check
* Added terraform provider code for MAC address check Posture check

    * [Issue #13](https://github.com/netfoundry/terraform-provider-ziti/issues/13) - Support for host.v2 configs


# v0.0.6

## What's New

* Handle state information when resources are deleted outside of terraform

    * [Issue #8](https://github.com/netfoundry/terraform-provider-ziti/issues/8) - Provider Behavior When Manual Changes Made
    

# v0.0.5

## What's New

* Return enrolment token when creating a new identity/router

    * [Issue #6](https://github.com/netfoundry/terraform-provider-ziti/issues/6) - Add enrollment token outputs to identity and router resources


# v0.0.4

## What's New

* Updated terraform provider ziti identity resource to ott type enrollment
* Added resource import examples and updated the docs