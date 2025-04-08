## using values inside provider
provider "ziti" {
  username = "ziti_session_username"
  password = "ziti_session_password"
  host     = "https://<domain>:<port>/edge/management/v1"
}

## using env values
provider "ziti" {
  //env variables ZITI_API_USERNAME, ZITI_API_PASSWORD and ZITI_API_HOST should be set.
}