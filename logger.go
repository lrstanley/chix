// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/lrstanley/chix/v2/pkg/logging"
	"github.com/lrstanley/chix/v2/pkg/pool"
)

var sourceIgnoreContains = [...]string{
	"/go-chi/",
	"/lrstanley/chix/",
}

type contextKeyLogAttrs struct{}

// AppendLogAttrs appends the given attributes to the log entry in the context. Use
// this to annotate logs when using [UseStructuredLogger], and when using [Log],
// [LogDebug], [LogInfo], [LogWarn], and [LogError].
func AppendLogAttrs(ctx context.Context, attrs ...slog.Attr) {
	if v, ok := ctx.Value(contextKeyLogAttrs{}).(*[]slog.Attr); ok && v != nil {
		*v = append(*v, attrs...)
	}
}

// GetLogAttrs returns the attributes that have been added to the log entry in the
// context.
func GetLogAttrs(ctx context.Context) []slog.Attr {
	if v, ok := ctx.Value(contextKeyLogAttrs{}).(*[]slog.Attr); ok && v != nil {
		return *v
	}
	return nil
}

// SetLogError sets the error that occurred in the request/response in the context.
// Not required when using [Error] and similar functions, as they will automatically
// set the error in the context for you.
func SetLogError(ctx context.Context, rerr *ResolvedError) {
	AppendLogAttrs(ctx, rerr.LogAttrs()...)
}

// Log logs a message at the given level, with the given attributes. This includes
// any attributes which have been added by [AppendLogAttrs] or [SetLogError].
//
// TODO: make this always include the request ID.
func Log(ctx context.Context, lvl slog.Level, msg string, attrs ...slog.Attr) {
	GetConfig(ctx).GetLogger().LogAttrs(ctx, lvl, msg, append(GetLogAttrs(ctx), attrs...)...) //nolint:sloglint
}

// LogDebug logs a message at the debug level, with the given attributes. This includes
// any attributes which have been added by [AppendLogAttrs] or [SetLogError].
func LogDebug(ctx context.Context, msg string, attrs ...slog.Attr) {
	Log(ctx, slog.LevelDebug, msg, attrs...)
}

// LogInfo logs a message at the info level, with the given attributes. This includes
// any attributes which have been added by [AppendLogAttrs] or [SetLogError].
func LogInfo(ctx context.Context, msg string, attrs ...slog.Attr) {
	Log(ctx, slog.LevelInfo, msg, attrs...)
}

// LogWarn logs a message at the warn level, with the given attributes. This includes
// any attributes which have been added by [AppendLogAttrs] or [SetLogError].
func LogWarn(ctx context.Context, msg string, attrs ...slog.Attr) {
	Log(ctx, slog.LevelWarn, msg, attrs...)
}

// LogError logs a message at the error level, with the given attributes. This includes
// any attributes which have been added by [AppendLogAttrs] or [SetLogError].
func LogError(ctx context.Context, msg string, attrs ...slog.Attr) {
	Log(ctx, slog.LevelError, msg, attrs...)
}

// DefaultLogConfig returns a new [LogConfig] with the recommended default values.
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		Schema:        MustGetLogSchema(LogSchemaSimple),
		RecoverPanics: true,
		RequestQuery:  []string{"*"},
		RequestHeaders: []string{
			"Content-Type",
			"Origin",
			"Cf-Ray",
			"Cf-Ipcountry",
			"X-Request-ID",
		},
		ResponseHeaders: []string{"Content-Type"},
		BodyContentTypes: []string{
			"application/json",
			"application/xml",
			"text/plain",
			"text/csv",
			"application/x-www-form-urlencoded",
			"",
		},
		MaxBodySize: 1024,
	}
}

