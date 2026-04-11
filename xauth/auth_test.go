// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in the
// LICENSE file.

package xauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUseAuthRequired(t *testing.T) {
	t.Parallel()

	h := UseAuthRequired[testUser]()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUseAuthRequired_authenticated(t *testing.T) {
	t.Parallel()

	u := &testUser{Name: "alice"}
	ctx := OverrideContextAuth(context.Background(), "alice", u)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", http.NoBody)
	req = req.WithContext(ctx)

	h := UseAuthRequired[testUser]()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IdentFromContext[testUser](r.Context()) == nil {
			t.Fatal("expected ident in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestOverrideContextAuth_IDFromContext_IdentFromContext(t *testing.T) {
	t.Parallel()

	u := &testUser{Name: "bob"}
	ctx := context.Background()
	ctx = OverrideContextAuth(ctx, 42, u)

	if got := IDFromContext[int](ctx); got != 42 {
		t.Fatalf("IDFromContext = %v, want 42", got)
	}
	if got := IdentFromContext[testUser](ctx); got != u {
		t.Fatalf("IdentFromContext = %v, want %p", got, u)
	}
}

func TestUseAuthContext_noSession_skips(t *testing.T) {
	t.Parallel()

	svc := &mockBasicAuth{ident: &testUser{Name: "x"}, validUser: "u", validPass: "p"}
	h := UseAuthContext(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IdentFromContext[testUser](r.Context()) != nil {
			t.Fatal("did not expect ident without session")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUseAuthContext_identAlreadyInContext_noop(t *testing.T) {
	t.Parallel()

	svc := &mockBasicAuth{ident: &testUser{Name: "other"}, validUser: "u", validPass: "p"}
	u := &testUser{Name: "alice"}
	ctx := OverrideContextAuth(context.Background(), "alice", u)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", http.NoBody)
	req = req.WithContext(ctx)

	h := UseAuthContext(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := IdentFromContext[testUser](r.Context()); got == nil || got.Name != "alice" {
			t.Fatalf("expected ident alice, got %v", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
