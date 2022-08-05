resource "tencentcloud_cdn_domain" "foo" {
  domain       = "xxxx.com"
  service_type = "web"
  area         = "mainland"
  range_origin_switch = "off"

  rule_cache{
  	cache_time = 10000
  	no_cache_switch="on"
  	re_validate="on"
  }

  request_header{
  	switch = "on"

  	header_rules {
  		header_mode = "add"
  		header_name = "tf-header-name"
  		header_value = "tf-header-value"
  		rule_type = "all"
  		rule_paths = ["*"]
  	}
  }

  origin {
    origin_type          = "ip"
    origin_list          = ["127.0.0.1"]
    origin_pull_protocol = "follow"
  }

  https_config {
    https_switch         = "off"
    http2_switch         = "off"
    ocsp_stapling_switch = "off"
    spdy_switch          = "off"
    verify_client        = "off"

    force_redirect {
      switch = "on"
    }
  }

  tags = {
    hello = "world"
  }
}

data "tencentcloud_cdn_domains" "cdn_domain" {
  domain       = tencentcloud_cdn_domain.foo.domain
  service_type = tencentcloud_cdn_domain.foo.service_type
}