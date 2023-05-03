// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/lrstanley/go-bogon"
)

const (
	OptTrustBogon        RealIPOptions = 1 << iota // Trust bogon IP ranges (private IP ranges).
	OptTrustAny                                    // Trust any proxy (DON'T USE THIS!).
	OptTrustCloudflare                             // Trust Cloudflare's origin IPs.
	OptUseXForwardedFor                            // Allow using the X-Forwarded-For header.
	OptUseXRealIP                                  // Allow using the X-Real-IP header.
	OptUseTrueClientIP                             // Allow using the True-Client-IP header.
	OptUseCFConnectingIP                           // Allow using the CF-Connecting-IP header.

	OptDefaultTrust = OptTrustBogon | OptUseXForwardedFor // Default trust options.

	xForwardedFor = "X-Forwarded-For"
	xRealIP       = "X-Real-IP"
	trueClientIP  = "True-Client-IP"
)

// RealIPOptions is a bitmask of options that can be passed to RealIP.
type RealIPOptions int

// UseRealIPDefault is a convenience function that wraps RealIP with the default
// options (OptTrustBogon and OptUseXForwardedFor).
func UseRealIPDefault(next http.Handler) http.Handler {
	return UseRealIP(nil, OptDefaultTrust)(next)
}

// UseRealIPCLIOpts is a convenience function that wraps RealIP, with support for
// configuring the middleware via CLI flags. You can pass in an array that contains
// a mix of different supported headers, "cloudflare", "*" (or "any", "all") to
// trust anything, "local" (or "localhost", "bogon", "internal") for bogon IPs,
// and anything else gets passed in as allowed CIDRs.
//
// If no options are passed in, the default will use the same as chix.UseRealIPDefault
// (OptTrustBogon and OptUseXForwardedFor).
func UseRealIPCLIOpts(options []string) func(next http.Handler) http.Handler {
	if len(options) == 0 {
		return UseRealIPDefault
	}

	var flags RealIPOptions
	var proxies []string

	for _, option := range options {
		switch strings.ToLower(option) {
		case "cloudflare", "cf-connecting-ip":
			flags |= OptTrustCloudflare | OptUseCFConnectingIP
		case "x-forwarded-for":
			flags |= OptUseXForwardedFor
		case "x-real-ip":
			flags |= OptUseXRealIP
		case "true-client-ip":
			flags |= OptUseTrueClientIP
		case "*", "any", "all":
			flags |= OptTrustAny
		case "local", "localhost", "bogon", "internal":
			flags |= OptTrustBogon
		default:
			proxies = append(proxies, option)
		}
	}

	if flags == 0 {
		flags = OptDefaultTrust
	}

	return UseRealIP(proxies, flags)
}

