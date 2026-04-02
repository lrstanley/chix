// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/lrstanley/chix/v2/internal/text"
)

// RealIPHeaderParser is a function that parses the real IP address from the
// request headers. It returns a list of IP addresses associated with that header
// (trusted or not).
type RealIPHeaderParser func(headers http.Header, remoteAddr net.IP) []net.IP

// RealIPXForwardedFor is a function that parses the [X-Forwarded-For] header and
// returns a list of IP addresses associated with that header, in reverse order.
//
// [X-Forwarded-For]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/X-Forwarded-For
func RealIPXForwardedFor() RealIPHeaderParser {
	return func(headers http.Header, _ net.IP) []net.IP {
		items := strings.Split(headers.Get("X-Forwarded-For"), ",")
		if len(items) == 0 {
			return nil
		}

		// X-Forwarded-For is appended by each proxy. Check IPs in reverse order
		// and stop when find untrusted proxy.
		var raw string
		ips := make([]net.IP, 0, len(items))
		for i := len(items) - 1; i >= 0; i-- {
			raw = strings.TrimSpace(items[i])
			ip := net.ParseIP(raw)
			if ip == nil {
				return nil
			}
			ips = append(ips, ip)
		}
		return ips
	}
}

// RealIPXRealIP is a function that parses the X-Real-Ip header and returns the
// IP address associated with that header.
func RealIPXRealIP() RealIPHeaderParser {
	return func(headers http.Header, _ net.IP) []net.IP {
		if v := parseIP(headers.Get("X-Real-Ip")); v != nil {
			return []net.IP{v}
		}
		return nil
	}
}

// RealIPTrueClientIP is a function that parses the True-Client-Ip header and returns
// the IP address associated with that header.
func RealIPTrueClientIP() RealIPHeaderParser {
	return func(headers http.Header, _ net.IP) []net.IP {
		if v := parseIP(headers.Get("True-Client-Ip")); v != nil {
			return []net.IP{v}
		}
		return nil
	}
}

// RealIPCFConnectingIP is a function that parses the [Cf-Connecting-Ip] header and
// returns the IP address associated with that header.
//
// [Cf-Connecting-Ip]: https://developers.cloudflare.com/fundamentals/reference/http-headers/#cf-connecting-ip
func RealIPCFConnectingIP() RealIPHeaderParser {
	return func(headers http.Header, _ net.IP) []net.IP {
		if v := parseIP(headers.Get("Cf-Connecting-Ip")); v != nil {
			return []net.IP{v}
		}
		return nil
	}
}

// DefaultRealIPConfig returns a new [RealIPConfig] with the recommended default values.
func DefaultRealIPConfig() *RealIPConfig {
	return &RealIPConfig{
		TrustPrivate: true,
		Headers:      []RealIPHeaderParser{RealIPXForwardedFor()},
	}
}

// RealIPConfig is the configuration for the realip middleware.
type RealIPConfig struct {
	// Trusted is a list of IP addresses or CIDR ranges that are trusted.
	Trusted []string
	// TrustPrivate is a boolean that indicates if private bogon IP ranges should
	// be trusted.
	TrustPrivate bool
	// TrustAny is a boolean that indicates if any IP should be trusted.
	TrustAny bool
	// TrustCloudflare is a boolean that indicates if Cloudflare IP ranges should
	// be trusted, in addition to their associated headers.
	TrustCloudflare bool
	// Headers is a list of header parsers that are used to parse the real IP address
	// from the request headers. The order here is important. If you have multiple,
	// it should be ordered from headers with highest specificity to lowest (i.e.
	// headers which only return 1 IP, vs headers like X-Forwarded-For which may
	// return multiple IPs). Recommended ordering:
	//
	//   - CF-Connecting-IP
	//   - X-Real-IP
	//   - True-Client-IP
	//   - X-Forwarded-For
	Headers []RealIPHeaderParser

	trustedParsed []*net.IPNet
}

