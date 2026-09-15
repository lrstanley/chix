// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const dummyDSN = "DUMMY_DSN=postgres://audit:audit@127.0.0.1/dummy"

type dummyExposableError struct {
	msg    string
	public bool
}

func (e dummyExposableError) Error() string { return e.msg }
func (e dummyExposableError) Public() bool  { return e.public }

func TestIsExposableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "public true", err: dummyExposableError{msg: dummyDSN, public: true}, want: true},
		{name: "public false", err: dummyExposableError{msg: dummyDSN, public: false}, want: false},
		{name: "plain error", err: errors.New(dummyDSN), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsExposableError(tt.err); got != tt.want {
				t.Fatalf("IsExposableError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorWithCode_5xxMasking(t *testing.T) {
	const publicMsg = "SAFE_PUBLIC_5XX"

	tests := []struct {
		name        string
		err         error
		wantContain string
		wantAbsent  string
	}{
		{
			name:        "public false",
			err:         dummyExposableError{msg: dummyDSN, public: false},
			wantContain: http.StatusText(http.StatusInternalServerError),
			wantAbsent:  dummyDSN,
		},
		{
			name:        "public true",
			err:         dummyExposableError{msg: publicMsg, public: true},
			wantContain: publicMsg,
		},
		{
			name:        "plain error",
			err:         errors.New(dummyDSN),
			wantContain: http.StatusText(http.StatusInternalServerError),
			wantAbsent:  dummyDSN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com/", http.NoBody)
			rec := httptest.NewRecorder()
			ErrorWithCode(rec, req, http.StatusInternalServerError, tt.err)

			if rec.Result().StatusCode != http.StatusInternalServerError {
				t.Fatalf("expected status code %d, got %d", http.StatusInternalServerError, rec.Result().StatusCode)
			}

			body := rec.Body.String()
			if !strings.Contains(body, tt.wantContain) {
				t.Fatalf("body %q does not contain %q", body, tt.wantContain)
			}
			if tt.wantAbsent != "" && strings.Contains(body, tt.wantAbsent) {
				t.Fatalf("body %q contains secret %q", body, tt.wantAbsent)
			}
		})
	}
}

func TestUseRecoverer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

	handler := UseRecoverer()(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
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
		if rvr := recover(); rvr != nil {
			if e, ok := rvr.(error); ok && errors.Is(e, http.ErrAbortHandler) {
				return
			}
			t.Fatalf("expected panic of type http.ErrAbortHandler, got %v", rvr)
		} else {
			t.Fatalf("expected panic of type http.ErrAbortHandler, got nil")
		}
	}()

	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

	handler := UseRecoverer()(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), req)
}