// UseRealIP is a middleware that allows passing the real IP address of the client
// only if the request headers that include an override, come from a trusted
// proxy. Pass an optional list of trusted proxies to trust, as well as
// any additional options to control the behavior of the middleware. See the
// related Opt* constants for more information. Will panic if invalid IP's or
// ranges are specified.
//
// NOTE: if multiple headers are configured to be trusted, the lookup order is:
//   - CF-Connecting-IP
//   - X-Real-IP
//   - True-Client-IP
//   - X-Forwarded-For
//
// Examples:
//
//	router.Use(chix.UseRealIP([]string{"1.2.3.4", "10.0.0.0/24"}, chix.OptUseXForwardedFor))
//	router.Use(nil, chix.OptTrustBogon|chix.OptUseXForwardedFor))
func UseRealIP(trusted []string, flags RealIPOptions) func(next http.Handler) http.Handler {
	if flags == 0 {
		panic(ErrRealIPNoOpts)
	}

	// Must provide at least one proxy header type.
	if flags&(OptUseXForwardedFor|OptUseXRealIP|OptUseTrueClientIP|OptUseCFConnectingIP) == 0 {
		panic(ErrRealIPNoSource)
	}

	// ¯\_(ツ)_/¯.
	if flags&OptTrustAny != 0 {
		trusted = append(trusted, "0.0.0.0/0")
	}

	rip := &realIP{
		trusted: []*net.IPNet{},
	}

	// Add all known bogon IP ranges.
	if flags&OptTrustBogon != 0 {
		rip.trusted = append(rip.trusted, bogon.DefaultRanges()...)
	}

	if flags&OptTrustCloudflare != 0 {
		rip.trusted = append(rip.trusted, cloudflareRanges()...)
	}

	// Start parsing user-provided CIDR's and/or IP's.
	for _, proxy := range trusted {
		if !strings.Contains(proxy, "/") {
			ip := parseIP(proxy)
			if ip == nil {
				panic(&ErrRealIPInvalidIP{Err: &net.ParseError{Type: "IP address", Text: proxy}})
			}

			switch len(ip) {
			case net.IPv4len:
				proxy += "/32"
			case net.IPv6len:
				proxy += "/128"
			}
		}

		_, cidr, err := net.ParseCIDR(proxy)
		if err != nil {
			panic(fmt.Errorf("chix: realip: invalid CIDR %w", err))
		}

		rip.trusted = append(rip.trusted, cidr)
	}

	if len(rip.trusted) == 0 {
		panic(ErrRealIPNoTrusted)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
			if err != nil {
				goto nexthandler // Fallback and don't modify.
			}

			if trusted := rip.isTrustedProxy(net.ParseIP(ip)); !trusted {
				goto nexthandler // Fallback and don't modify.
			}

			// Parse enabled headers by most specific (and common) to least.
			if flags&OptUseCFConnectingIP != 0 {
				if value := parseIP(r.Header.Get("CF-Connecting-IP")); value != nil {
					r.RemoteAddr = value.String()
					goto nexthandler
				}
			}

			if flags&OptUseXRealIP != 0 {
				if value := parseIP(r.Header.Get(xRealIP)); value != nil {
					r.RemoteAddr = value.String()
					goto nexthandler
				}
			}

			if flags&OptUseTrueClientIP != 0 {
				if value := parseIP(r.Header.Get(trueClientIP)); value != nil {
					r.RemoteAddr = value.String()
					goto nexthandler
				}
			}

			if flags&OptUseXForwardedFor != 0 {
				if value, valid := rip.parseForwardedFor(r.Header.Get(xForwardedFor)); valid && value != "" {
					r.RemoteAddr = value
					goto nexthandler
				}
			}

		nexthandler:
			next.ServeHTTP(w, r)
		})
	}
}

type realIP struct {
	trusted []*net.IPNet
}

// isTrustedProxy will check whether the IP address is included in the trusted
// list according to realIP.trusted.
func (rip *realIP) isTrustedProxy(ip net.IP) bool {
	if ip == nil || rip.trusted == nil {
		return false
	}

	for _, cidr := range rip.trusted {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}

// parseForwardedFor will parse the X-Forwarded-For header in the proper
// direction (reversed).
func (rip *realIP) parseForwardedFor(value string) (clientIP string, valid bool) {
	if value == "" {
		return "", false
	}

	items := strings.Split(value, ",")

	// X-Forwarded-For is appended by each proxy. Check IPs in reverse order
	// and stop when find untrusted proxy.
	for i := len(items) - 1; i >= 0; i-- {
		raw := strings.TrimSpace(items[i])

		ip := net.ParseIP(raw)
		if ip == nil {
			break
		}

		if (i == 0) || (!rip.isTrustedProxy(ip)) {
			return raw, true
		}
	}

	return "", false
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

// UsePrivateIP can be used to allow only private IP's to access specific
// routes. Make sure to register this middleware after UseRealIP, otherwise
// the IP checking may be incorrect.
func UsePrivateIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ok, _ := bogon.Is(sanitizeIP(r.RemoteAddr)); ok {
			next.ServeHTTP(w, r)
			return
		}

		_ = Error(w, r, WrapError(ErrAccessDenied, http.StatusForbidden))
	})
}

// UseContextIP can be used to add the requests IP to the context. This is beneficial
// for passing the request context to a request-unaware function/method/service, that
// does not have access to the original request. Ensure that this middleware is
// registered after UseRealIP, otherwise the stored IP may be incorrect.
func UseContextIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(
			context.WithValue(
				r.Context(),
				contextIP,
				parseIP(sanitizeIP(r.RemoteAddr)),
			),
		))
	})
}

// GetContextIP can be used to retrieve the IP from the context, that was previously
// set by UseContextIP. If no IP was set, nil is returned.
func GetContextIP(ctx context.Context) net.IP {
	if ip, ok := ctx.Value(contextIP).(net.IP); ok {
		return ip
	}

	return nil
}

func sanitizeIP(input string) (ip string) {
	ip, _, err := net.SplitHostPort(strings.TrimSpace(input))
	if err != nil || ip == "" {
		ip = input
	}
	return ip
}
