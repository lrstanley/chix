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

func TestGothConfig_Validate(t *testing.T) {
	t.Parallel()

	t.Run("nil config", func(t *testing.T) {
		t.Parallel()
		var c *GothConfig[testUser, string]
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "nil") {
			t.Fatalf("Validate() = %v, want nil config error", err)
		}
	})

	t.Run("nil service", func(t *testing.T) {
		t.Parallel()
		c := &GothConfig[testUser, string]{SessionStorage: testSessionStore}
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "service") {
			t.Fatalf("Validate() = %v, want service error", err)
		}
	})

	t.Run("nil session storage", func(t *testing.T) {
		t.Parallel()
		c := &GothConfig[testUser, string]{Service: &mockGothService{}}
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "session") {
			t.Fatalf("Validate() = %v, want session storage error", err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		c := &GothConfig[testUser, string]{
			Service:        &mockGothService{},
			SessionStorage: testSessionStore,
		}
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestNewGothHandler_panicsOnInvalidConfig(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for invalid config")
		}
	}()
	NewGothHandler(&GothConfig[testUser, string]{})
}

func TestNewGothHandler_providers(t *testing.T) {
	svc := &mockGothService{id: "x", ident: &testUser{Name: "a"}}
	h := NewGothHandler(&GothConfig[testUser, string]{
		Service:        svc,
		SessionStorage: testSessionStore,
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/providers", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload struct {
		Providers []string `json:"providers"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Providers == nil {
		t.Fatal("expected providers slice (may be empty)")
	}
}

func TestNewGothHandler_unknownProvider(t *testing.T) {
	svc := &mockGothService{id: "x", ident: &testUser{Name: "a"}}
	h := NewGothHandler(&GothConfig[testUser, string]{
		Service:        svc,
		SessionStorage: testSessionStore,
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/providers/nonexistent-provider-xyz", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestNewGothHandler_selfUnauthorized(t *testing.T) {
	svc := &mockGothService{id: "1", ident: &testUser{Name: "a"}}
	h := NewGothHandler(&GothConfig[testUser, string]{
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

func TestNewGothHandler_disableSelfEndpoint(t *testing.T) {
	svc := &mockGothService{id: "1", ident: &testUser{Name: "a"}}
	h := NewGothHandler(&GothConfig[testUser, string]{
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

func TestNewGothHandler_logout(t *testing.T) {
	svc := &mockGothService{id: "1", ident: &testUser{Name: "a"}}
	h := NewGothHandler(&GothConfig[testUser, string]{
		Service:        svc,
		SessionStorage: testSessionStore,
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/logout", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
}