// IsTrusted checks if the given IP is trusted. If [RealIPConfig.TrustAny] is true,
// this will always return true.
func (c *RealIPConfig) IsTrusted(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if c.TrustAny {
		return true
	}
	for _, cidr := range c.trustedParsed {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// Validate validates the realip config. Use this to validate the config before using
// it, otherwise [UseRealIP] will panic if an invalid config is provided.
func (c *RealIPConfig) Validate() error {
	c.trustedParsed = nil

	for _, v := range c.Trusted {
		_, cidr, err := net.ParseCIDR(v)
		if err == nil {
			c.trustedParsed = append(c.trustedParsed, cidr)
			continue
		}

		ip := net.ParseIP(v)
		if ip == nil {
			return fmt.Errorf("invalid IP or CIDR: %s", v)
		}

		if v4 := ip.To4(); v4 != nil {
			c.trustedParsed = append(c.trustedParsed, &net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)})
		} else {
			c.trustedParsed = append(c.trustedParsed, &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)})
		}
	}

	if c.TrustPrivate {
		c.trustedParsed = append(c.trustedParsed, privateCIDRs[:]...)
	}

	if c.TrustAny {
		c.trustedParsed = append(c.trustedParsed, &net.IPNet{IP: net.IP{0, 0, 0, 0}, Mask: net.CIDRMask(0, 0)})
	}

	if c.TrustCloudflare {
		c.trustedParsed = append(c.trustedParsed, cloudflareRanges()...)
		if len(c.Headers) == 0 {
			c.Headers = append(c.Headers, RealIPCFConnectingIP())
		}
	}

	if len(c.trustedParsed) == 0 {
		return errors.New("no trusted proxies or bogon IPs specified")
	}

	if len(c.Headers) == 0 {
		return errors.New("no header parsers specified")
	}

	return nil
}

// FromStringOpts parses a list of string options and updates the config accordingly.
// See [UseRealIPStringOpts] for supported options.
func (c *RealIPConfig) FromStringOpts(options []string) error {
	if len(options) == 0 {
		*c = *DefaultRealIPConfig()
		return nil
	}

	options = text.Map(options, strings.TrimSpace, strings.ToLower)

	for _, option := range options {
		switch option {
		case "cloudflare", "cf-connecting-ip":
			c.TrustCloudflare = true
		case "x-forwarded-for":
			c.Headers = append(c.Headers, RealIPXForwardedFor())
		case "x-real-ip":
			c.Headers = append(c.Headers, RealIPXRealIP())
		case "true-client-ip":
			c.Headers = append(c.Headers, RealIPTrueClientIP())
		case "*", "any", "all":
			c.TrustAny = true
			c.Headers = []RealIPHeaderParser{RealIPXForwardedFor()}
		case "local", "localhost", "bogon", "internal", "private":
			c.TrustPrivate = true
		case "":
			continue
		default:
			c.Trusted = append(c.Trusted, option)
		}
	}

	return c.Validate()
}

// UseRealIPStringOpts is a convenience function that wraps [UseRealIP], with support
// for configuring the middleware via CLI/string flags. You can pass in an array that
// contains a mix of different supported headers.
//
// Supported options are provided below:
//
//   - "cloudflare", "cf-connecting-ip": trust cloudflare ranges, and use Cf-Connecting-Ip header.
//   - "x-forwarded-for": use X-Forwarded-For header.
//   - "x-real-ip": use X-Real-Ip header.
//   - "true-client-ip": use True-Client-Ip header.
//   - "*", "any", "all": trust any IP and use X-Forwarded-For header.
//   - "local", "localhost", "bogon", "internal", "private": trust private IP ranges.
//   - any other string is treated as a trusted IP or CIDR.
//
// If no options are passed in, [DefaultRealIPConfig] is used.
func UseRealIPStringOpts(options []string) func(next http.Handler) http.Handler {
	c := &RealIPConfig{}
	if err := c.FromStringOpts(options); err != nil {
		panic(fmt.Errorf("failed to validate realip config: %w", err))
	}
	return UseRealIP(c)
}

// UseRealIP is a middleware that allows passing the real IP address of the client
// only if the request headers that include an override, come from a trusted
// proxy.
func UseRealIP(config *RealIPConfig) func(next http.Handler) http.Handler {
	if config == nil {
		config = DefaultRealIPConfig()
	}

	if err := config.Validate(); err != nil {
		panic(fmt.Errorf("failed to validate realip config: %w", err))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var ips []net.IP
			var allTrusted bool

			ip := parseIP(sanitizeIP(r.RemoteAddr))
			if ip == nil || !config.IsTrusted(ip) {
				goto nexthandler // Fallback and don't modify.
			}

			for i := range config.Headers {
				ips = config.Headers[i](r.Header, ip)
				allTrusted = true
				for _, rip := range ips {
					if rip == nil {
						continue
					}

					if !config.IsTrusted(rip) {
						ip = rip
						allTrusted = false
						break
					}
				}

				if len(ips) > 0 && allTrusted { // All IPs were trusted, so take the last one.
					ip = ips[len(ips)-1]
					goto nexthandler
				}
				if !allTrusted {
					goto nexthandler
				}
			}

		nexthandler:
			if ip != nil {
				r.RemoteAddr = ip.String()
			}
			next.ServeHTTP(w, r)
		})
	}
}

