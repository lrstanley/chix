// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"bytes"
	"context"
	"encoding"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sseContentType             = "text/event-stream"
	ssePing                    = ": ping\n\n"
	sseDefaultWriteIdleTimeout = 300 * time.Second
	headerLastEventID          = "Last-Event-Id"
)

type contextKeySSELastEventID struct{}

// SSEEvent is a single Server-Sent Event frame.
type SSEEvent struct {
	// ID is the optional event id field. Omitted when empty. CR, LF, and NULL
	// bytes are stripped.
	ID string

	// Event is the optional event type. When empty, user agents dispatch the
	// event as type "message", which is what EventSource.onmessage receives.
	// When set, the frame is a named event: onmessage does not fire; the
	// client must addEventListener(name, ...).
	//
	// Leave empty when the client uses onmessage (typical), or when the event
	// kind lives inside JSON Data. Set it for distinct listeners per name, or
	// to hide events from onmessage.
	//
	// See https://html.spec.whatwg.org/multipage/server-sent-events.html#event-stream-interpretation
	Event string

	// Data is the event payload. Encoding order: string or *string (no quotes;
	// CR, LF, and CRLF become additional data: lines); JSON via
	// [Config.GetJSONEncoder]; [encoding.TextMarshaler]; [fmt.Stringer]. A nil
	// *string is treated as nil Data. Events with nil Data and no other fields
	// are skipped. Panics if Data cannot be encoded (programmer error;
	// recovered by [UseStructuredLogger] if installed).
	Data any

	// Retry is the optional retry field (reconnection time). Omitted when
	// zero. Encoded as ASCII milliseconds.
	Retry time.Duration
}

// SSEConfig is a [net/http.Handler] that serves a Server-Sent Events stream.
// See [UseSSE] for more information.
type SSEConfig struct {
	// ProducerFn generates events for the stream. It must return when ctx is
	// done (select on ctx.Done() versus send). Returning ends the stream: the
	// handler writes any remaining buffered events, then closes the channel.
	// Send only; do not close events. A nil *SSEEvent is skipped.
	//
	// Example:
	//
	//	ProducerFn: func(ctx context.Context, _ *http.Request, events chan<- *SSEEvent) error {
	//		t := time.NewTicker(5 * time.Second)
	//		defer t.Stop()
	//		for {
	//			select {
	//			case <-ctx.Done():
	//				return ctx.Err()
	//			case now := <-t.C:
	//				events <- &SSEEvent{Data: now.Format(time.RFC3339)}
	//			}
	//		}
	//	},
	ProducerFn func(ctx context.Context, r *http.Request, events chan<- *SSEEvent) error

	// HeartbeatInterval, when greater than zero, sends SSE comment keepalives
	// (": ping") immediately after the stream starts and on each tick.
	// Comments are ignored by user agents and are not dispatched as events.
	// 0 disables heartbeats; there is no implicit default. A typical value is
	// 25s, just under many load-balancer idle timeouts.
	//
	// Enabling heartbeats commits to HTTP 200 text/event-stream immediately,
	// so [Error] and 204 from ProducerFn are no longer possible.
	//
	// See https://html.spec.whatwg.org/multipage/server-sent-events.html#authoring-notes
	HeartbeatInterval time.Duration

	// WriteIdleTimeout is the per-write deadline via
	// [http.ResponseController.SetWriteDeadline], overriding [NewServer]'s
	// default 15s WriteTimeout. Defaults to 3*HeartbeatInterval when
	// heartbeats are enabled, otherwise 300s.
	WriteIdleTimeout time.Duration
}

// Validate validates the SSE config and applies defaults. Use this to validate
// the config before using it, otherwise [UseSSE] will panic if an invalid
// config is provided.
func (c *SSEConfig) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}
	if c.ProducerFn == nil {
		return errors.New("ProducerFn is nil")
	}
	if c.HeartbeatInterval < 0 {
		c.HeartbeatInterval = 0
	}
	if c.WriteIdleTimeout <= 0 {
		if c.HeartbeatInterval > 0 {
			c.WriteIdleTimeout = 3 * c.HeartbeatInterval
		} else {
			c.WriteIdleTimeout = sseDefaultWriteIdleTimeout
		}
	}
	return nil
}

