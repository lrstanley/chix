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
// If validation fails, Bind will respond with an HTTP 400 Bad Request, using
// Error().
func Bind(w http.ResponseWriter, r *http.Request, v any) (ok bool) {
	var err, rerr error

	if err = r.ParseForm(); err != nil {
		rerr = fmt.Errorf("error parsing %s parameters, invalid request", r.Method)
		goto handle
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		err = DefaultDecoder.Decode(v, r.Form)
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			dec := json.NewDecoder(r.Body)
			defer r.Body.Close()
			err = dec.Decode(v)
		} else {
			err = DefaultDecoder.Decode(v, r.PostForm)
		}
	default:
		_ = Error(w, r, WrapError(fmt.Errorf("unsupported method %s", r.Method), http.StatusBadRequest))
		return false
	}
	if err != nil {
		rerr = fmt.Errorf("error decoding %s request into required format (%T): validate request parameters", r.Method, v)
	}

handle:
	if err != nil {
		_ = Error(w, r, WrapError(rerr, http.StatusBadRequest))
		return false
	}

	if v, ok := v.(Validatable); ok {
		if err = v.Validate(); err != nil {
			_ = Error(w, r, WrapError(err, http.StatusBadRequest))
			return false
		}

		return true
	}

	if DefaultValidator != nil {
		if err := DefaultValidator.Struct(v); err != nil {
			if _, ok := err.(*validator.InvalidValidationError); ok {
				panic(fmt.Errorf("invalid validation error: %w", err))
			}

			// for _, err := range err.(validator.ValidationErrors) {}
			_ = Error(w, r, WrapError(err, http.StatusBadRequest))
			return false
		}

		return true
	}

	return true
}
