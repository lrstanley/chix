// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUseRecoverer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

	handler := UseRecoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("testing panic")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status code %d, got %d", http.StatusInternalServerError, rec.Result().StatusCode)
	}
}

func TestUseRecovererAbort(t *testing.T) {
	defer func() {
		if rvr := recover(); rvr != http.ErrAbortHandler {
			t.Fatalf("expected panic of type http.ErrAbortHandler, got %v", rvr)
		}
	}()

	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

	handler := UseRecoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), req)
}
