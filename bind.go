// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/form/v4"
	"github.com/go-playground/validator/v10"
)

var (
	// DefaultDecoder is the default decoder used by Bind. You can either override
	// this, or provide your own. Make sure it is set before Bind is called.
	DefaultDecoder = form.NewDecoder()

	// DefaultDecodeMaxMemory is the maximum amount of memory in bytes that will be
	// used for decoding multipart/form-data requests.
	DefaultDecodeMaxMemory int64 = 8 << 20

	// DefaultValidator is the default validator used by Bind, when the provided
	// struct to the Bind() call doesn't implement Validatable. Set this to nil
	// to disable validation using go-playground/validator.
	DefaultValidator = validator.New()
)

// Validatable is an interface that can be implemented by structs to
// provide custom validation logic, on top of the default go-playground/form
// validation.
type Validatable interface {
	Validate() error
}

// Bind decodes the request body to the given struct. Take a look at
// DefaultDecoder to add additional customizations to the default decoder.
// You can add additional customizations by using the Validatable interface,
// with a custom implementation of the Validate() method on v. Alternatively,
// chix also supports the go-playground/validator package, which allows various
// validation methods via struct tags.
//
// At this time the only supported content-types are application/json,
// application/x-www-form-urlencoded, as well as GET parameters.
//
// If validation fails, an error that is wrapped with the necessary status code
// will be returned (can just pass to chix.Error() and it will know the appropriate
// HTTP code to return, and if it should be a JSON body or not).
func Bind(r *http.Request, v any) (err error) {
	var rerr error

	if err = r.ParseForm(); err != nil {
		rerr = fmt.Errorf("error parsing %s parameters, invalid request", r.Method)
		goto handle
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		err = DefaultDecoder.Decode(v, r.Form)
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		switch {
		case strings.HasPrefix(r.Header.Get("Content-Type"), "application/json"):
			dec := json.NewDecoder(r.Body)
			defer r.Body.Close()
			err = dec.Decode(v)
		case strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data"):
			err = r.ParseMultipartForm(DefaultDecodeMaxMemory)
			if err == nil {
				err = DefaultDecoder.Decode(v, r.MultipartForm.Value)
			}
		default:
			err = DefaultDecoder.Decode(v, r.PostForm)
		}
	default:
		return WrapError(fmt.Errorf("unsupported method %s", r.Method), http.StatusBadRequest)
	}
	if err != nil {
		rerr = fmt.Errorf("error decoding %s request into required format (%T): validate request parameters", r.Method, v)
	}

handle:
	if err != nil {
		return WrapError(rerr, http.StatusBadRequest)
	}

	if v, ok := v.(Validatable); ok {
		if err = v.Validate(); err != nil {
			return WrapError(err, http.StatusBadRequest)
		}

		return nil
	}

	if DefaultValidator != nil {
		err = DefaultValidator.Struct(v)
		if err != nil {
			if _, ok := err.(*validator.InvalidValidationError); ok {
				panic(fmt.Errorf("invalid validation error: %w", err))
			}

			// for _, err := range err.(validator.ValidationErrors) {}
			return WrapError(err, http.StatusBadRequest)
		}

		return nil
	}

	return nil
}
