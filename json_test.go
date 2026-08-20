// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultJSONDecoderOptions(t *testing.T) {
	t.Parallel()

	type payload struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name    string
		opts    []json.Options
		body    string
		wantErr bool
		want    string
	}{
		{
			name: "unknown member allowed by default",
			body: `{"name":"x","extra":1}`,
			want: "x",
		},
		{
			name:    "unknown member rejected",
			opts:    []json.Options{json.RejectUnknownMembers(true)},
			body:    `{"name":"x","extra":1}`,
			wantErr: true,
		},
		{
			name: "known members with reject option",
			opts: []json.Options{json.RejectUnknownMembers(true)},
			body: `{"name":"x"}`,
			want: "x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			req = requestWithConfig(NewConfig(), req)

			var v payload
			err := DefaultJSONDecoder(tt.opts...)(req, &v)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.Name != tt.want {
				t.Fatalf("name = %q, want %q", v.Name, tt.want)
			}
		})
	}
}

func TestDefaultJSONEncoderOptions(t *testing.T) {
	t.Parallel()

	type payload struct {
		Items []string `json:"items"`
	}

	tests := []struct {
		name   string
		opts   []json.Options
		pretty bool
		want   string
	}{
		{
			name: "nil slice as empty array by default",
			want: `{"items":[]}`,
		},
		{
			name: "nil slice as null with option",
			opts: []json.Options{json.FormatNilSliceAsNull(true)},
			want: `{"items":null}`,
		},
		{
			name:   "pretty indent",
			pretty: true,
			want: `{
	"items": []
}`,
		},
		{
			name:   "pretty indent keeps custom options",
			opts:   []json.Options{json.FormatNilSliceAsNull(true)},
			pretty: true,
			want: `{
	"items": null
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			url := "http://example.com/"
			if tt.pretty {
				url += "?pretty=true"
			}
			req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
			rec := httptest.NewRecorder()

			if err := DefaultJSONEncoder(tt.opts...)(rec, req, payload{}); err != nil {
				t.Fatalf("encode: %v", err)
			}
			if got := rec.Body.String(); got != tt.want {
				t.Fatalf("body = %q, want %q", got, tt.want)
			}
		})
	}
}
