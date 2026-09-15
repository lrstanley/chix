// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/go-chi/chi/v5"
	"github.com/lrstanley/chix/v2/internal/logging"
)

func findSlogAttr(t *testing.T, attrs []slog.Attr, key string) *slog.Attr {
	t.Helper()
	for _, attr := range attrs {
		if attr.Key == key {
			return &attr
		}
	}
	return nil
}

func assertSlogRecordCount(t *testing.T, records [][]slog.Attr, count int) {
	t.Helper()
	if len(records) != count {
		t.Errorf("expected %d records, got %d", count, len(records))
	}
}

func assertFindSlogAttr(t *testing.T, attrs []slog.Attr, key string, expected slog.Value) {
	t.Helper()
	found := findSlogAttr(t, attrs, key)
	if found == nil {
		t.Errorf("expected attribute %s not found", key)
		return
	}
	if !reflect.DeepEqual(found.Value, expected) {
		t.Errorf("expected %v, got %v", expected, found)
	}
}

var loggerTestCases = []struct {
	name         string
	config       *Config
	middleware   []func(http.Handler) http.Handler
	req          func() *http.Request
	status       int
	bodyContents string
	validate     func(t *testing.T, records [][]slog.Attr)
}{
	{
		name: "simple",
		middleware: []func(http.Handler) http.Handler{
			UseRequestID(),
			UseStructuredLogger(&LogConfig{
				Schema: MustGetLogSchema(LogSchemaSimple),
			}),
		},
		req: func() *http.Request {
			req := httptest.NewRequest(http.MethodGet, "http://example.com/json", http.NoBody)
			req.Header.Set("X-Request-ID", "123")
			return req
		},
		status:       http.StatusOK,
		bodyContents: "Hello, world!",
		validate: func(t *testing.T, records [][]slog.Attr) {
			assertSlogRecordCount(t, records, 1)
			for _, attrs := range records {
				assertFindSlogAttr(t, attrs, "req.id", slog.StringValue("123"))
				assertFindSlogAttr(t, attrs, "resp.status", slog.IntValue(http.StatusOK))
			}
		},
	},
	{
		name: "simple-large",
		middleware: []func(http.Handler) http.Handler{
			UseRequestID(),
			UseStructuredLogger(&LogConfig{
				Schema: MustGetLogSchema(LogSchemaSimple),
			}),
		},
		req: func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "http://example.com/json-large", http.NoBody)
		},
		status:       http.StatusOK,
		bodyContents: "foo bar baz abc1234567890",
		validate: func(t *testing.T, records [][]slog.Attr) {
			assertSlogRecordCount(t, records, 1)
			for _, attrs := range records {
				assertFindSlogAttr(t, attrs, "resp.status", slog.IntValue(http.StatusOK))
			}
		},
	},
	{
		name: "simple-large-req-resp-body",
		middleware: []func(http.Handler) http.Handler{
			UseRequestID(),
			UseStructuredLogger(&LogConfig{
				Schema:       MustGetLogSchema(LogSchemaSimple),
				RequestBody:  func(_ *http.Request) bool { return true },
				ResponseBody: func(_ *http.Request) bool { return true },
				MaxBodySize:  30,
			}),
		},
		req: func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "http://example.com/json-large", bytes.NewBufferString(`{"foo": "bar baz abc1234567890"}`))
		},
		status: http.StatusOK,
		validate: func(t *testing.T, records [][]slog.Attr) {
			assertSlogRecordCount(t, records, 1)
			for _, attrs := range records {
				assertFindSlogAttr(t, attrs, "resp.status", slog.IntValue(http.StatusOK))
				assertFindSlogAttr(t, attrs, "req.body", slog.StringValue(`{"foo": "bar baz[...truncated]`))
				assertFindSlogAttr(t, attrs, "resp.body", slog.StringValue(`[{"Foo":"foo bar[...truncated]`))
			}
		},
	},
}

func TestUseStructuredLogger(t *testing.T) {
	for _, tt := range loggerTestCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config == nil {
				tt.config = NewConfig()
			}

			logHandler := &logging.MockHandler{Store: true}
			tt.config = tt.config.SetLogger(slog.New(logHandler))

			router := newMockRouter(t, append(
				[]func(http.Handler) http.Handler{tt.config.Use()},
				tt.middleware...,
			))

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, tt.req())
			resp := rec.Result()

			if tt.status != 0 && resp.StatusCode != tt.status {
				t.Errorf("expected status code %d, got %d", tt.status, resp.StatusCode)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("error reading body: %v", err)
			}
			resp.Body.Close()

			if tt.bodyContents != "" && !strings.Contains(string(body), tt.bodyContents) {
				t.Errorf("expected body contents %s, got %s", tt.bodyContents, string(body))
			}

			if tt.validate != nil {
				for _, attrs := range logHandler.Records {
					var out []string
					for _, attr := range attrs {
						out = append(out, fmt.Sprintf("%s=%v", attr.Key, attr.Value))
					}
					t.Logf("received record with attrs: %s", strings.Join(out, ", "))
				}
				tt.validate(t, logHandler.Records)
			}
		})
	}
}