// parseIP parse a string representation of an IP and returns a net.IP with
// the appropriate byte representation or nil, if the input is invalid.
func parseIP(ip string) net.IP {
	parsedIP := net.ParseIP(strings.TrimSpace(ip))
	if parsedIP != nil {
		if v4 := parsedIP.To4(); v4 != nil {
			return v4
		}
	}
	return parsedIP
}

// sanitizeIP parses a string representation of an IP (potentially with a port)
// and returns a string with the IP only. This is a best effort attempt, otherwise
// returning the original input.
func sanitizeIP(input string) (ip string) {
	ip, _, err := net.SplitHostPort(strings.TrimSpace(input))
	if err != nil || ip == "" {
		ip = input
	}
	return ip
}

// parseCIDR parses a string representation of a CIDR or IP and returns a [net.IPNet],
// if it is an invalid CIDR or IP (which would get turned into a CIDR by defaulting to
// a /32 or /128 mask), nil is returned.
func parseCIDR(input string) *net.IPNet {
	_, cidr, err := net.ParseCIDR(input)
	if err == nil {
		return cidr
	}

	ip := net.ParseIP(input)
	if ip == nil {
		return nil
	}

	if v4 := ip.To4(); v4 != nil {
		return &net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)}
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
}

// mustCIDR parses a string representation of a CIDR or IP and returns a [net.IPNet],
// if it is an invalid CIDR or IP (which would get turned into a CIDR by defaulting to
// a /32 or /128 mask), a panic is thrown.
func mustCIDR(cidr string) *net.IPNet {
	v := parseCIDR(cidr)
	if v == nil {
		panic(fmt.Sprintf("%s is not a valid CIDR or IP", cidr))
	}
	return v
}

type contextKeyIP struct{}

// UseContextIP can be used to add the requests IP to the context. This is beneficial
// for passing the request context to a request-unaware function/method/service, that
// does not have access to the original request. Ensure that this middleware is
// registered after [UseRealIP], otherwise the stored IP may be incorrect.
func UseContextIP() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(
				context.WithValue(
					r.Context(),
					contextKeyIP{},
					parseIP(sanitizeIP(r.RemoteAddr)),
				),
			))
		})
	}
}

// GetContextIP can be used to retrieve the IP from the context, that was previously
// set by [UseContextIP]. If no IP was set, nil is returned.
func GetContextIP(ctx context.Context) net.IP {
	if ip, ok := ctx.Value(contextKeyIP{}).(net.IP); ok {
		return ip
	}
	return nil
}

