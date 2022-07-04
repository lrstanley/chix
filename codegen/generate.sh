#!/bin/bash
# shellcheck disable=SC2155

export BASE=$(basename "$PWD")

if [ "$BASE" != "chix" ]; then
	echo "error: please run this script from the repository root"
	exit 1
fi

function cf_ip {
	curl -sS "https://www.cloudflare.com/ips-${1}" | grep -Eo "^[a-zA-Z0-9:.]+/[0-9]+$"
}

export IP_RANGES="$(
	cf_ip v4
	cf_ip v6
)"

cat << EOF > realip_cloudflare.go
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
$(while read -r ip; do echo -e "\t\tbogon.MustCIDR(\"$ip\"),"; done <<< "$IP_RANGES")
	}
}
EOF