type LogConfig struct {
	// Level defines the minimum level to be logged. If not provided, the default
	// is defined by the level of the [log/slog.Logger] provided through [Config.Logger].
	// You can use this to have lower levels for most logging in your application,
	// but only log warnings/errors/etc. See [DefaultLogRequestLeveler] for the default
	// level determination, which can be customized.
	Level *slog.Level

	// Leveler can be used to customize the log level determination for a given request
	// and response status code. See [DefaultLogRequestLeveler] for the default level
	// determination, which can be customized.
	Leveler func(req *http.Request, respStatus int) slog.Level

	// Schema defines the mapping of semantic log fields to any custom format, as
	// needed by other logging systems.
	Schema *LogSchema

	// RecoverPanics recovers from panics occurring in the underlying HTTP handlers
	// and middleware and returns HTTP 500 (unless status was already sent). Note
	// that panics are logged as errors automatically, regardless of this setting.
	RecoverPanics bool

	// SkipPre can be used to skip logging for a given request. If not provided,
	// and not skipped by [LogConfig.SkipPost], all requests are recorded
	// (accounting for the provided [LogConfig.Level]). This is a more performant
	// alternative to [LogConfig.SkipPost], as it doesn't initialize any
	// wrapping of the response writer.
	SkipPre func(req *http.Request) bool

	// SkipPost can be used to skip logging for a given request, after the response
	// has been sent. If not provided, all requests are recorded (accounting for
	// the provided [LogConfig.Level]). If provided, true will cause the request to
	// not be logged. Use [LogConfig.SkipPost] If you don't need to account for the
	// response status code, as it is much more performant.
	SkipPost func(req *http.Request, respStatus int) bool

	// RequestQuery is a list of query parameters to be logged as attributes. Note
	// that if you use query parameters for sensitive data (don't, please), you
	// will want to customize this. If not provided, all query parameters will be
	// logged. Set to nil or [] to not log any query parameters.
	RequestQuery []string

	// RequestHeaders is a list of headers to be logged as attributes. If not
	// provided, the default is ["Content-Type", "Origin"]. Note that this can
	// cause sensitive information to be leaked if not used carefully. Set to "*"
	// to log all headers.
	RequestHeaders []string

	// ResponseHeaders is a list of headers to be logged as attributes. Note that this
	// can cause sensitive information to be leaked if not used carefully. Set to "*"
	// to log all headers.
	ResponseHeaders []string

	// RequestBody can be used to control logging of the request body. If not
	// provided, bodies will not be logged. Note that this can cause sensitive
	// information to be leaked if not used carefully.
	RequestBody func(req *http.Request) bool

	// ResponseBody can be used to control logging of the response body. If not
	// provided, bodies will not be logged. Note that this can cause sensitive
	// information to be leaked if not used carefully.
	ResponseBody func(req *http.Request) bool

	// BodyContentTypes is a list of body Content-Types that can safely be logged.
	// Default is "application/json", "application/xml", "text/plain", "text/csv",
	// "application/x-www-form-urlencoded" & "".
	BodyContentTypes []string

	// MaxBodySize defines the maximum length of the body to be logged, in bytes.
	// If not provided, defaults to 1KB. Set to -1 to log the entire body.
	MaxBodySize int

	// AppendAttrs can be used to add extra attributes to the log entry globally.
	// Note that this will be called after the request has been processed, and can
	// also log sensitive information if not used carefully.
	AppendAttrs func(req *http.Request, reqBody string, respStatus int) []slog.Attr
}

// Validate validates the log config, and sets the default values for any missing fields,
// which are required. Use this to validate the config before using it, otherwise
// [UseStructuredLogger] will panic if an invalid config is provided.
func (c *LogConfig) Validate() error {
	defaultConfig := DefaultLogConfig()

	if c == nil {
		c = defaultConfig
	}
	if len(c.BodyContentTypes) == 0 {
		c.BodyContentTypes = defaultConfig.BodyContentTypes
	}
	if c.MaxBodySize == 0 {
		c.MaxBodySize = defaultConfig.MaxBodySize
	}
	if c.Leveler == nil {
		c.Leveler = DefaultLogRequestLeveler
	}
	if c.Schema == nil {
		c.Schema = defaultConfig.Schema
	}
	if c.Schema.ResponseDurationFormat == nil {
		c.Schema.ResponseDurationFormat = func(key string, duration time.Duration) slog.Attr {
			return slog.Float64(key, float64(duration.Milliseconds()))
		}
	}

	// Disable some functions, if the schema fields are not set, to optimize perf.
	if c.Schema.RequestBody == "" && c.RequestBody != nil {
		c.RequestBody = nil
	}
	if c.Schema.ResponseBody == "" && c.ResponseBody != nil {
		c.ResponseBody = nil
	}
	if c.Schema.RequestHeaders == "" && len(c.RequestHeaders) > 0 {
		c.RequestHeaders = nil
	}
	if c.Schema.ResponseHeaders == "" && len(c.ResponseHeaders) > 0 {
		c.ResponseHeaders = nil
	}

	c.Schema.checkHasGroupDelimiter()
	return nil
}

// GetRequestScheme returns the scheme of a given HTTP request.
func (c *LogConfig) GetRequestScheme(r *http.Request) string {
	if r.TLS != nil || r.URL.Scheme == "https" { //nolint:goconst
		return "https"
	}
	return "http"
}

// GetRequestURL returns the full request URL, including the scheme, host, path, and query parameters,
// of a given HTTP request, accounting for items excluded in the logging config.
func (c *LogConfig) GetRequestURL(r *http.Request) string {
	return c.GetRequestScheme(r) +
		"://" +
		r.Host +
		r.URL.EscapedPath() +
		c.filterQueryParams(r.URL.Query()).Encode()
}

