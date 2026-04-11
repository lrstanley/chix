// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in the
// LICENSE file.

package xauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBasicAuthConfig_Validate(t *testing.T) {
	t.Parallel()

	t.Run("nil config", func(t *testing.T) {
		t.Parallel()
		var c *BasicAuthConfig[testUser]
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "nil") {
			t.Fatalf("Validate() = %v, want nil config error", err)
		}
	})

	t.Run("nil service", func(t *testing.T) {
		t.Parallel()
		c := &BasicAuthConfig[testUser]{SessionStorage: testSessionStore}
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "service") {
			t.Fatalf("Validate() = %v, want service error", err)
		}
	})

	t.Run("nil session storage", func(t *testing.T) {
		t.Parallel()
		c := &BasicAuthConfig[testUser]{Service: &mockBasicAuth{}}
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "session") {
			t.Fatalf("Validate() = %v, want session storage error", err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		c := &BasicAuthConfig[testUser]{
			Service:        &mockBasicAuth{},
			SessionStorage: testSessionStore,
		}
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestNewBasicAuthHandler_panicsOnInvalidConfig(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for invalid config")
		}
	}()
	NewBasicAuthHandler(&BasicAuthConfig[testUser]{})
}

func TestNewBasicAuthHandler_loginAndSelf(t *testing.T) {
	svc := &mockBasicAuth{
		ident:     &testUser{Name: "alice"},
		validUser: "alice",
		validPass: "secret",
	}
	h := NewBasicAuthHandler(&BasicAuthConfig[testUser]{
		Service:        svc,
		SessionStorage: testSessionStore,
	})
	mux := http.NewServeMux()
	mux.Handle("/auth/", http.StripPrefix("/auth", h))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth/login", http.NoBody)
	req.SetBasicAuth("alice", "secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}

	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/auth/self", http.NoBody)
	for _, c := range rec.Result().Cookies() {
		req2.AddCookie(c)
	}
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("/self status = %d, want %d", rec2.Code, http.StatusOK)
	}

	var payload struct {
		Auth *testUser `json:"auth"`
	}
	if err := json.NewDecoder(rec2.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Auth == nil || payload.Auth.Name != "alice" {
		t.Fatalf("auth = %+v, want alice", payload.Auth)
	}
}

func TestNewBasicAuthHandler_loginUnauthorized_noHeader(t *testing.T) {
	svc := &mockBasicAuth{validUser: "u", validPass: "p"}
	h := NewBasicAuthHandler(&BasicAuthConfig[testUser]{
		Service:        svc,
		SessionStorage: testSessionStore,
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/login", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, "Basic") {
		t.Fatalf("WWW-Authenticate = %q, want Basic challenge", got)
	}
}

func TestNewBasicAuthHandler_loginUnauthorized_badCredentials(t *testing.T) {
	svc := &mockBasicAuth{validUser: "alice", validPass: "secret"}
	h := NewBasicAuthHandler(&BasicAuthConfig[testUser]{
		Service:        svc,
		SessionStorage: testSessionStore,
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/login", http.NoBody)
	req.SetBasicAuth("alice", "wrong")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNewBasicAuthHandler_selfUnauthorized(t *testing.T) {
	svc := &mockBasicAuth{ident: &testUser{Name: "a"}, validUser: "a", validPass: "p"}
	h := NewBasicAuthHandler(&BasicAuthConfig[testUser]{
		Service:        svc,
		SessionStorage: testSessionStore,
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/self", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNewBasicAuthHandler_loginWhenAlreadyAuthenticated(t *testing.T) {
	svc := &mockBasicAuth{ident: &testUser{Name: "alice"}, validUser: "alice", validPass: "secret"}
	h := NewBasicAuthHandler(&BasicAuthConfig[testUser]{
		Service:        svc,
		SessionStorage: testSessionStore,
	})
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := &testUser{Name: "alice"}
		r = r.WithContext(OverrideContextAuth(r.Context(), "alice", u))
		h.ServeHTTP(w, r)
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/login", http.NoBody)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}
}

func TestNewBasicAuthHandler_logout(t *testing.T) {
	svc := &mockBasicAuth{
		ident:     &testUser{Name: "alice"},
		validUser: "alice",
		validPass: "secret",
	}
	h := NewBasicAuthHandler(&BasicAuthConfig[testUser]{
		Service:        svc,
		SessionStorage: testSessionStore,
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/login", http.NoBody)
	req.SetBasicAuth("alice", "secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/logout", http.NoBody)
	for _, c := range rec.Result().Cookies() {
		req2.AddCookie(c)
	}
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusFound {
		t.Fatalf("logout status = %d, want %d", rec2.Code, http.StatusFound)
	}
}

func TestNewBasicAuthHandler_disableSelfEndpoint(t *testing.T) {
	svc := &mockBasicAuth{ident: &testUser{Name: "a"}, validUser: "a", validPass: "p"}
	h := NewBasicAuthHandler(&BasicAuthConfig[testUser]{
		Service:             svc,
		SessionStorage:      testSessionStore,
		DisableSelfEndpoint: true,
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/self", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
