// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-playground/form/v4"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

// RequestDecoder is a function that decodes request bodies, url params, etc to the
// given struct.
type RequestDecoder func(r *http.Request, v any) error

// DefaultRequestDecoder returns the default form decoder.
func DefaultRequestDecoder() RequestDecoder {
	dec := form.NewDecoder()

	return func(r *http.Request, v any) error {
		var err error

		if r.Body != nil {
			defer r.Body.Close()
		}

		jsonDecoder := GetConfig(r.Context()).GetJSONDecoder()

		err = dec.Decode(v, r.Form)

		if err == nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
			switch {
			case strings.HasPrefix(r.Header.Get("Content-Type"), "application/json"):
				err = jsonDecoder(r, v)
			case strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data"):
				err = r.ParseMultipartForm(4 << 20) // 4MB
				if err == nil {
					err = dec.Decode(v, r.MultipartForm.Value)
				}
			default:
				err = dec.Decode(v, r.PostForm)
			}
		}

		if err != nil {
			var invalidDecoderError *form.InvalidDecoderError
			if errors.As(err, &invalidDecoderError) {
				return &ResolvedError{
					Err:        err,
					StatusCode: http.StatusInternalServerError,
				}
			}
			return &ResolvedError{
				Err:        err,
				StatusCode: http.StatusBadRequest,
				Public:     true,
			}
		}

		return nil
	}
}

// RequestValidator is a function that validates a struct.
type RequestValidator func(r *http.Request, v any) error

type translationWrappedError struct {
	err        validator.FieldError
	translated string
}

func (e *translationWrappedError) Error() string {
	return e.translated
}

func (e *translationWrappedError) Unwrap() error {
	return e.err
}

// DefaultRequestValidator returns the default validator. It supports both
// [Validatable] implemented structs, in addition to go-playground/validator
// struct tags.
func DefaultRequestValidator() RequestValidator {
	structValidator := validator.New()
	uni := ut.New(en.New())

	return func(r *http.Request, v any) error {
		if v, ok := v.(Validatable); ok {
			if err := v.Validate(); err != nil {
				if _, rok := IsResolvedError(err); rok {
					return err
				}
				return &ResolvedError{
					Err:        err,
					StatusCode: http.StatusBadRequest,
					Public:     true,
				}
			}
			return nil
		}

		err := structValidator.StructCtx(r.Context(), v)
		if err != nil {
			var invalidValidationError *validator.InvalidValidationError
			if errors.As(err, &invalidValidationError) {
				return &ResolvedError{
					Err:        err,
					StatusCode: http.StatusInternalServerError,
					Public:     false,
				}
			}

			var validationErrors *validator.ValidationErrors
			if errors.As(err, &validationErrors) {
				var errs []error
				for _, err := range *validationErrors {
					errs = append(errs, &translationWrappedError{
						err:        err,
						translated: err.Translate(uni.GetFallback()),
					})
				}
				return &ResolvedError{
					Errs:       errs,
					StatusCode: http.StatusBadRequest,
					Public:     true,
				}
			}

			return &ResolvedError{
				Err:        err,
				StatusCode: http.StatusBadRequest,
				Public:     true,
			}
		}

		return nil
	}
}

// Validatable is an interface that can be implemented by structs to
// provide custom validation logic, on top of the default go-playground/form
// validation.
type Validatable interface {
	Validate() error
}

// Bind binds various request attributes to the given struct, including query
// parameters, form data (multipart/form-data included), JSON bodies, etc, in
// addition to validating the struct. It calls both [Config.GetRequestDecoder] and
// [Config.GetRequestValidator] to perform the necessary operations. Behind the
// scenes, both go-playground/form and go-playground/validator are used to perform
// the necessary operations.
//
// Example:
//
//	type User struct {
//		Name	string	`json:"name" validate:"required"`
//		Email	string	`json:"email" validate:"required,email"`
//		Pretty	bool	`form:"pretty"`
//	}
//
//	func (u *User) Validate() error {
//		if u.Name == "system" {
//			return errors.New("system users are not allowed")
//		}
//		return nil
//	}
//
//	func main() {
//		// [...]
//		r.Post("/user", func(w http.ResponseWriter, r *http.Request) {
//			var user User
//			if err := chix.Bind(r, &user); err != nil {
//				chix.Error(w, r, err)
//				return
//			}
//			// [... more request-specific logic...]
//			w.WriteHeader(http.StatusOK)
//		})
//	}
//
// References:
//   - https://github.com/go-playground/validator#fields
//   - https://github.com/go-playground/form#examples
func Bind(r *http.Request, v any) (err error) {
	err = r.ParseForm()
	if err != nil {
		return &ResolvedError{
			Err:        err,
			StatusCode: http.StatusBadRequest,
			Public:     true,
		}
	}

	cfg := GetConfig(r.Context())

	if dec := cfg.GetRequestDecoder(); dec != nil {
		err = dec(r, v)
		if err != nil {
			return err
		}
	}

	if val := cfg.GetRequestValidator(); val != nil {
		err = val(r, v)
		if err != nil {
			return err
		}
	}

	return nil
}
