## Option A — username/password (inline values)
provider "ziti" {
  host     = "https://<domain>:<port>/edge/management/v1"
  username = "ziti_session_username"
  password = "ziti_session_password"
}

## Option B — username/password (environment variables)
## Set ZITI_API_HOST, ZITI_API_USERNAME, ZITI_API_PASSWORD before running Terraform.
provider "ziti" {}

## Option C — identity file (mTLS, no username/password required)
## Point at a Ziti identity JSON file on the Terraform runner.
## Env equivalent: ZITI_API_IDENTITY_FILE
provider "ziti" {
  host          = "https://<domain>:<port>/edge/management/v1"
  identity_file = "/secure/terraform-automation.json"
}

## Option D — identity JSON inline (mTLS, sourced from a secret store)
## Useful when the identity file content is retrieved from Vault, AWS Secrets Manager, etc.
## Env equivalent: ZITI_API_IDENTITY_JSON
provider "ziti" {
  host          = "https://<domain>:<port>/edge/management/v1"
  identity_json = file("/secure/terraform-automation.json")
}

## Option E — explicit PEM material (mTLS)
## Supply cert, key, and CA directly (e.g. extracted from a secret store or generated inline).
provider "ziti" {
  host = "https://<domain>:<port>/edge/management/v1"
  cert = file("client.cert.pem")
  key  = file("client.key.pem")
  ca   = file("ca.pem")
}

## Option F — HA with username/password
## Authenticates against the first reachable controller; prefers the cluster leader.
provider "ziti" {
  username = "ziti_session_username"
  password = "ziti_session_password"
  hosts    = ["https://<domain1>:<port1>/edge/management/v1", "https://<domain2>:<port2>/edge/management/v1"]
}

## Option G — HA with identity file (mTLS)
provider "ziti" {
  identity_file = "/secure/terraform-automation.json"
  hosts         = ["https://<domain1>:<port1>/edge/management/v1", "https://<domain2>:<port2>/edge/management/v1"]
}
