// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.
//
// THIS FILE IS AUTO-GENERATED. DO NOT EDIT.

package chix

import "net"

// cloudflareRanges returns the list of Cloudflare IP ranges.
func cloudflareRanges() []*net.IPNet {
	return []*net.IPNet{
		mustCIDR("173.245.48.0/20"),
		mustCIDR("103.21.244.0/22"),
		mustCIDR("103.22.200.0/22"),
		mustCIDR("103.31.4.0/22"),
		mustCIDR("141.101.64.0/18"),
		mustCIDR("108.162.192.0/18"),
		mustCIDR("190.93.240.0/20"),
		mustCIDR("188.114.96.0/20"),
		mustCIDR("197.234.240.0/22"),
		mustCIDR("198.41.128.0/17"),
		mustCIDR("162.158.0.0/15"),
		mustCIDR("104.16.0.0/13"),
		mustCIDR("104.24.0.0/14"),
		mustCIDR("172.64.0.0/13"),
		mustCIDR("131.0.72.0/22"),
		mustCIDR("2400:cb00::/32"),
		mustCIDR("2606:4700::/32"),
		mustCIDR("2803:f800::/32"),
		mustCIDR("2405:b500::/32"),
		mustCIDR("2405:8100::/32"),
		mustCIDR("2a06:98c0::/29"),
		mustCIDR("2c0f:f248::/32"),
	}
}
