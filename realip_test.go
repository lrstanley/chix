// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var testsRealIP = []struct {
	name       string
	args       []string
	headers    map[string]string
	remoteAddr string
	wantRealIP string
}{
	{
		name:       "ipv4:none:bogon:untrusted",
		args:       []string{"x-forwarded-for", "local"},
		headers:    map[string]string{},
		remoteAddr: "1.2.3.4:12345",
		wantRealIP: "1.2.3.4:12345",
	},
	{
		name:       "ipv4:x-forwarded-for:bogon:invalid-remote-addr",
		args:       []string{"x-forwarded-for", "local"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "invalid",
		wantRealIP: "invalid",
	},
	{
		name:       "ipv4:cf-connecting-ip:cloudflare:trusted",
		args:       []string{"cloudflare"},
		headers:    map[string]string{"CF-Connecting-IP": "1.1.1.1"},
		remoteAddr: "173.245.48.0:12345",
		wantRealIP: "1.1.1.1",
	},
	{
		name:       "ipv4:x-forwarded-for:cloudflare:untrusted",
		args:       []string{"cloudflare"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "173.245.48.0:12345",
		wantRealIP: "173.245.48.0:12345",
	},
	{
		name:       "ipv4:x-forwarded-for:cloudflare:untrusted",
		args:       []string{"cloudflare"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "1.2.3.4:12345",
		wantRealIP: "1.2.3.4:12345",
	},
	{
		name:       "ipv4:cf-connecting-ip:cloudflare:untrusted",
		args:       []string{"cloudflare"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "1.2.3.4:12345",
		wantRealIP: "1.2.3.4:12345",
	},
	{
		name:       "ipv4:x-forwarded-for-invalid:bogon:untrusted",
		args:       []string{"x-forwarded-for", "local"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.999"},
		remoteAddr: "10.1.2.3:12345",
		wantRealIP: "10.1.2.3:12345",
	},
	{
		name:       "ipv6:x-forwarded-for:bogon:trusted-different-protocol",
		args:       []string{"x-forwarded-for", "local"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "[::1]:12345",
		wantRealIP: "1.1.1.1",
	},
	{
		name:       "ipv6:x-forwarded-for:bogon:trusted-same-protocol",
		args:       []string{"x-forwarded-for", "local"},
		headers:    map[string]string{"X-Forwarded-For": "2607:f8b0:4002:c00::8b"},
		remoteAddr: "[::1]:12345",
		wantRealIP: "2607:f8b0:4002:c00::8b",
	},
	{
		name:       "ipv6:x-forwarded-for:bogon:untrusted-1",
		args:       []string{"x-forwarded-for", "local"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "[2607:f8b0:4002:c00::8a]:12345",
		wantRealIP: "[2607:f8b0:4002:c00::8a]:12345",
	},
	{
		name:       "ipv6:x-forwarded-for:bogon:untrusted-2",
		args:       []string{"x-forwarded-for", "local"},
		headers:    map[string]string{"X-Forwarded-For": "2607:f8b0:4002:c00::8b"},
		remoteAddr: "[2607:f8b0:4002:c00::8a]:12345",
		wantRealIP: "[2607:f8b0:4002:c00::8a]:12345",
	},
	{
		name:       "ipv4:x-forwarded-for:bogon:untrusted",
		args:       []string{"x-forwarded-for", "local"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "1.2.3.4:12345",
		wantRealIP: "1.2.3.4:12345",
	},
	{
		name:       "ipv4:x-forwarded-for:custom-cidr:untrusted",
		args:       []string{"x-forwarded-for", "8.8.8.8/32"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "1.2.3.4:12345",
		wantRealIP: "1.2.3.4:12345",
	},
	{
		name:       "ipv4:x-forwarded-for:custom-cidr-multiple-1:trusted",
		args:       []string{"x-forwarded-for", "8.0.0.0/8", "9.0.0.0/8"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "8.1.2.3:12345",
		wantRealIP: "1.1.1.1",
	},
	{
		name:       "ipv4:x-forwarded-for:custom-cidr-multiple-2:trusted",
		args:       []string{"x-forwarded-for", "8.0.0.0/8", "9.0.0.0/8"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "9.1.2.3:12345",
		wantRealIP: "1.1.1.1",
	},
	{
		name:       "ipv4:x-forwarded-for:custom-cidr-multiple-3:untrusted",
		args:       []string{"x-forwarded-for", "8.0.0.0/8", "9.0.0.0/8"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "10.1.2.3:12345",
		wantRealIP: "10.1.2.3:12345",
	},
	{
		name:       "ipv4:x-forwarded-for:custom-cidr:trusted",
		args:       []string{"x-forwarded-for", "8.8.8.8/32"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "8.8.8.8:12345",
		wantRealIP: "1.1.1.1",
	},
	{
		name:       "ipv4:x-forwarded-for:one-ip:trusted",
		args:       []string{"x-forwarded-for", "8.8.8.8"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "8.8.8.8:12345",
		wantRealIP: "1.1.1.1",
	},
	{
		name:       "ipv6:x-forwarded-for:one-ip:trusted",
		args:       []string{"x-forwarded-for", "::1"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "[::1]:12345",
		wantRealIP: "1.1.1.1",
	},
	{
		name:       "ipv4:x-forwarded-for:all:trusted",
		args:       []string{"x-forwarded-for", "all"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "8.8.8.8:12345",
		wantRealIP: "1.1.1.1",
	},
	{
		name:       "ipv4:x-forwarded-for:bogon:trusted",
		args:       []string{"x-forwarded-for", "local"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1"},
		remoteAddr: "10.1.1.1:12345",
		wantRealIP: "1.1.1.1",
	},
	{
		name:       "ipv4:x-forwarded-for-multiple:bogon:trusted",
		args:       []string{"x-forwarded-for", "local"},
		headers:    map[string]string{"X-Forwarded-For": "1.1.1.1,2.2.2.2"},
		remoteAddr: "10.1.1.1:12345",
		wantRealIP: "2.2.2.2",
	},
	{
		name:       "ipv4:x-forwarded-for:bogon:x-real-ip",
		args:       []string{"x-forwarded-for", "local"},
		headers:    map[string]string{"X-Real-IP": "1.1.1.1"},
		remoteAddr: "1.2.3.4:12345",
		wantRealIP: "1.2.3.4:12345",
	},
	{
		name:       "ipv4:x-real-ip:bogon:untrusted",
		args:       []string{"x-real-ip", "local"},
		headers:    map[string]string{"X-Real-IP": "1.1.1.1"},
		remoteAddr: "1.2.3.4:12345",
		wantRealIP: "1.2.3.4:12345",
	},
	{
		name:       "ipv4:x-real-ip:bogon:trusted",
		args:       []string{"x-real-ip", "local"},
		headers:    map[string]string{"X-Real-IP": "1.1.1.1"},
		remoteAddr: "10.1.1.1:12345",
		wantRealIP: "1.1.1.1",
	},
	{
		name:       "ipv4:true-client-ip:bogon:untrusted",
		args:       []string{"true-client-ip", "local"},
		headers:    map[string]string{"True-Client-IP": "1.1.1.1"},
		remoteAddr: "1.2.3.4:12345",
		wantRealIP: "1.2.3.4:12345",
	},
	{
		name:       "ipv4:true-client-ip:bogon:trusted",
		args:       []string{"true-client-ip", "local"},
		headers:    map[string]string{"True-Client-IP": "1.1.1.1"},
		remoteAddr: "10.1.1.1:12345",
		wantRealIP: "1.1.1.1",
	},
}

func FuzzUseRealIPCLIOpts(f *testing.F) {
	for _, tt := range testsRealIP {
		for _, v := range tt.headers {
			f.Add(v)
		}
		f.Add(tt.wantRealIP)
		f.Add(tt.remoteAddr)
	}

	f.Fuzz(func(t *testing.T, data string) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		req.RemoteAddr = "1.2.3.4:12345"
		req.Header.Set("X-Forwarded-For", data)

		handler := UseRealIPCLIOpts(
			[]string{"x-forwarded-for", "all"},
		)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = parseIP(sanitizeIP(r.RemoteAddr))
		}))

		handler.ServeHTTP(httptest.NewRecorder(), req)
	})
}

func TestUseRealIPCLIOpts(t *testing.T) {
	for _, tt := range testsRealIP {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
			req.RemoteAddr = tt.remoteAddr

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			handler := UseRealIPCLIOpts(tt.args)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.RemoteAddr != tt.wantRealIP {
					t.Errorf("UseRealIPCLIOpts() = %v, want %v", r.RemoteAddr, tt.wantRealIP)
				}
			}))

			handler.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
}

func TestUsePrivateIP(t *testing.T) {
	tests := []struct {
		name       string
		allowed    bool
		remoteAddr string
		statusCode int
	}{
		{
			name:       "ipv4:private",
			allowed:    true,
			remoteAddr: "10.1.2.3:12345",
			statusCode: http.StatusOK,
		},
		{
			name:       "ipv4:not-private",
			allowed:    false,
			remoteAddr: "1.1.1.1:12345",
			statusCode: http.StatusForbidden,
		},
		{
			name:       "ipv6:private",
			allowed:    true,
			remoteAddr: "[::1]:12345",
			statusCode: http.StatusOK,
		},
		{
			name:       "ipv6:not-private",
			allowed:    false,
			remoteAddr: "[2001:4860:4860::8888]:12345",
			statusCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
			req.RemoteAddr = tt.remoteAddr

			// Also test UseContextIP/GetContextIP.
			handler := UseContextIP(UsePrivateIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !tt.allowed {
					t.Errorf("UsePrivateIP() = %v but allowed (true), want %v", r.RemoteAddr, tt.allowed)
				}

				if !GetContextIP(r.Context()).Equal(parseIP(sanitizeIP(r.RemoteAddr))) {
					t.Errorf("GetContextIP() = %v, want %v", r.RemoteAddr, GetContextIP(r.Context()))
				}

				w.WriteHeader(http.StatusOK)
			})))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Result().StatusCode != tt.statusCode {
				t.Errorf("UsePrivateIP() returned status %v, want %v", rec.Result().StatusCode, tt.statusCode)
			}
		})
	}
}
