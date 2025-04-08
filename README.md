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

The Ziti provider offers different ways of providing credentials for authentication.
The following methods are supported:

* Static credentials
* Environment variables


#### Static credentials

Default static credentials can be provided by adding the `username`, `password` and `host`:

```terraform
provider "ziti" {
    username  = "zitiuser"
    password  = "zitipassword"
    host      = ["https://localhost:443/edge/management/v1"]
}
```

#### Environment Variables

You can provide your credentials for the default connection via the `ZITI_API_USERNAME`, `ZITI_API_PASSWORD` and `ZITI_API_HOST`,
environment variables, representing your user, password and domain/host respectively.

```terraform
provider "ziti" {
}
```


## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements)).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.


### Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.8+
- [Go](https://golang.org/doc/install) >= 1.21+


### Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the `go install` command:
```sh
$ go install
```


### Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```
go get github.com/author/dependency
go mod tidy
```