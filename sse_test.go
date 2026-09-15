// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in the
// LICENSE file.

package chix

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestSSEConfig_Validate(t *testing.T) {
	t.Parallel()

	nop := func(context.Context, *http.Request, chan<- *SSEEvent) error { return nil }

	tests := []struct {
		name    string
		config  *SSEConfig
		wantErr string
		after   func(*testing.T, *SSEConfig)
	}{
		{
			name:    "nil-config",
			wantErr: "nil",
		},
		{
			name:    "nil-producer",
			config:  &SSEConfig{},
			wantErr: "ProducerFn",
		},
		{
			name:   "negative-heartbeat-disables",
			config: &SSEConfig{ProducerFn: nop, HeartbeatInterval: -time.Second},
			after: func(t *testing.T, c *SSEConfig) {
				t.Helper()
				if c.HeartbeatInterval != 0 {
					t.Fatalf("HeartbeatInterval = %s, want 0", c.HeartbeatInterval)
				}
				if c.WriteIdleTimeout != 300*time.Second {
					t.Fatalf("WriteIdleTimeout = %s, want 300s", c.WriteIdleTimeout)
				}
			},
		},
		{
			name:   "negative-write-idle-gets-default",
			config: &SSEConfig{ProducerFn: nop, WriteIdleTimeout: -time.Second},
			after: func(t *testing.T, c *SSEConfig) {
				t.Helper()
				if c.WriteIdleTimeout != 300*time.Second {
					t.Fatalf("WriteIdleTimeout = %s, want 300s", c.WriteIdleTimeout)
				}
			},
		},
		{
			name:   "heartbeat-zero-stays-disabled",
			config: &SSEConfig{ProducerFn: nop},
			after: func(t *testing.T, c *SSEConfig) {
				t.Helper()
				if c.HeartbeatInterval != 0 {
					t.Fatalf("HeartbeatInterval = %s, want 0", c.HeartbeatInterval)
				}
				if c.WriteIdleTimeout != 300*time.Second {
					t.Fatalf("WriteIdleTimeout = %s, want 300s", c.WriteIdleTimeout)
				}
			},
		},
		{
			name:   "heartbeat-sets-write-idle-default",
			config: &SSEConfig{ProducerFn: nop, HeartbeatInterval: 25 * time.Second},
			after: func(t *testing.T, c *SSEConfig) {
				t.Helper()
				if c.WriteIdleTimeout != 75*time.Second {
					t.Fatalf("WriteIdleTimeout = %s, want 75s", c.WriteIdleTimeout)
				}
			},
		},
		{
			name:   "explicit-write-idle-kept",
			config: &SSEConfig{ProducerFn: nop, HeartbeatInterval: time.Second, WriteIdleTimeout: 10 * time.Second},
			after: func(t *testing.T, c *SSEConfig) {
				t.Helper()
				if c.WriteIdleTimeout != 10*time.Second {
					t.Fatalf("WriteIdleTimeout = %s, want 10s", c.WriteIdleTimeout)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.config.Validate()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.after != nil {
				tt.after(t, tt.config)
			}
		})
	}
}

func TestUseSSE_panicsOnInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config *SSEConfig
	}{
		{name: "nil", config: nil},
		{name: "nil-producer", config: &SSEConfig{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
			}()
			_ = UseSSE(tt.config)
		})
	}
}

