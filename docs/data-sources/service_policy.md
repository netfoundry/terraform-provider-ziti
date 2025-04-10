---
page_title: "ziti_service_policy Data Source - terraform-provider-ziti"
subcategory: ""
description: |-
  Ziti Service Policy Data Source
---

# ziti_service_policy (Data Source)

Ziti Service Policy Data Source

## Example Usage

```terraform
data "ziti_service_policy" "test_service_policy_data" {
  name = "test_service_policy"
}

output "ziti_sp" {
  value = data.ziti_service_policy.test_service_policy_data
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `id` (String) Identifier
- `name` (String) Name of the service policy

### Read-Only

- `identityroles` (List of String) Identity Roles
- `posturecheckroles` (List of String) Posture Check Roles
- `semantic` (String) Semantic Value
- `serviceroles` (List of String) Service Roles
- `tags` (Map of String) Service Policy Tags
- `type` (String) Service Policy Type