// GetSSELastEventID returns the Last-Event-Id value injected by [UseSSE].
//
// See https://html.spec.whatwg.org/multipage/server-sent-events.html#the-last-event-id-header
func GetSSELastEventID(ctx context.Context) string {
	id, _ := ctx.Value(contextKeySSELastEventID{}).(string)
	return id
}

// UseSSE returns a handler that serves a Server-Sent Events stream.
//
// ProducerFn must return when ctx is done. Do not mount
// chi/middleware.Compress on this route; it buffers and breaks flushing.
// [http.Server.WriteTimeout] (15s via [NewServer]) would kill long-lived
// streams; UseSSE resets the write deadline to WriteIdleTimeout after each
// successful write (and once at the start of the request).
//
// EventSource reconnects if a 200 text/event-stream response ends. Returning
// a [*ResolvedError] with status 204 before the stream starts stops
// reconnection
// (https://html.spec.whatwg.org/multipage/server-sent-events.html#server-sent-events-intro).
// Auth belongs in middleware. Headers are delayed until the first event,
// ProducerFn returns, or heartbeats are enabled (which always writes an
// initial comment).
//
// See https://html.spec.whatwg.org/multipage/server-sent-events.html#event-stream-interpretation
func UseSSE(config *SSEConfig) http.Handler {
	if err := config.Validate(); err != nil {
		panic(err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			ErrorWithCode(w, r, http.StatusInternalServerError, errors.New("streaming unsupported"))
			return
		}
		serveSSE(config, w, r, flusher)
	})
}

type sseStream struct {
	w       http.ResponseWriter
	r       *http.Request
	config  *SSEConfig
	encode  JSONEncoder
	flusher http.Flusher
	rc      *http.ResponseController
	started bool
}

func serveSSE(config *SSEConfig, w http.ResponseWriter, r *http.Request, flusher http.Flusher) { //nolint:gocognit,funlen
	ctx := context.WithValue(r.Context(), contextKeySSELastEventID{}, r.Header.Get(headerLastEventID))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	r = r.WithContext(ctx)

	s := &sseStream{
		w:       w,
		r:       r,
		config:  config,
		encode:  GetConfig(ctx).GetJSONEncoder(),
		flusher: flusher,
		rc:      http.NewResponseController(w),
	}
	s.bump()

	ch := make(chan *SSEEvent, 1)
	done := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			if rec := recover(); rec != nil {
				err = fmt.Errorf("sse producer panic: %v", rec)
			}
			done <- err
		}()
		err = config.ProducerFn(ctx, r, ch)
	}()

	producerDone := false
	defer func() {
		cancel()
		if !producerDone {
			_ = waitSSEProducer(done, ch)
		}
		close(ch)
	}()

	var tickerC <-chan time.Time
	if config.HeartbeatInterval > 0 {
		s.start()
		if err := s.write([]byte(ssePing)); err != nil {
			return
		}
		ticker := time.NewTicker(config.HeartbeatInterval)
		defer ticker.Stop()
		tickerC = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-done:
			producerDone = true
			s.onProducerDone(err, ch)
			return
		case ev := <-ch:
			if err := s.dispatch(ev); err != nil {
				return
			}
		case <-tickerC:
			if err := s.write([]byte(ssePing)); err != nil {
				return
			}
		}
	}
}

func (s *sseStream) bump() {
	_ = s.rc.SetWriteDeadline(time.Now().Add(s.config.WriteIdleTimeout))
}

func (s *sseStream) start() {
	if s.started {
		return
	}
	h := s.w.Header()
	h.Set("Content-Type", sseContentType)
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	if s.r.ProtoMajor < 2 {
		h.Set("Connection", "keep-alive")
	}
	s.w.WriteHeader(http.StatusOK)
	s.flusher.Flush()
	s.bump()
	s.started = true
}

func (s *sseStream) write(p []byte) error {
	if _, err := s.w.Write(p); err != nil {
		return err
	}
	s.bump()
	s.flusher.Flush()
	return nil
}

