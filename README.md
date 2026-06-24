<div align="center">
  <h1>Ziti Terraform Provider</h1>
</div>

-------------------------------------

The Ziti Terraform Provider allows you to manage and configure Ziti network as code using
[Terraform](https://www.terraform.io/).


## Getting started

Configuring [required providers](https://www.terraform.io/docs/language/providers/requirements.html#requiring-providers):

```terraform
terraform {
  required_providers {
    ziti = {
      source  = "netfoundry/ziti"
    }
  }
}
```


### Authentication

The Ziti provider supports two authentication methods: **username/password** and **certificate (mTLS)**. Credentials can be supplied as static values in the provider block or via environment variables.

#### Username/password — static credentials

```terraform
provider "ziti" {
  host     = "https://localhost:443/edge/management/v1"
  username = "zitiuser"
  password = "zitipassword"
}
```

#### Username/password — environment variables

Set `ZITI_API_HOST`, `ZITI_API_USERNAME`, and `ZITI_API_PASSWORD` before running Terraform:

```terraform
provider "ziti" {}
```

#### Certificate authentication (mTLS) — prerequisites

> **Note**
> 1. **Create an admin identity for Terraform.** Admin role grants full Management API access. The provider creates, updates, and deletes entities, so admin is required.
>
>    ```bash
>    ziti edge create identity terraform-automation \
>      --admin \
>      --role-attributes automation \
>      --jwt-output-file terraform-automation.jwt
>    ```
>
>    This produces `terraform-automation.jwt` — a **one-time enrollment token**, not yet a usable credential.
>
> 2. **Enroll the identity to produce the identity JSON.** Enrollment exchanges the OTT for a client certificate + private key (signed by the controller's edge signing CA) and bundles the controller CA chain into a single JSON file:
>
>    ```bash
>    ziti edge enroll terraform-automation.jwt --out terraform-automation.json
>    ```
>
> 3. **Confirm the identity's auth policy allows certificate auth** before using any of the mTLS options below.
>
>    ```bash
>    # Which auth policy is the identity using?
>    ziti edge list identities 'name="terraform-automation"' --output-json | jq '.data[].authPolicy'
>
>    # Inspect the policy — primary.cert.allowed must be true
>    ziti edge list auth-policies --output-json | jq '.data[] | {name, cert: .primary.cert}'
>    ```
>
>    If `primary.cert.allowed` is `false`, either move the identity to the `Default` policy or update the custom policy to allow cert auth.

#### Certificate authentication — identity file (mTLS)

Point at a Ziti identity JSON file on the Terraform runner. No username or password is required.
The environment variable equivalent is `ZITI_API_IDENTITY_FILE`.

```terraform
provider "ziti" {
  host          = "https://localhost:443/edge/management/v1"
  identity_file = "/secure/terraform-automation.json"
}
```

#### Certificate authentication — identity JSON inline (mTLS)

Useful when the identity JSON is retrieved at runtime from a secret store rather than stored as a file on the runner.
The environment variable equivalent is `ZITI_API_IDENTITY_JSON`.

```terraform
# Example: pull the identity JSON from AWS Secrets Manager
data "aws_secretsmanager_secret_version" "ziti" {
  secret_id = "terraform-automation/ziti-identity"
}

provider "ziti" {
  host          = "https://localhost:443/edge/management/v1"
  identity_json = data.aws_secretsmanager_secret_version.ziti.secret_string
}
```

#### Certificate authentication — explicit PEM material (mTLS)

Supply `cert`, `key`, and optionally `ca` directly (e.g. extracted from a secret store or generated inline).
When `ca` is provided the controller's server certificate is verified against it; otherwise TLS verification is skipped.

```terraform
provider "ziti" {
  host = "https://localhost:443/edge/management/v1"
  cert = file("client.cert.pem")
  key  = file("client.key.pem")
  ca   = file("ca.pem")
}
```

#### High-availability (HA)

The `hosts` list enables failover across multiple controllers. The provider authenticates against the first reachable controller and then re-authenticates against the cluster leader. Both username/password and certificate auth are supported.

```terraform
## HA with username/password
provider "ziti" {
  username = "zitiuser"
  password = "zitipassword"
  hosts    = [
    "https://controller1:443/edge/management/v1",
    "https://controller2:443/edge/management/v1",
  ]
}

## HA with certificate authentication
provider "ziti" {
  identity_file = "/secure/terraform-automation.json"
  hosts         = [
    "https://controller1:443/edge/management/v1",
    "https://controller2:443/edge/management/v1",
  ]
}
```

> **Security notes**
> - **The identity JSON and the extracted `client.key.pem` contain a private key.** Never commit them to version control. Store them in a secret manager (Vault, AWS Secrets Manager, etc.) and inject at runtime.
> - **Certificate lifetime / rotation.** Enrolled certificates have a finite validity. When a cert nears expiry, re-enroll (issue a fresh OTT and re-run step 2) or extend it via the controller. An expired client cert produces a TLS handshake failure at auth time.
> - **Least privilege.** This identity has full admin rights over the network. Scope its use to your automation pipeline and rotate/revoke it if exposed (`ziti edge delete identity terraform-automation`).


## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements)).

For local plugin development see [Local Plugin Development](#local-plugin-development) before building the provider.

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.


### Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.8+
- [Go](https://golang.org/doc/install) >= 1.21+


### Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the `go install` command:

   ```sh
   go install
   ```


### Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```
go get github.com/author/dependency
go mod tidy
```

## Local Plugin Development
Add the below snippet at ~/.terraformrc on your machine.

```
provider_installation {

  dev_overrides {
    # point to local go path for compiled binaries
    "netfoundry/ziti" = "/path/to/go/bin"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```

> **Note:** Do not run `terraform init` during local development.