type captureSourceHandler struct {
	got *slog.Source
}

func (h *captureSourceHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureSourceHandler) Handle(_ context.Context, r slog.Record) error {
	h.got = r.Source()
	return nil
}

func (h *captureSourceHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *captureSourceHandler) WithGroup(string) slog.Handler { return h }

func TestUseStructuredLogger_sseAndWebsocketStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		req       func() *http.Request
		handler   http.Handler
		wantStart string
	}{
		{
			name: "sse",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "http://example.com/events", http.NoBody)
				req.Header.Set("Accept", "text/event-stream")
				return req
			},
			handler: UseSSE(&SSEConfig{
				ProducerFn: func(_ context.Context, _ *http.Request, events chan<- *SSEEvent) error {
					events <- &SSEEvent{Data: "ok"}
					return nil
				},
			}),
			wantStart: "GET /events => received",
		},
		{
			name: "websocket",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", http.NoBody)
				req.Header.Set("Connection", "Upgrade")
				req.Header.Set("Upgrade", "websocket")
				return req
			},
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusSwitchingProtocols)
			}),
			wantStart: "GET /ws => received",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logHandler := &logging.MockHandler{Store: true}
			cfg := NewConfig().SetLogger(slog.New(logHandler))
			h := cfg.Use()(UseStructuredLogger(DefaultLogConfig())(tt.handler))

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, tt.req())

			if len(logHandler.Messages) < 2 {
				t.Fatalf("messages = %v, want at least 2", logHandler.Messages)
			}
			if logHandler.Messages[0] != tt.wantStart {
				t.Fatalf("start message = %q, want %q", logHandler.Messages[0], tt.wantStart)
			}
			if !strings.Contains(logHandler.Messages[1], "=>") {
				t.Fatalf("completion message = %q, want status line", logHandler.Messages[1])
			}
		})
	}
}

func TestUseStructuredLogger_sseCancelNotClientAborted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logHandler := &logging.MockHandler{Store: true}
		cfg := NewConfig().SetLogger(slog.New(logHandler))
		h := cfg.Use()(UseStructuredLogger(DefaultLogConfig())(UseSSE(&SSEConfig{
			ProducerFn: func(ctx context.Context, _ *http.Request, events chan<- *SSEEvent) error {
				select {
				case events <- &SSEEvent{Data: "ok"}:
				case <-ctx.Done():
					return ctx.Err()
				}
				<-ctx.Done()
				return nil
			},
		})))

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/events", http.NoBody).WithContext(ctx)
		req.Header.Set("Accept", "text/event-stream")

		go h.ServeHTTP(httptest.NewRecorder(), req)
		synctest.Wait()
		cancel()
		synctest.Wait()

		if len(logHandler.Messages) < 2 {
			t.Fatalf("messages = %v, want start and completion", logHandler.Messages)
		}
		if logHandler.Messages[0] != "GET /events => received" {
			t.Fatalf("start message = %q", logHandler.Messages[0])
		}
		for _, attrs := range logHandler.Records {
			for _, attr := range attrs {
				if attr.Key == "error" && strings.Contains(attr.Value.String(), "client disconnected before response was sent") {
					t.Fatalf("completion tagged ClientAborted: %v", attrs)
				}
				if attr.Value.String() == "ClientAborted" {
					t.Fatalf("completion tagged ClientAborted: %v", attrs)
				}
			}
		}
	})
}

func TestUseStructuredLogger_panicLogSource(t *testing.T) {
	srcCapture := &captureSourceHandler{}
	cfg := NewConfig().SetLogger(slog.New(srcCapture))
	r := chi.NewRouter()
	r.Use(cfg.Use())
	r.Use(UseStructuredLogger(DefaultLogConfig()))
	r.Get("/panic", func(_ http.ResponseWriter, _ *http.Request) {
		panic("test panic for source attribution")
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://example.com/panic", http.NoBody))
	if srcCapture.got == nil {
		t.Fatal("expected non-nil source from slog.Record")
	}
	if !strings.Contains(srcCapture.got.File, "logger_test.go") {
		t.Fatalf("expected source file to be the panicking handler (logger_test.go), got %q", srcCapture.got.File)
	}
}

func BenchmarkUseStructuredLogger(b *testing.B) {
	for _, tt := range loggerTestCases {
		b.Run(tt.name, func(b *testing.B) {
			if tt.config == nil {
				tt.config = NewConfig()
			}

			logHandler := &logging.MockHandler{Store: false}
			tt.config = tt.config.SetLogger(slog.New(logHandler))

			router := newMockRouter(b, append(
				[]func(http.Handler) http.Handler{tt.config.Use()},
				tt.middleware...,
			))

			var rec *httptest.ResponseRecorder
			var req *http.Request

			for b.Loop() {
				rec = httptest.NewRecorder()
				req = tt.req()
				router.ServeHTTP(rec, req)
				_ = rec.Result().Body.Close()
			}
		})
	}
}
