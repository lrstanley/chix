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
)

var DefaultDecoder = form.NewDecoder()

// Validatable is an interface that can be implemented by structs to
// provide custom validation logic, on top of the default go-playground/form
// validation.
type Validatable interface {
	Validate() error
}

// Bind decodes the request body to the given struct. Take a look at
// DefaultDecoder to add additional customizations to the default decoder.
// You can add additional customizations by using the Validatable interface,
// with a custom implementation of the Validate() method on v.
//
// At this time the only supported content-types are application/json,
// application/x-www-form-urlencoded, as well as GET parameters.
//
// If validation fails, Bind will respond with an HTTP 400 Bad Request, using
// Error().
func Bind(w http.ResponseWriter, r *http.Request, v interface{}) (ok bool) {
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
		_ = Error(w, r, http.StatusBadRequest, fmt.Errorf("unsupported method %s", r.Method))
		return false
	}
	if err != nil {
		rerr = fmt.Errorf("error decoding %s request into required format (%T): validate request parameters", r.Method, v)
	}

handle:
	if err != nil {
		_ = Error(w, r, http.StatusBadRequest, rerr)
		return false
	}

	if v, ok := v.(Validatable); ok {
		if err = v.Validate(); err != nil {
			_ = Error(w, r, http.StatusBadRequest, err)
			return false
		}
	}
	return true
}
