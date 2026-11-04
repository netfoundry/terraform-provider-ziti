# v2.2.1

## What's New

* Added `enroll_to_token_enabled`, `enroll_to_cert_enabled`, `enroll_name_claims_selector`, `enroll_attribute_claims_selector`, and `enroll_auth_policy_id` as optional attributes of `ziti_external_jwt_signer`, so JIT enrollment settings can be managed through the provider instead of being reset on every unrelated `apply`.
    * [Issue #58](https://github.com/netfoundry/terraform-provider-ziti/issues/58)

        * `enroll_to_token_enabled`, `enroll_to_cert_enabled`, `enroll_name_claims_selector`, and `enroll_auth_policy_id` are optional and computed: if a signer already has these set outside of Terraform (e.g. via the `ziti` CLI), a `terraform refresh`/`plan`/`apply` after upgrading picks up and preserves the existing values automatically, with no config changes required.
        * `enroll_attribute_claims_selector` has no server-side default and remains purely optional (not computed). If it's already set on a signer outside of Terraform, it will still be reset on the next unrelated `apply` unless it's also added to the resource's configuration with its current value. For an existing signer, `terraform import` it, then populate `enroll_attribute_claims_selector` in the config with the value returned by `ziti edge list ext-jwt-signers -j` before applying any further changes.


# v2.2.0

## What's New

* Added terraform provider code for Proxy v1 config
* Added terraform provider code for Interfaces v1 config
* Removed setting of connectTimeoutSeconds default value in dial_options schema
* Added missing validation on "Allowed Addresses" - Rejects protocol expression patterns like http:// https:// amqp:// jdbc://