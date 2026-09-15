// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in the
// LICENSE file.

package xauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/markbates/goth/gothic"
)

// recordingAuth records every ID passed to Get. ok determines which IDs succeed.
type recordingAuth[ID comparable] struct {
	ident *testUser
	got   []ID
	ok    func(ID) bool
}

func (m *recordingAuth[ID]) Get(_ context.Context, id ID) (*testUser, error) {
	m.got = append(m.got, id)
	if m.ok != nil && m.ok(id) {
		return m.ident, nil
	}
	return nil, errors.New("not found")
}

func requestWithAuthSession(t *testing.T, value string) *http.Request {
	t.Helper()
	gothic.Store = testSessionStore

	seed := httptest.NewRequest(http.MethodGet, "http://example.com/", http.NoBody)
	rec := httptest.NewRecorder()
	if err := gothic.StoreInSession(authSessionKey, value, seed, rec); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", http.NoBody)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

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

func TestUseAuthContext_uintSession_getReceivesID(t *testing.T) {
	ident := &testUser{Name: "alice"}
	svc := &recordingAuth[uint]{
		ident: ident,
		ok:    func(id uint) bool { return id == 42 },
	}

	h := UseAuthContext(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := IDFromContext[uint](r.Context()); got != 42 {
			t.Fatalf("IDFromContext = %v, want 42", got)
		}
		if got := IdentFromContext[testUser](r.Context()); got != ident {
			t.Fatalf("IdentFromContext = %v, want %p", got, ident)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithAuthSession(t, "42")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(svc.got) != 1 || svc.got[0] != 42 {
		t.Fatalf("Get ids = %v, want [42]", svc.got)
	}
}

func TestUseAuthContext_uintSession_doesNotCallGetZero(t *testing.T) {
	ident := &testUser{Name: "alice"}
	svc := &recordingAuth[uint]{
		ident: ident,
		ok:    func(id uint) bool { return id == 0 || id == 42 },
	}

	h := UseAuthContext(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := IDFromContext[uint](r.Context()); got != 42 {
			t.Fatalf("IDFromContext = %v, want 42", got)
		}
		if got := IdentFromContext[testUser](r.Context()); got != ident {
			t.Fatalf("IdentFromContext = %v, want %p", got, ident)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithAuthSession(t, "42")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(svc.got) != 1 || svc.got[0] != 42 {
		t.Fatalf("Get ids = %v, want [42] (must not call Get(0))", svc.got)
	}
}

func TestUseAuthContext_uint64Session_getReceivesID(t *testing.T) {
	ident := &testUser{Name: "alice"}
	svc := &recordingAuth[uint64]{
		ident: ident,
		ok:    func(id uint64) bool { return id == 42 },
	}

	h := UseAuthContext(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := IDFromContext[uint64](r.Context()); got != 42 {
			t.Fatalf("IDFromContext = %v, want 42", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithAuthSession(t, "42")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(svc.got) != 1 || svc.got[0] != 42 {
		t.Fatalf("Get ids = %v, want [42]", svc.got)
	}
}

func TestUseAuthContext_intSession_getReceivesID(t *testing.T) {
	ident := &testUser{Name: "alice"}
	svc := &recordingAuth[int]{
		ident: ident,
		ok:    func(id int) bool { return id == 42 },
	}

	h := UseAuthContext(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := IDFromContext[int](r.Context()); got != 42 {
			t.Fatalf("IDFromContext = %v, want 42", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithAuthSession(t, "42")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(svc.got) != 1 || svc.got[0] != 42 {
		t.Fatalf("Get ids = %v, want [42]", svc.got)
	}
}

func TestUseAuthContext_invalidSession_skipsGet(t *testing.T) {
	svc := &recordingAuth[uint]{
		ident: &testUser{Name: "alice"},
		ok:    func(uint) bool { return true },
	}

	h := UseAuthContext(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IdentFromContext[testUser](r.Context()) != nil {
			t.Fatal("did not expect ident for invalid session")
		}
		if got := IDFromContext[uint](r.Context()); got != 0 {
			t.Fatalf("IDFromContext = %v, want 0", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithAuthSession(t, "not-a-number")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(svc.got) != 0 {
		t.Fatalf("Get ids = %v, want none", svc.got)
	}
}