func (c *LogConfig) filterQueryParams(values url.Values) url.Values {
	if len(c.RequestQuery) == 0 || c.RequestQuery[0] == "*" {
		return values
	}
	for k := range values {
		if !slices.Contains(c.RequestQuery, k) {
			values.Del(k)
		}
	}
	return values
}

func (c *LogConfig) getRequestBody(body *logging.LimitedBuffer, header http.Header) string {
	if body.Len() == 0 {
		return ""
	}
	contentType := header.Get("Content-Type")
	for _, whitelisted := range c.BodyContentTypes {
		if strings.HasPrefix(contentType, whitelisted) {
			return body.String()
		}
	}
	return "[redacted due to Content-Type: " + contentType + "]"
}

// DefaultLogRequestLeveler determines the log level for a given request and response
// status code. The following request/response attributes define how the level is
// determined:
//
//   - statusCode >= 500: ERROR
//   - statusCode == 429: INFO
//   - statusCode >= 400: WARN
//   - method == OPTIONS: DEBUG
//   - default: INFO
func DefaultLogRequestLeveler(r *http.Request, statusCode int) slog.Level {
	switch {
	case statusCode >= 500:
		return slog.LevelError
	case statusCode == 429:
		return slog.LevelInfo
	case statusCode >= 400:
		return slog.LevelWarn
	case r.Method == http.MethodOptions:
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

type logEntry struct {
	reqBody  *logging.LimitedBuffer
	respBody *logging.LimitedBuffer
	attrs    []slog.Attr
}

func (e *logEntry) append(attrs ...slog.Attr) {
	for _, attr := range attrs {
		if attr.Key != "" {
			e.attrs = append(e.attrs, attr)
		}
	}
}

func (e *logEntry) Reset() {
	e.reqBody.Reset()
	e.respBody.Reset()
	e.attrs = e.attrs[0:0]
}

// UseStructuredLogger is a middleware that logs the request and response as a structured
// log entry. It uses the [LogConfig] to determine the log level, and what the schema
// of the log entry should be. See [DefaultLogConfig] for the default configuration,
// which can be customized.
//
// NOTE:
//   - This can recover from panics (and always will, to log them, regardless  the
//     [LogConfig.RecoverPanics] setting, but will re-throw if panic recovery is disabled),
//     and will always log the request/response in that scenario. This means that if
//     [LogConfig.RecoverPanics] is enabled, you do not need to use [UseRecoverer]
//     middleware after this one.
//   - This should be loaded after request-id type middleware, but before everything
//     else.
//   - [AppendLogAttrs] can be used to add extra attributes to the log entry.
//   - [GetLogAttrs] can be used to get the attributes that have been added to the log entry,
//     if you want to use them in other middleware or handlers.
//   - [SetLogError] can be used to set the error that occurred in the request/response,
//     though if using [Error] and similar functions, this is automatically done for you.
func UseStructuredLogger(config *LogConfig) func(http.Handler) http.Handler { //nolint:gocognit
	if err := config.Validate(); err != nil {
		panic(err)
	}
	hasGroupDelimiter := config.Schema.hasGroupDelimiter.Load()

	logEntryPool := pool.New(func() *logEntry {
		return &logEntry{
			reqBody:  logging.NewLimitedBuffer(config.MaxBodySize),
			respBody: logging.NewLimitedBuffer(config.MaxBodySize),
			attrs:    make([]slog.Attr, 0, 10),
		}
	})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if config.SkipPre != nil && config.SkipPre(r) {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), contextKeyLogAttrs{}, &[]slog.Attr{})
			logger := GetConfig(ctx).GetLogger()

			shouldLogRequestBody := config.RequestBody != nil && config.RequestBody(r)
			shouldLogResponseBody := config.ResponseBody != nil && config.ResponseBody(r)

			entry := logEntryPool.Get()
			defer logEntryPool.Put(entry)

			if shouldLogRequestBody || config.AppendAttrs != nil {
				r.Body = io.NopCloser(io.TeeReader(r.Body, entry.reqBody))
			}

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			if shouldLogResponseBody {
				ww.Tee(entry.respBody)
			}

			start := time.Now()

			defer func() {
				if rec := recover(); rec != nil {
					// Return HTTP 500 if recover is enabled and no response status was set.
					if config.RecoverPanics && ww.Status() == 0 && r.Header.Get("Connection") != "Upgrade" {
						ww.WriteHeader(http.StatusInternalServerError)
					}

					if rec == http.ErrAbortHandler || !config.RecoverPanics { //nolint:errorlint
						// Re-panic http.ErrAbortHandler unconditionally, and re-panic other errors if
						// panic recovery is disabled.
						defer panic(rec)
					}

					entry.append(slog.String(config.Schema.ErrorMessage, fmt.Sprintf("panic: %v", rec)))

					if rec != http.ErrAbortHandler { //nolint:errorlint
						pc := make([]uintptr, 10)   // Capture up to 10 stack frames.
						n := runtime.Callers(3, pc) // Skip 3 frames (Callers, this middleware, runtime/panic.go).
						pc = pc[:n]

						// Process panic stack frames to print detailed information.
						frames := runtime.CallersFrames(pc)
						var stackValues []string
						for frame, more := frames.Next(); more; frame, more = frames.Next() {
							if !strings.Contains(frame.File, "runtime/panic.go") {
								stackValues = append(stackValues, fmt.Sprintf("%s:%d", frame.File, frame.Line))
							}
						}
						entry.append(slog.Any(config.Schema.ErrorStackTrace, stackValues))
					}
				}

				duration := time.Since(start)
				statusCode := ww.Status()
				if statusCode == 0 {
					// net/http automatically sends HTTP 200 OK to the client when
					// a user doesn't explicitly set a status code.
					statusCode = 200
				}

				if config.SkipPost != nil && config.SkipPost(r, statusCode) {
					return
				}

				level := config.Leveler(r, statusCode)

				// Skip logging if the message level is below the logger's level,
				// or if the global config level is set and the message level is below it.
				if !logger.Enabled(ctx, level) || (config.Level != nil && level < *config.Level) {
					return
				}

				entry.append(
					slog.String(config.Schema.RequestURL, config.GetRequestURL(r)),
					slog.String(config.Schema.RequestMethod, r.Method),
					slog.String(config.Schema.RequestPath, r.URL.Path),
					slog.String(config.Schema.RequestRemoteIP, sanitizeIP(r.RemoteAddr)),
					slog.String(config.Schema.RequestHost, r.Host),
					slog.String(config.Schema.RequestScheme, config.GetRequestScheme(r)),
					slog.String(config.Schema.RequestProto, r.Proto),
					slog.Any(config.Schema.RequestHeaders, slog.GroupValue(logging.GetHeaderAttrs(r.Header, config.RequestHeaders)...)),
					slog.Int64(config.Schema.RequestBytes, r.ContentLength),
					slog.String(config.Schema.RequestUserAgent, r.UserAgent()),
					slog.String(config.Schema.RequestReferer, r.Referer()),
					slog.Any(config.Schema.ResponseHeaders, slog.GroupValue(logging.GetHeaderAttrs(ww.Header(), config.ResponseHeaders)...)),
					slog.Int(config.Schema.ResponseStatus, statusCode),
					config.Schema.ResponseDurationFormat(config.Schema.ResponseDuration, duration),
					slog.Int(config.Schema.ResponseBytes, ww.BytesWritten()),
				)

				if err := ctx.Err(); errors.Is(err, context.Canceled) {
					entry.append(
						slog.String("error", "request aborted: client disconnected before response was sent"),
						slog.String(config.Schema.ErrorType, "ClientAborted"),
					)
				}

				if id := GetRequestIDOrHeader(ctx, r); id != "" {
					entry.append(slog.String(config.Schema.RequestID, id))
				}

				if shouldLogRequestBody || config.AppendAttrs != nil {
					// Ensure the request body is fully read if the underlying
					// HTTP handler didn't do so.
					n, _ := io.Copy(io.Discard, r.Body)
					if n > 0 {
						entry.append(slog.Any(config.Schema.RequestBytesUnread, n))
					}
				}
				if shouldLogRequestBody {
					entry.append(slog.String(config.Schema.RequestBody, config.getRequestBody(entry.reqBody, r.Header)))
				}
				if shouldLogResponseBody {
					entry.append(slog.String(config.Schema.ResponseBody, config.getRequestBody(entry.respBody, ww.Header())))
				}
				if config.AppendAttrs != nil {
					entry.append(config.AppendAttrs(r, entry.reqBody.String(), statusCode)...)
				}
				entry.append(GetLogAttrs(ctx)...)

				// Group attributes into nested objects.
				if hasGroupDelimiter {
					entry.append(logging.GroupAttrsRecursive(entry.attrs)...)
				}

				logger.LogAttrs(
					ctx,
					level,
					r.Method+" "+r.URL.String()+" => "+strconv.Itoa(statusCode)+" ("+duration.String()+")",
					entry.attrs...,
				)
			}()

			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}
