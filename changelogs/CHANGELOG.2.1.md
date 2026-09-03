# v2.1.3

## What's New

* Removed the debug `fmt.Print` statements from every resource and data source; the session token is no longer written to the plugin's output stream, and payload/response dumps now go through `tflog` so they only appear when `TF_LOG` is enabled


# v2.1.2

## What's New

* Handle drift caused by listenOptions.connectTimeoutSeconds in host.v1
* Expose listenOptions.identity and listenOptions.connectTimeoutSeconds in host.v1 and host.v2. Updated by [@paradizelost](https://github.com/paradizelost)


# v2.1.1

## What's New

* Handled empty string field in host v1 config


# v2.1.0

## What's New

* Added support for certificate authentication and Ziti identity JSON config format in provider

    * [Issue #5](https://github.com/netfoundry/terraform-provider-ziti/issues/5) - Support certificate authentication and Ziti identity JSON config format in provider