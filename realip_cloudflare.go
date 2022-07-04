// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.
//
// THIS FILE IS AUTO-GENERATED. DO NOT EDIT.

package chix

import (
	"net"

	"github.com/lrstanley/go-bogon"
)

// cloudflareRanges returns the list of Cloudflare IP ranges.
func cloudflareRanges() []*net.IPNet {
	return []*net.IPNet{
		bogon.MustCIDR("173.245.48.0/20"),
		bogon.MustCIDR("103.21.244.0/22"),
		bogon.MustCIDR("103.22.200.0/22"),
		bogon.MustCIDR("103.31.4.0/22"),
		bogon.MustCIDR("141.101.64.0/18"),
		bogon.MustCIDR("108.162.192.0/18"),
		bogon.MustCIDR("190.93.240.0/20"),
		bogon.MustCIDR("188.114.96.0/20"),
		bogon.MustCIDR("197.234.240.0/22"),
		bogon.MustCIDR("198.41.128.0/17"),
		bogon.MustCIDR("162.158.0.0/15"),
		bogon.MustCIDR("104.16.0.0/13"),
		bogon.MustCIDR("104.24.0.0/14"),
		bogon.MustCIDR("172.64.0.0/13"),
		bogon.MustCIDR("131.0.72.0/22"),
		bogon.MustCIDR("2400:cb00::/32"),
		bogon.MustCIDR("2606:4700::/32"),
		bogon.MustCIDR("2803:f800::/32"),
		bogon.MustCIDR("2405:b500::/32"),
		bogon.MustCIDR("2405:8100::/32"),
		bogon.MustCIDR("2a06:98c0::/29"),
		bogon.MustCIDR("2c0f:f248::/32"),
	}
}