func sseSend(events ...*SSEEvent) func(context.Context, *http.Request, chan<- *SSEEvent) error {
	return func(ctx context.Context, _ *http.Request, ch chan<- *SSEEvent) error {
		for _, ev := range events {
			select {
			case ch <- ev:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
}

type jsonOverStringer struct {
	N int `json:"n"`
}

func (jsonOverStringer) String() string { return "stringer" }

type textAfterJSON struct{}

func (textAfterJSON) MarshalJSON() ([]byte, error) { return nil, errors.New("no json") }

func (textAfterJSON) MarshalText() ([]byte, error) { return []byte("from-text"), nil }

func (textAfterJSON) String() string { return "from-stringer" }

type stringerAfter struct{}

func (stringerAfter) MarshalJSON() ([]byte, error) { return nil, errors.New("no json") }

func (stringerAfter) MarshalText() ([]byte, error) { return nil, errors.New("no text") }

func (stringerAfter) String() string { return "from-stringer" }

func TestUseSSE(t *testing.T) {
	t.Parallel()

	hello := "hello"
	var nilString *string

	tests := []struct {
		name           string
		config         *SSEConfig
		req            func() *http.Request
		wrap           func(http.Handler) http.Handler
		wantStatus     int
		wantHeaders    map[string]string
		forbidHeaders  []string
		wantBody       string
		forbidBody     string
		contentTypeHas string
	}{
		{
			name: "headers-http1",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: "ok"}),
			},
			wantStatus: http.StatusOK,
			wantHeaders: map[string]string{
				"Content-Type":      sseContentType,
				"Cache-Control":     "no-cache",
				"X-Accel-Buffering": "no",
				"Connection":        "keep-alive",
			},
			wantBody: "data: ok\n\n",
		},
		{
			name: "headers-http2-omits-connection",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: "ok"}),
			},
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "http://example.com/events", http.NoBody)
				req.Proto = "HTTP/2.0"
				req.ProtoMajor = 2
				req.ProtoMinor = 0
				return req
			},
			wantStatus:    http.StatusOK,
			forbidHeaders: []string{"Connection"},
			wantBody:      "data: ok\n\n",
		},
		{
			name: "last-event-id",
			config: &SSEConfig{
				ProducerFn: func(ctx context.Context, _ *http.Request, ch chan<- *SSEEvent) error {
					return sseSend(&SSEEvent{Data: GetSSELastEventID(ctx)})(ctx, nil, ch)
				},
			},
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "http://example.com/events", http.NoBody)
				req.Header.Set("Last-Event-ID", "abc-123")
				return req
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: abc-123\n\n",
		},
		{
			name: "string-data-as-is",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: "hello"}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: hello\n\n",
		},
		{
			name: "string-pointer-as-is",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: &hello}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: hello\n\n",
		},
		{
			name: "nil-string-pointer-skipped-then-event",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: nilString}, &SSEEvent{Data: "after"}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: after\n\n",
		},
		{
			name: "json-over-stringer",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: jsonOverStringer{N: 1}}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: {\"n\":1}\n\n",
		},
		{
			name: "json-fail-then-text-marshaler",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: textAfterJSON{}}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: from-text\n\n",
		},
		{
			name: "json-and-text-fail-then-stringer",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: stringerAfter{}}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: from-stringer\n\n",
		},
		{
			name: "multiline-data",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: "line1\nline2"}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: line1\ndata: line2\n\n",
		},
		{
			name: "interior-cr-data",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: "hello\revent: injected"}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: hello\ndata: event: injected\n\n",
			forbidBody: "\r",
		},
		{
			name: "crlf-data",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: "line1\r\nline2"}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: line1\ndata: line2\n\n",
			forbidBody: "\r",
		},
		{
			name: "mixed-cr-lf-data",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: "a\rb\nc"}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: a\ndata: b\ndata: c\n\n",
			forbidBody: "\r",
		},
		{
			name: "last-event-id-cr-in-data",
			config: &SSEConfig{
				ProducerFn: func(ctx context.Context, _ *http.Request, ch chan<- *SSEEvent) error {
					return sseSend(&SSEEvent{Data: GetSSELastEventID(ctx)})(ctx, nil, ch)
				},
			},
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "http://example.com/events", http.NoBody)
				req.Header.Set("Last-Event-ID", "hello\revent: injected")
				return req
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: hello\ndata: event: injected\n\n",
			forbidBody: "\r",
		},
		{
			name: "json-cr-escaped",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: map[string]string{"s": "hello\revent: injected"}}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: {\"s\":\"hello\\revent: injected\"}\n\n",
			forbidBody: "\r",
		},
		{
			name: "id-event-retry-and-strip",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{
					ID:    "a\nb\x00c",
					Event: "foo\rbar",
					Data:  "x",
					Retry: 3 * time.Second,
				}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "id: abc\nevent: foobar\nretry: 3000\ndata: x\n\n",
		},
		{
			name: "nil-event-skipped",
			config: &SSEConfig{
				ProducerFn: sseSend(nil, &SSEEvent{Data: "kept"}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: kept\n\n",
		},
		{
			name: "empty-event-skipped",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{}, &SSEEvent{Data: "kept"}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: kept\n\n",
		},
		{
			name: "producer-return-drains",
			config: &SSEConfig{
				ProducerFn: sseSend(&SSEEvent{Data: "a"}, &SSEEvent{Data: "b"}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: a\n\ndata: b\n\n",
		},
		{
			name: "heartbeats-disabled",
			config: &SSEConfig{
				HeartbeatInterval: 0,
				ProducerFn:        sseSend(&SSEEvent{Data: "hello"}),
			},
			wantStatus: http.StatusOK,
			wantBody:   "data: hello\n\n",
			forbidBody: ": ping",
		},
		{
			name: "204-before-stream",
			config: &SSEConfig{
				ProducerFn: func(context.Context, *http.Request, chan<- *SSEEvent) error {
					return &ResolvedError{Err: errors.New("no content"), StatusCode: http.StatusNoContent}
				},
			},
			wantStatus:     http.StatusNoContent,
			forbidHeaders:  []string{"Content-Type"},
			contentTypeHas: "",
			wantBody:       "",
		},
		{
			name: "json-error-before-stream",
			config: &SSEConfig{
				ProducerFn: func(context.Context, *http.Request, chan<- *SSEEvent) error {
					return &ResolvedError{Err: errors.New("nope"), StatusCode: http.StatusBadRequest}
				},
			},
			wrap: func(next http.Handler) http.Handler {
				return NewConfig().SetAPIBasePath("/api").Use()(next)
			},
			req: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "http://example.com/api/events", http.NoBody)
			},
			wantStatus:     http.StatusBadRequest,
			contentTypeHas: "application/json",
			wantBody:       "nope",
		},
		{
			name: "producer-panic-before-stream",
			config: &SSEConfig{
				ProducerFn: func(context.Context, *http.Request, chan<- *SSEEvent) error {
					panic("boom")
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "http://example.com/events", http.NoBody)
			if tt.req != nil {
				req = tt.req()
			}

			h := UseSSE(tt.config)
			if tt.wrap != nil {
				h = tt.wrap(h)
			}

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			resp := rec.Result()
			defer resp.Body.Close()

			if tt.wantStatus != 0 && resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			for k, v := range tt.wantHeaders {
				if got := resp.Header.Get(k); got != v {
					t.Fatalf("header %s = %q, want %q", k, got, v)
				}
			}
			for _, k := range tt.forbidHeaders {
				if got := resp.Header.Get(k); got != "" {
					t.Fatalf("header %s = %q, want empty", k, got)
				}
			}
			if tt.contentTypeHas != "" && !strings.Contains(resp.Header.Get("Content-Type"), tt.contentTypeHas) {
				t.Fatalf("content-type %q, want substring %q", resp.Header.Get("Content-Type"), tt.contentTypeHas)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			got := string(body)
			if tt.wantBody != "" && !strings.Contains(got, tt.wantBody) {
				t.Fatalf("body %q, want substring %q", got, tt.wantBody)
			}
			if tt.wantBody == "" && tt.wantStatus == http.StatusNoContent && got != "" {
				t.Fatalf("body %q, want empty", got)
			}
			if tt.forbidBody != "" && strings.Contains(got, tt.forbidBody) {
				t.Fatalf("body %q, forbid substring %q", got, tt.forbidBody)
			}
		})
	}
}

func TestUseSSE_encodePanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	UseSSE(&SSEConfig{
		ProducerFn: sseSend(&SSEEvent{Data: make(chan int)}),
	}).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://example.com/events", http.NoBody))
}

