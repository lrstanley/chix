// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func requestWithConfig(cfg *Config, req *http.Request) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), contextKeyConfig{}, cfg))
}

func TestConfigDefaultMaxRequestBodyBytes(t *testing.T) {
	cfg := NewConfig()
	if got := cfg.GetMaxRequestBodyBytes(); got != DefaultMaxRequestBodyBytes {
		t.Fatalf("GetMaxRequestBodyBytes() = %d, want %d", got, DefaultMaxRequestBodyBytes)
	}
}

func TestSetMaxRequestBodyBytes(t *testing.T) {
	cfg := NewConfig().SetMaxRequestBodyBytes(1024)
	if got := cfg.GetMaxRequestBodyBytes(); got != 1024 {
		t.Fatalf("GetMaxRequestBodyBytes() = %d, want 1024", got)
	}
	if got := NewConfig().GetMaxRequestBodyBytes(); got != DefaultMaxRequestBodyBytes {
		t.Fatalf("original config was mutated: got %d", got)
	}
}

func TestNewConfigBodyLimitErrorResolver(t *testing.T) {
	cfg := NewConfig()
	if len(cfg.GetErrorResolvers()) == 0 {
		t.Fatal("expected default body limit error resolver")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = requestWithConfig(cfg, req)

	Error(rec, req, &http.MaxBytesError{Limit: 1024})

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestBodyLimitErrorResolverAllowsChain(t *testing.T) {
	called := false
	cfg := NewConfig().AddErrorResolvers(func(oerr *ResolvedError) *ResolvedError {
		called = true
		oerr.StatusCode = http.StatusTeapot
		return oerr
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = requestWithConfig(cfg, req)

	Error(rec, req, errors.New("some other error"))

	if !called {
		t.Fatal("appended error resolver was not called")
	}
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
}

func TestBodyLimitErrorResolverPrecedesChain(t *testing.T) {
	called := false
	cfg := NewConfig().AddErrorResolvers(func(oerr *ResolvedError) *ResolvedError {
		called = true
		oerr.StatusCode = http.StatusTeapot
		return oerr
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = requestWithConfig(cfg, req)

	Error(rec, req, &http.MaxBytesError{Limit: 100})

	if called {
		t.Fatal("appended error resolver should not run when body limit resolver matches")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestBindOversizedJSONBody(t *testing.T) {
	const limit = 1024
	body := []byte(`{"name":"` + strings.Repeat("x", limit) + `"}`)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithConfig(NewConfig().SetMaxRequestBodyBytes(limit), req)

	var v struct {
		Name string `json:"name"`
	}
	err := Bind(req, &v)
	if err == nil {
		t.Fatal("Bind() expected error for oversized body, got nil")
	}
	re, ok := IsResolvedError(err)
	if !ok {
		t.Fatalf("Bind() error = %T (%v), want *ResolvedError", err, err)
	}
	if re.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("Bind() status = %d, want %d", re.StatusCode, http.StatusRequestEntityTooLarge)
	}
}

func TestBindSmallJSONBody(t *testing.T) {
	body := []byte(`{"name":"alice"}`)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithConfig(NewConfig(), req)

	var v struct {
		Name string `json:"name" validate:"required"`
	}
	if err := Bind(req, &v); err != nil {
		t.Fatalf("Bind() unexpected error: %v", err)
	}
	if v.Name != "alice" {
		t.Fatalf("Bind() name = %q, want alice", v.Name)
	}
}

func TestBindChunkedOversizedBody(t *testing.T) {
	const limit = 512
	body := []byte(`{"name":"` + strings.Repeat("x", limit+100) + `"}`)

	req := &http.Request{
		Method:        http.MethodPost,
		URL:           &url.URL{Path: "/"},
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: -1,
	}
	req = requestWithConfig(NewConfig().SetMaxRequestBodyBytes(limit), req)

	var v struct {
		Name string `json:"name"`
	}
	err := Bind(req, &v)
	if err == nil {
		t.Fatal("Bind() expected error for oversized chunked body, got nil")
	}
	re, ok := IsResolvedError(err)
	if !ok {
		t.Fatalf("Bind() error = %T (%v), want *ResolvedError", err, err)
	}
	if re.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("Bind() status = %d, want %d", re.StatusCode, http.StatusRequestEntityTooLarge)
	}
}

func TestBindContentLengthEarlyReject(t *testing.T) {
	const limit = 512

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.ContentLength = limit + 1
	req = requestWithConfig(NewConfig().SetMaxRequestBodyBytes(limit), req)

	var v struct{}
	err := Bind(req, &v)
	if err == nil {
		t.Fatal("Bind() expected error, got nil")
	}
	re, ok := IsResolvedError(err)
	if !ok {
		t.Fatalf("error = %T, want *ResolvedError", err)
	}
	if re.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", re.StatusCode, http.StatusRequestEntityTooLarge)
	}
	var maxBytesErr *http.MaxBytesError
	if !errors.As(re.Err, &maxBytesErr) {
		t.Fatalf("Err = %T, want *http.MaxBytesError", re.Err)
	}
	if !re.Public() {
		t.Fatal("expected public error")
	}
}

func TestDefaultJSONDecoderWithoutBind(t *testing.T) {
	const limit = 256
	body := []byte(`{"value":"` + strings.Repeat("a", limit) + `"}`)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = requestWithConfig(NewConfig().SetMaxRequestBodyBytes(limit), req)

	var v struct {
		Value string `json:"value"`
	}
	dec := DefaultJSONDecoder()
	if err := dec(req, &v); err == nil {
		t.Fatal("decoder expected error for oversized body, got nil")
	}

	smallBody := []byte(`{"value":"ok"}`)
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(smallBody))
	req = requestWithConfig(NewConfig().SetMaxRequestBodyBytes(limit), req)
	if err := dec(req, &v); err != nil {
		t.Fatalf("decoder unexpected error: %v", err)
	}
	if v.Value != "ok" {
		t.Fatalf("value = %q, want ok", v.Value)
	}
}

func TestUseMaxBodyBytesRejectsOversizedBody(t *testing.T) {
	const limit = 512
	body := bytes.Repeat([]byte("a"), limit+1)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler := UseMaxBodyBytes(limit)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Fatal("expected body read to fail for oversized request")
		}
	}))

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestUseMaxBodyBytesEarlyReject(t *testing.T) {
	const limit = 512

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.ContentLength = limit + 1
	rec := httptest.NewRecorder()

	called := false
	handler := UseMaxBodyBytes(limit)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))

	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("handler should not be called when Content-Length exceeds limit")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestUseMaxBodyBytesAllowsSmallBody(t *testing.T) {
	body := []byte("hello")

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler := UseMaxBodyBytes(DefaultMaxRequestBodyBytes)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, len(body))
		if _, err := r.Body.Read(buf); err != nil {
			t.Fatalf("unexpected read error: %v", err)
		}
		if string(buf) != "hello" {
			t.Fatalf("body = %q, want hello", string(buf))
		}
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUseMaxBodyBytesWithBind(t *testing.T) {
	const limit = 1024
	body := []byte(`{"name":"alice"}`)

	cfg := NewConfig().SetMaxRequestBodyBytes(limit)
	handler := cfg.Use()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Name string `json:"name" validate:"required"`
		}
		if err := Bind(r, &v); err != nil {
			t.Fatalf("Bind() unexpected error: %v", err)
		}
		if v.Name != "alice" {
			t.Fatalf("name = %q, want alice", v.Name)
		}
		limited, ok := isBodyLimited(r)
		if !ok {
			t.Fatal("expected body to be marked limited")
		}
		if limited != limit {
			t.Fatalf("limit marker = %d, want %d", limited, limit)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestConflictingLimitsStricterWins(t *testing.T) {
	const middlewareLimit = 2048
	const bindLimit = 1024
	body := []byte(`{"name":"` + strings.Repeat("a", bindLimit+100) + `"}`)

	cfg := NewConfig().SetMaxRequestBodyBytes(bindLimit)
	handler := UseMaxBodyBytes(middlewareLimit)(cfg.Use()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Name string `json:"name"`
		}
		err := Bind(r, &v)
		if err == nil {
			t.Fatal("Bind() expected error, got nil")
		}
		re, ok := IsResolvedError(err)
		if !ok {
			t.Fatalf("error = %T, want *ResolvedError", err)
		}
		if re.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d", re.StatusCode, http.StatusRequestEntityTooLarge)
		}
	})))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