var privateCIDRs = [...]*net.IPNet{
	// IPv4 ranges.
	mustCIDR("10.0.0.0/8"),         // Private-use networks.
	mustCIDR("100.64.0.0/10"),      // Carrier-grade NAT (CGN) networks.
	mustCIDR("127.0.0.0/8"),        // Loopback network.
	mustCIDR("169.254.0.0/16"),     // Link-local networks.
	mustCIDR("172.16.0.0/12"),      // Private-use networks.
	mustCIDR("192.0.0.0/24"),       // IETF protocol assignments.
	mustCIDR("192.0.2.0/24"),       // Documentation (TEST-NET-1) networks.
	mustCIDR("192.168.0.0/16"),     // Private-use networks.
	mustCIDR("198.18.0.0/15"),      // Benchmarking networks.
	mustCIDR("198.51.100.0/24"),    // Documentation (TEST-NET-2) networks.
	mustCIDR("203.0.113.0/24"),     // Documentation (TEST-NET-3) networks.
	mustCIDR("224.0.0.0/4"),        // Multicast networks.
	mustCIDR("240.0.0.0/4"),        // Reserved for future use.
	mustCIDR("255.255.255.255/32"), // Limited broadcast network.

	// IPv6 ranges.
	mustCIDR("::/128"),        // Node-scope unicast unspecified address.
	mustCIDR("::1/128"),       // Node-scope unicast loopback address.
	mustCIDR("100::/64"),      // Remotely triggered black hole addresses.
	mustCIDR("2001:10::/28"),  // Overlay routable cryptographic hash identifiers (ORCHID).
	mustCIDR("2001:db8::/32"), // Documentation prefix.
	mustCIDR("3fff::/20"),     // Documentation prefix.
	mustCIDR("fc00::/7"),      // Unique local addresses (ULA).
	mustCIDR("fe80::/10"),     // Link-local unicast.
	mustCIDR("fec0::/10"),     // Site-local unicast (deprecated).
	mustCIDR("ff00::/8"),      // Multicast (Note: ff0e:/16 is global scope and may appear on the global internet.).

	// 6to4 bogons. These aren't "officially" part of the bogon list, but are
	// trusted for this type of scenario.
	mustCIDR("2002:a00::/24"),         // 6to4 bogon (10.0.0.0/8).
	mustCIDR("2002:7f00::/24"),        // 6to4 bogon (127.0.0.0/8).
	mustCIDR("2002:a9fe::/32"),        // 6to4 bogon (169.254.0.0/16).
	mustCIDR("2002:ac10::/28"),        // 6to4 bogon (172.16.0.0/12).
	mustCIDR("2002:c000::/40"),        // 6to4 bogon (192.0.0.0/24).
	mustCIDR("2002:c000:200::/40"),    // 6to4 bogon (192.0.2.0/24).
	mustCIDR("2002:c0a8::/32"),        // 6to4 bogon (192.168.0.0/16).
	mustCIDR("2002:c612::/31"),        // 6to4 bogon (198.18.0.0/15).
	mustCIDR("2002:c633:6400::/40"),   // 6to4 bogon (198.51.100.0/24).
	mustCIDR("2002:cb00:7100::/40"),   // 6to4 bogon (203.0.113.0/24).
	mustCIDR("2002:e000::/20"),        // 6to4 bogon (224.0.0.0/4).
	mustCIDR("2002:f000::/20"),        // 6to4 bogon (240.0.0.0/4).
	mustCIDR("2002:ffff:ffff::/48"),   // 6to4 bogon (255.255.255.255/32).
	mustCIDR("2001:0:a00::/40"),       // Teredo bogon (10.0.0.0/8).
	mustCIDR("2001:0:7f00::/40"),      // Teredo bogon (127.0.0.0/8).
	mustCIDR("2001:0:a9fe::/48"),      // Teredo bogon (169.254.0.0/16).
	mustCIDR("2001:0:ac10::/44"),      // Teredo bogon (172.16.0.0/12).
	mustCIDR("2001:0:c000::/56"),      // Teredo bogon (192.0.0.0/24).
	mustCIDR("2001:0:c000:200::/56"),  // Teredo bogon (192.0.2.0/24).
	mustCIDR("2001:0:c0a8::/48"),      // Teredo bogon (192.168.0.0/16).
	mustCIDR("2001:0:c612::/47"),      // Teredo bogon (198.18.0.0/15).
	mustCIDR("2001:0:c633:6400::/56"), // Teredo bogon (198.51.100.0/24).
	mustCIDR("2001:0:cb00:7100::/56"), // Teredo bogon (203.0.113.0/24).
	mustCIDR("2001:0:e000::/36"),      // Teredo bogon (224.0.0.0/4).
	mustCIDR("2001:0:f000::/36"),      // Teredo bogon (240.0.0.0/4).
	mustCIDR("2001:0:ffff:ffff::/64"), // Teredo bogon (255.255.255.255/32).
}

// UsePrivateIP can be used to allow only private IP's to access specific
// routes. Make sure to register this middleware after [UseRealIP], otherwise
// the IP checking may be incorrect.
func UsePrivateIP() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := parseIP(sanitizeIP(r.RemoteAddr))
			if ip == nil || ip.IsUnspecified() {
				ErrorWithCode(w, r, http.StatusForbidden, ErrAccessDenied)
				return
			}
			for i := range privateCIDRs {
				if privateCIDRs[i].Contains(ip) {
					next.ServeHTTP(w, r)
					return
				}
			}
			ErrorWithCode(w, r, http.StatusForbidden, ErrAccessDenied)
		})
	}
}
