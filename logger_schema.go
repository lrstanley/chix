// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lrstanley/chix/v2/internal/logging"
)

type LogSchemaID string

const (
	// LogSchemaSimple represents a simple log schema that works well for local
	// development, and is also the default schema.
	LogSchemaSimple LogSchemaID = "simple"

	// LogSchemaECS represents the Elastic Common Schema (ECS) version 9.0.0.
	//
	// Reference: https://www.elastic.co/docs/reference/ecs/ecs-http
	LogSchemaECS LogSchemaID = "ecs"

	// LogSchemaLegacy represents the legacy json schema used by chix v1, as close
	// as possible (some fields missing, e.g. ray_id, country (from Cf-Ipcountry),
	// "src").
	LogSchemaLegacy LogSchemaID = "legacy"
)

var BuiltinLogSchemas = []LogSchemaID{
	LogSchemaSimple,
	LogSchemaECS,
	LogSchemaLegacy,
}

func MustGetLogSchema(id LogSchemaID) *LogSchema {
	schema, err := GetLogSchema(id)
	if err != nil {
		panic(err)
	}
	return schema
}

func GetLogSchema(id LogSchemaID) (*LogSchema, error) {
	if id == "" {
		id = LogSchemaSimple
	}

	switch id {
	case LogSchemaECS:
		return &LogSchema{
			Timestamp:          "@timestamp",
			Level:              "log.level",
			Message:            "message",
			ErrorMessage:       "error.message",
			ErrorType:          "error.type",
			ErrorStackTrace:    "error.stack_trace",
			SourceFile:         "log.origin.file.name",
			SourceLine:         "log.origin.file.line",
			SourceFunction:     "log.origin.function",
			RequestURL:         "url.full",
			RequestMethod:      "http.request.method",
			RequestPath:        "url.path",
			RequestRemoteIP:    "client.ip",
			RequestHost:        "url.domain",
			RequestScheme:      "url.scheme",
			RequestProto:       "http.version",
			RequestHeaders:     "http.request.headers",
			RequestBody:        "http.request.body.content",
			RequestBytes:       "http.request.body.bytes",
			RequestBytesUnread: "http.request.body.unread.bytes",
			RequestID:          "http.request.id",
			RequestUserAgent:   "user_agent.original",
			RequestReferer:     "http.request.referrer",
			ResponseHeaders:    "http.response.headers",
			ResponseBody:       "http.response.body.content",
			ResponseStatus:     "http.response.status_code",
			ResponseDuration:   "event.duration",
			ResponseDurationFormat: func(key string, duration time.Duration) slog.Attr {
				// https://www.elastic.co/docs/reference/ecs/ecs-event#field-event-duration
				return slog.Int64(key, duration.Nanoseconds())
			},
			ResponseBytes: "http.response.body.bytes",
		}, nil
	case LogSchemaSimple:
		return &LogSchema{
			Message:          "msg",
			ErrorMessage:     "error",
			Errors:           "errors",
			ErrorStackTrace:  "stack",
			SourceFile:       "src.file",
			SourceLine:       "src.line",
			SourceFunction:   "src.fn",
			RequestRemoteIP:  "req.ip",
			RequestHeaders:   "req.headers",
			RequestBody:      "req.body",
			RequestBytes:     "req.bytes",
			RequestUserAgent: "req.ua",
			RequestID:        "req.id",
			ResponseStatus:   "resp.status",
			ResponseBytes:    "resp.bytes",
			ResponseBody:     "resp.body",
		}, nil
	case LogSchemaLegacy:
		return &LogSchema{
			Message:          "msg",
			ErrorMessage:     "error",
			ErrorStackTrace:  "stack",
			Errors:           "errors",
			RequestID:        "rid",
			RequestRemoteIP:  "ip",
			RequestHost:      "host",
			RequestProto:     "proto",
			RequestMethod:    "method",
			RequestUserAgent: "ua",
			RequestBytes:     "bytes_in",
			ResponseStatus:   "code",
			ResponseDuration: "duration_ms",
			ResponseDurationFormat: func(key string, duration time.Duration) slog.Attr {
				return slog.Int64(key, duration.Milliseconds())
			},
			ResponseBytes: "bytes_out",
		}, nil
	default:
		return nil, fmt.Errorf("unknown log schema: %s", id)
	}
}