func TestDisabledBodyLimit(t *testing.T) {
	const bodySize = 2048
	body := bytes.Repeat([]byte("a"), bodySize)

	cfg := NewConfig().SetMaxRequestBodyBytes(0)
	handler := cfg.Use()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}
		if len(data) != bodySize {
			t.Fatalf("read %d bytes, want %d", len(data), bodySize)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestConfigUseProtectsRawBodyRead(t *testing.T) {
	const limit = 512
	body := bytes.Repeat([]byte("b"), limit+1)

	cfg := NewConfig().SetMaxRequestBodyBytes(limit)
	handler := cfg.Use()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Fatal("expected read error for oversized body")
		}
	}))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestBindMultipartOversize(t *testing.T) {
	const limit = 512

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormField("name")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(bytes.Repeat([]byte("x"), limit+100)); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = requestWithConfig(NewConfig().SetMaxRequestBodyBytes(limit), req)

	var v struct {
		Name string `form:"name"`
	}
	err = Bind(req, &v)
	if err == nil {
		t.Fatal("Bind() expected error for oversized multipart body, got nil")
	}
	re, ok := IsResolvedError(err)
	if !ok {
		t.Fatalf("error = %T (%v), want *ResolvedError", err, err)
	}
	if re.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", re.StatusCode, http.StatusRequestEntityTooLarge)
	}
}

func TestBodyLimitErrorResolverPublic(t *testing.T) {
	cfg := NewConfig()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = requestWithConfig(cfg, req)

	maxErr := &http.MaxBytesError{Limit: 100}
	Error(rec, req, maxErr)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}
