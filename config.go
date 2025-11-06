// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/lrstanley/chix/v2/pkg/logging"
)

type contextKeyConfig struct{}

// Config is the configuration for the chix package.
type Config struct {
	apiBasePath string

	maskPrivateErrors bool
	errorResolvers    []ErrorResolverFn
	errorHandler      ErrorHandler

	requestDecoder   RequestDecoder
	requestValidator RequestValidator
	jsonDecoder      JSONDecoder
	jsonEncoder      JSONEncoder

	requestIDHeader string

	logger *slog.Logger
}

// NewConfig creates a new [Config] with the default values.
func NewConfig() *Config {
	return &Config{
		apiBasePath: "/",

		maskPrivateErrors: true,
		errorResolvers:    nil,
		errorHandler:      DefaultErrorHandler,

		requestDecoder:   DefaultRequestDecoder(),
		requestValidator: DefaultRequestValidator(),
		jsonDecoder:      DefaultJSONDecoder(),
		jsonEncoder:      DefaultJSONEncoder(),

		requestIDHeader: "X-Request-Id",

		logger: slog.New(&logging.Discard{}),
	}
}

// Clone creates a new [Config] with the same values as the current [Config].
func (c *Config) Clone() *Config {
	nc := &Config{
		apiBasePath: c.apiBasePath,

		maskPrivateErrors: c.maskPrivateErrors,
		errorResolvers:    c.errorResolvers,
		errorHandler:      c.errorHandler,

		requestDecoder:   c.requestDecoder,
		requestValidator: c.requestValidator,
		jsonDecoder:      c.jsonDecoder,
		jsonEncoder:      c.jsonEncoder,

		requestIDHeader: c.requestIDHeader,

		logger: c.logger,
	}
	return nc
}

// Use returns a middleware that sets the [Config] in the context.
func (c *Config) Use() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKeyConfig{}, c)))
		})
	}
}

// GetAPIBasePath returns the configured API base path.
func (c *Config) GetAPIBasePath() string {
	return c.apiBasePath
}

// SetAPIBasePath sets the API base path. Defaults to "/".
func (c *Config) SetAPIBasePath(path string) *Config {
	nc := c.Clone()
	nc.apiBasePath = path
	return nc
}

// GetMaskPrivateErrors returns the configured flag for masking private errors.
func (c *Config) GetMaskPrivateErrors() bool {
	return c.maskPrivateErrors
}

// SetMaskPrivateErrors sets the flag for masking private errors. If enabled, errors
// that are not public (like database errors) will be masked and a generic error
// message will be returned. This only relates to 5xx errors, not 4xx errors.
// Defaults to true.
func (c *Config) SetMaskPrivateErrors(enabled bool) *Config {
	nc := c.Clone()
	nc.maskPrivateErrors = enabled
	return nc
}

// GetErrorResolvers returns the configured error resolvers.
func (c *Config) GetErrorResolvers() []ErrorResolverFn {
	return c.errorResolvers
}

// SetErrorResolvers sets the error resolvers.
func (c *Config) SetErrorResolvers(resolvers ...ErrorResolverFn) *Config {
	nc := c.Clone()
	nc.errorResolvers = resolvers
	return nc
}

// AddErrorResolvers adds additional error resolvers to the existing ones.
func (c *Config) AddErrorResolvers(resolvers ...ErrorResolverFn) *Config {
	nc := c.Clone()
	nc.errorResolvers = append(nc.errorResolvers, resolvers...)
	return nc
}

// GetErrorHandler returns the configured error handler.
func (c *Config) GetErrorHandler() ErrorHandler {
	return c.errorHandler
}

// SetErrorHandler sets the error handler. Defaults to [DefaultErrorHandler].
func (c *Config) SetErrorHandler(handler ErrorHandler) *Config {
	if handler == nil {
		return c
	}
	nc := c.Clone()
	nc.errorHandler = handler
	return nc
}

// GetRequestDecoder returns the configured request decoder.
func (c *Config) GetRequestDecoder() RequestDecoder {
	return c.requestDecoder
}

// SetRequestDecoder sets the request/body decoder. Defaults to go-playground/form's
// form.NewDecoder(), in addition to JSON and similar content-types.
func (c *Config) SetRequestDecoder(decoder RequestDecoder) *Config {
	nc := c.Clone()
	nc.requestDecoder = decoder
	return nc
}

// GetRequestValidator returns the configured request validator.
func (c *Config) GetRequestValidator() RequestValidator {
	return c.requestValidator
}

// SetRequestValidator sets the global request validator. Defaults to go-playground/validator's
// validator.New().
func (c *Config) SetRequestValidator(validator RequestValidator) *Config {
	nc := c.Clone()
	nc.requestValidator = validator
	return nc
}

// GetJSONDecoder returns the configured JSON decoder.
func (c *Config) GetJSONDecoder() JSONDecoder {
	return c.jsonDecoder
}

// SetJSONDecoder sets the JSON decoder. Defaults to encoding/json or encoding/json/v2,
// if the experimental encoding/json/v2 is enabled for the build.
func (c *Config) SetJSONDecoder(decoder JSONDecoder) *Config {
	if decoder == nil {
		return c
	}
	nc := c.Clone()
	nc.jsonDecoder = decoder
	return nc
}

// GetJSONEncoder returns the configured JSON decoder.
func (c *Config) GetJSONEncoder() JSONEncoder {
	return c.jsonEncoder
}

// SetJSONEncoder sets the JSON encoder. Defaults to encoding/json or encoding/json/v2,
// if the experimental encoding/json/v2 is enabled for the build.
func (c *Config) SetJSONEncoder(encoder JSONEncoder) *Config {
	if encoder == nil {
		return c
	}
	nc := c.Clone()
	nc.jsonEncoder = encoder
	return nc
}

// GetRequestIDHeader returns the configured request ID header.
func (c *Config) GetRequestIDHeader() string {
	return c.requestIDHeader
}

// SetRequestIDHeader sets the request ID header. Defaults to "X-Request-Id".
func (c *Config) SetRequestIDHeader(header string) *Config {
	if header == "" {
		return c
	}
	nc := c.Clone()
	nc.requestIDHeader = header
	return nc
}

// GetLogger returns the configured logger.
func (c *Config) GetLogger() *slog.Logger {
	return c.logger
}

// SetLogger sets the logger. Defaults to a [discardLogger], which discards all log
// records.
func (c *Config) SetLogger(logger *slog.Logger) *Config {
	nc := c.Clone()
	nc.logger = logger
	return nc
}

// GetConfig returns the [Config] from the context, or creates a new [Config] if
// none is found.
func GetConfig(ctx context.Context) *Config {
	c, ok := ctx.Value(contextKeyConfig{}).(*Config)
	if !ok {
		return NewConfig()
	}
	return c
}
