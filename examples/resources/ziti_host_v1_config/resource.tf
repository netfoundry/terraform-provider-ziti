resource "ziti_host_v1_config" "simple_host" {
  name     = "simple_host.host.v1"
  address  = "localhost"
  port     = 5432
  protocol = "tcp"
}

resource "ziti_host_v1_config" "test_host" {
  name                     = "test.host.v1"
  forward_protocol         = true
  forward_address          = true
  forward_port             = true
  allowed_addresses        = ["localhost"]
  allowed_source_addresses = ["192.168.1.1"]
  allowed_protocols        = ["tcp", "udp"]
  forward_address_translations = [
    {
      from          = "192.168.1.0"
      to            = "172.16.4.0"
      prefix_length = 24
    },
    {
      from          = "10.0.0.0"
      to            = "10.10.0.0"
      prefix_length = 16
    }
  ]
  http_checks = [
    {
      url            = "https://localhost/health"
      method         = "GET"
      expect_status  = 200
      expect_in_body = "healthy"
      interval       = "5s"
      timeout        = "10s"
      actions = [
        {
          trigger  = "fail"
          duration = "10s"
          action   = "mark unhealthy"
        }
      ]

    }
  ]
  port_checks = [
    {
      address  = "localhost"
      interval = "5s"
      timeout  = "10s"
      actions = [
        {
          trigger  = "fail"
          duration = "10s"
          action   = "mark unhealthy"
        }
      ]

    }
  ]
  listen_options = {
    connect_timeout = "10s"
    precedence      = "default"
  }
  allowed_port_ranges = [
    {
      low  = 80
      high = 443
    }
  ]
}