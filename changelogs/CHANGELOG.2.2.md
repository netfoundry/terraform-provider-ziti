# v2.2.1

## What's New

* Added `enroll_to_token_enabled`, `enroll_to_cert_enabled`, `enroll_name_claims_selector`, `enroll_attribute_claims_selector`, and `enroll_auth_policy_id` as optional attributes of `ziti_external_jwt_signer` (resource and data source), so JIT enrollment settings can be managed through the provider instead of being reset on every unrelated `apply`.
    * [Issue #58](https://github.com/netfoundry/terraform-provider-ziti/issues/58)

        * All five attributes are optional and computed: if a signer already has these set outside of Terraform (e.g. via the `ziti` CLI), a `terraform refresh`/`plan`/`apply` after upgrading picks up and preserves the existing values automatically, with no config changes required. Updates are still sent as a full PUT, but these attributes always carry their current value through the plan, so a full-replace update never resets them.

* Changed `scopes` on `ziti_external_jwt_signer` (resource and data source) from an ordered list to a set, removing a perpetual plan diff caused by the controller storing scopes in a different order than configured.


# v2.2.0

## What's New

* Added terraform provider code for Proxy v1 config
* Added terraform provider code for Interfaces v1 config
* Removed setting of connectTimeoutSeconds default value in dial_options schema
* Added missing validation on "Allowed Addresses" - Rejects protocol expression patterns like http:// https:// amqp:// jdbc://