func (s *sseStream) dispatch(ev *SSEEvent) error {
	if ev == nil || !sseEventMeaningful(ev) {
		return nil
	}

	buf := renderBufferPool.Get()
	defer renderBufferPool.Put(buf)
	writeSSEFrame(buf, s.encode, s.r, ev)

	if !s.started {
		s.start()
	}
	return s.write(buf.Bytes())
}

func (s *sseStream) onProducerDone(err error, ch <-chan *SSEEvent) {
	if !s.started && err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if rerr, ok := IsResolvedError(err); ok && rerr.StatusCode == http.StatusNoContent {
			s.w.WriteHeader(http.StatusNoContent)
			return
		}
		Error(s.w, s.r, err)
		return
	}

	_ = drainSSEEvents(ch, s.dispatch)
	if !s.started {
		s.start()
		return
	}
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		if rerr, ok := IsResolvedError(err); ok {
			SetLogError(s.r.Context(), rerr)
		} else {
			SetLogError(s.r.Context(), &ResolvedError{Err: err})
		}
	}
}

func waitSSEProducer(done <-chan error, ch <-chan *SSEEvent) error {
	for {
		select {
		case err := <-done:
			return err
		case <-ch:
		}
	}
}

func drainSSEEvents(ch <-chan *SSEEvent, write func(*SSEEvent) error) error {
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			if err := write(ev); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func sseEventMeaningful(ev *SSEEvent) bool {
	if ev.ID != "" || ev.Event != "" || ev.Retry > 0 {
		return true
	}
	if ev.Data == nil {
		return false
	}
	s, ok := ev.Data.(*string)
	return !ok || s != nil
}

func stripSSEField(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', 0:
			return -1
		default:
			return r
		}
	}, s)
}

func writeSSEFrame(buf *bytes.Buffer, enc JSONEncoder, r *http.Request, ev *SSEEvent) {
	if id := stripSSEField(ev.ID); id != "" {
		buf.WriteString("id: ")
		buf.WriteString(id)
		buf.WriteByte('\n')
	}
	if name := stripSSEField(ev.Event); name != "" {
		buf.WriteString("event: ")
		buf.WriteString(name)
		buf.WriteByte('\n')
	}
	if ev.Retry > 0 {
		buf.WriteString("retry: ")
		buf.WriteString(strconv.FormatInt(ev.Retry.Milliseconds(), 10))
		buf.WriteByte('\n')
	}
	if ev.Data == nil {
		buf.WriteByte('\n')
		return
	}
	if s, ok := ev.Data.(*string); ok && s == nil {
		buf.WriteByte('\n')
		return
	}

	payload := encodeSSEData(enc, r, ev.Data)
	payload = strings.ReplaceAll(payload, "\r\n", "\n")
	payload = strings.ReplaceAll(payload, "\r", "\n")
	for {
		line, rest, found := strings.Cut(payload, "\n")
		buf.WriteString("data: ")
		buf.WriteString(line)
		buf.WriteByte('\n')
		if !found {
			break
		}
		payload = rest
	}
	buf.WriteByte('\n')
}

type sseJSONWriter struct {
	buf    *bytes.Buffer
	header http.Header
}

func (w *sseJSONWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *sseJSONWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

func (w *sseJSONWriter) WriteHeader(int) {}

func encodeSSEData(enc JSONEncoder, r *http.Request, data any) string {
	switch v := data.(type) {
	case string:
		return v
	case *string:
		return *v
	}

	buf := renderBufferPool.Get()
	defer renderBufferPool.Put(buf)
	if err := enc(&sseJSONWriter{buf: buf}, r, data); err == nil {
		return buf.String()
	}

	if tm, ok := data.(encoding.TextMarshaler); ok {
		b, err := tm.MarshalText()
		if err == nil {
			return string(b)
		}
	}

	if s, ok := data.(fmt.Stringer); ok {
		return s.String()
	}

	panic("chix: SSEEvent.Data type " + fmt.Sprintf("%T", data) + " is not JSON-marshalable, encoding.TextMarshaler, or fmt.Stringer")
}