type nopFlushWriter struct {
	rec *httptest.ResponseRecorder
}

func (w nopFlushWriter) Header() http.Header         { return w.rec.Header() }
func (w nopFlushWriter) Write(p []byte) (int, error) { return w.rec.Write(p) }
func (w nopFlushWriter) WriteHeader(status int)      { w.rec.WriteHeader(status) }

func TestUseSSE_requiresFlusher(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	UseSSE(&SSEConfig{
		ProducerFn: sseSend(&SSEEvent{Data: "ok"}),
	}).ServeHTTP(nopFlushWriter{rec: rec}, httptest.NewRequest(http.MethodGet, "http://example.com/events", http.NoBody))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestUseSSE_heartbeats(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := UseSSE(&SSEConfig{
			HeartbeatInterval: time.Second,
			ProducerFn: func(ctx context.Context, _ *http.Request, _ chan<- *SSEEvent) error {
				<-ctx.Done()
				return nil
			},
		})
		srv := httptest.NewTestServer(t, h)
		resp, err := srv.Client().Get("http://example.com/events")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		buf := make([]byte, 64)
		n, err := resp.Body.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(buf[:n]); got != ssePing {
			t.Fatalf("initial comment = %q, want %q", got, ssePing)
		}

		synctest.Sleep(time.Second)
		n, err = resp.Body.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(buf[:n]); got != ssePing {
			t.Fatalf("tick comment = %q, want %q", got, ssePing)
		}
	})
}

func TestUseSSE_cancelUnblocksSend(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		finished := make(chan struct{})
		h := UseSSE(&SSEConfig{
			ProducerFn: func(ctx context.Context, _ *http.Request, events chan<- *SSEEvent) error {
				defer close(finished)
				select {
				case events <- &SSEEvent{Data: "one"}:
				case <-ctx.Done():
					return ctx.Err()
				}
				events <- &SSEEvent{Data: "two"}
				events <- &SSEEvent{Data: "three"}
				<-ctx.Done()
				return ctx.Err()
			},
		})

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/events", http.NoBody).WithContext(ctx)
		go h.ServeHTTP(httptest.NewRecorder(), req)
		synctest.Wait()
		cancel()
		synctest.Wait()

		select {
		case <-finished:
		default:
			t.Fatal("producer did not return after context cancel")
		}
	})
}