// LogSchema defines the mapping of semantic log fields to their corresponding
// field names in different logging systems and standards. If a field is not
// provided, it will not be included in the log entry.
//
// Use ">" as a delimiter for objects that should be nested. For example:
//
//   - `req>body` & `req>ua`
//   - becomes: `{"req": {"body": "...", "ua": "..."}}`
type LogSchema struct {
	// Base attributes for core logging information.
	Timestamp       string // Timestamp of the entry. Only applies when registering the schema as a ReplaceAttr handler.
	Level           string // Log level. Only applies when registering the schema as a ReplaceAttr handler.
	Message         string // Primary log message. Only applies when registering the schema as a ReplaceAttr handler.
	ErrorMessage    string // Error message when an error occurs.
	Errors          string // Multiple errors, as an array.
	ErrorType       string // Low-cardinality error type (e.g. "ClientAborted", "ValidationError")
	ErrorStackTrace string // Stack trace for panic or error

	// Source code location attributes for tracking origin of log statements.
	SourceFile     string // Source file name where the log originated. Only applies when registering the schema as a ReplaceAttr handler.
	SourceLine     string // Line number in the source file. Only applies when registering the schema as a ReplaceAttr handler.
	SourceFunction string // Function name where the log originated. Only applies when registering the schema as a ReplaceAttr handler.

	// Request attributes for the incoming HTTP request.
	RequestURL         string // Full request URL.
	RequestMethod      string // HTTP method (e.g. GET, POST).
	RequestPath        string // URL path component.
	RequestRemoteIP    string // Client IP address.
	RequestHost        string // Host header value.
	RequestScheme      string // URL scheme (http, https).
	RequestProto       string // HTTP protocol version (e.g. HTTP/1.1, HTTP/2).
	RequestHeaders     string // Selected request headers.
	RequestBody        string // Request body content, if enabled.
	RequestBytes       string // Size of request body in bytes.
	RequestBytesUnread string // Unread bytes in request body.
	RequestUserAgent   string // User-Agent header value.
	RequestReferer     string // Referer header value.
	RequestID          string // Request ID, if present.

	// Response attributes for the HTTP response.
	ResponseHeaders        string // Selected response headers
	ResponseBody           string // Response body content, if enabled.
	ResponseStatus         string // HTTP status code
	ResponseDuration       string // Request processing duration
	ResponseDurationFormat func(key string, duration time.Duration) slog.Attr
	ResponseBytes          string // Size of response body in bytes

	hasGroupDelimiter atomic.Bool
}

// checkHasGroupDelimiter checks if the schema has any fields that contain the group
// delimiter.
func (s *LogSchema) checkHasGroupDelimiter() {
	v := reflect.Indirect(reflect.ValueOf(s))
	for _, field := range v.Fields() {
		if !field.CanInterface() {
			continue
		}
		intf := field.Interface()
		if reflect.TypeOf(intf).Kind() != reflect.String {
			continue
		}
		if strings.Contains(intf.(string), logging.AttrGroupDelimiter) { //nolint:errcheck
			s.hasGroupDelimiter.Store(true)
			return
		}
	}
	s.hasGroupDelimiter.Store(false)
}

// ReplaceAttr returns transforms standard slog attribute names to the schema format.
func (s *LogSchema) ReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}

	hasGroupDelimiter := s.hasGroupDelimiter.Load()

	switch a.Key {
	case slog.TimeKey:
		if s.Timestamp == "" {
			return a
		}
		return slog.String(s.Timestamp, a.Value.Time().Format(time.RFC3339))
	case slog.LevelKey:
		if s.Level == "" {
			return a
		}
		return slog.String(s.Level, a.Value.String())
	case slog.MessageKey:
		if s.Message == "" {
			return a
		}
		return slog.String(s.Message, a.Value.String())
	case slog.SourceKey:
		source, ok := a.Value.Any().(*slog.Source)
		if !ok {
			return a
		}

		if s.SourceFile == "" {
			for i := range sourceIgnoreContains {
				if strings.Contains(source.File, sourceIgnoreContains[i]) {
					return slog.Attr{}
				}
			}
			return a
		}

		if !hasGroupDelimiter {
			// TODO: empty group name gets dropped completely.
			return slog.GroupAttrs(
				"",
				slog.String(s.SourceFile, source.File),
				slog.Int(s.SourceLine, source.Line),
				slog.String(s.SourceFunction, source.Function),
			)
		}

		grp, file, _ := strings.Cut(s.SourceFile, logging.AttrGroupDelimiter)
		_, line, _ := strings.Cut(s.SourceLine, logging.AttrGroupDelimiter)
		_, fn, _ := strings.Cut(s.SourceFunction, logging.AttrGroupDelimiter)
		return slog.GroupAttrs(
			grp,
			slog.String(file, source.File),
			slog.Int(line, source.Line),
			slog.String(fn, source.Function),
		)
	}

	return a
}
