// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package logging

import (
	"log/slog"
	"reflect"
	"testing"
)

func TestGroupAttrsRecursive(t *testing.T) {
	tests := []struct {
		name     string
		input    []slog.Attr
		expected []slog.Attr
	}{
		{
			name:     "empty-input",
			input:    []slog.Attr{},
			expected: []slog.Attr{},
		},
		{
			name:     "nil-input",
			input:    nil,
			expected: nil,
		},
		{
			name: "no-delimiters-single-level",
			input: []slog.Attr{
				slog.String("foo", "bar"),
				slog.Int("baz", 42),
			},
			expected: []slog.Attr{
				slog.String("foo", "bar"),
				slog.Int("baz", 42),
			},
		},
		{
			name: "single-delimiter-two-levels",
			input: []slog.Attr{
				slog.String("foo>bar", "baz"),
				slog.Int("foo>qux", 42),
				slog.String("abc", "def"),
			},
			expected: []slog.Attr{
				slog.String("abc", "def"),
				slog.GroupAttrs("foo",
					slog.String("bar", "baz"),
					slog.Int("qux", 42),
				),
			},
		},
		{
			name: "multiple-delimiters-three-levels",
			input: []slog.Attr{
				slog.String("foo>bar>baz", "value1"),
				slog.String("foo>bar>qux", "value2"),
				slog.String("foo>baz>qux", "value3"),
				slog.String("abc", "def"),
			},
			expected: []slog.Attr{
				slog.String("abc", "def"),
				slog.GroupAttrs("foo",
					slog.GroupAttrs("bar",
						slog.String("baz", "value1"),
						slog.String("qux", "value2"),
					),
					slog.GroupAttrs("baz",
						slog.String("qux", "value3"),
					),
				),
			},
		},
		{
			name: "deep-nesting-four-levels",
			input: []slog.Attr{
				slog.String("a>b>c>d", "value1"),
				slog.String("a>b>c>e", "value2"),
				slog.String("a>b>f>g", "value3"),
				slog.String("a>h>i", "value4"),
				slog.String("j", "value5"),
			},
			expected: []slog.Attr{
				slog.String("j", "value5"),
				slog.GroupAttrs("a",
					slog.GroupAttrs("b",
						slog.GroupAttrs("c",
							slog.String("d", "value1"),
							slog.String("e", "value2"),
						),
						slog.GroupAttrs("f",
							slog.String("g", "value3"),
						),
					),
					slog.GroupAttrs("h",
						slog.String("i", "value4"),
					),
				),
			},
		},
		{
			name: "mixed-types",
			input: []slog.Attr{
				slog.String("user>name", "john"),
				slog.Int("user>age", 30),
				slog.Bool("user>active", true),
				slog.Float64("user>score", 95.5),
				slog.String("status", "ok"),
			},
			expected: []slog.Attr{
				slog.String("status", "ok"),
				slog.GroupAttrs("user",
					slog.String("name", "john"),
					slog.Int("age", 30),
					slog.Bool("active", true),
					slog.Float64("score", 95.5),
				),
			},
		},
		{
			name: "empty-key-attributes-are-skipped",
			input: []slog.Attr{
				slog.String("", "empty"),
				slog.String("foo>bar", "baz"),
				slog.String("", "another empty"),
			},
			expected: []slog.Attr{
				slog.GroupAttrs("foo",
					slog.String("bar", "baz"),
				),
			},
		},
		{
			name: "single-attribute-with-delimiter",
			input: []slog.Attr{
				slog.String("single>level", "value"),
			},
			expected: []slog.Attr{
				slog.GroupAttrs("single",
					slog.String("level", "value"),
				),
			},
		},
		{
			name: "complex-nested-structure",
			input: []slog.Attr{
				slog.String("request>headers>content-type", "application/json"),
				slog.String("request>headers>authorization", "Bearer token"),
				slog.String("request>body>data>name", "test"),
				slog.String("request>body>data>value", "123"),
				slog.String("response>status", "200"),
				slog.String("response>headers>content-type", "application/json"),
				slog.String("metadata>timestamp", "2023-01-01T00:00:00Z"),
			},
			expected: []slog.Attr{
				slog.GroupAttrs("metadata",
					slog.String("timestamp", "2023-01-01T00:00:00Z"),
				),
				slog.GroupAttrs("request",
					slog.GroupAttrs("body",
						slog.GroupAttrs("data",
							slog.String("name", "test"),
							slog.String("value", "123"),
						),
					),
					slog.GroupAttrs("headers",
						slog.String("content-type", "application/json"),
						slog.String("authorization", "Bearer token"),
					),
				),
				slog.GroupAttrs("response",
					slog.String("status", "200"),
					slog.GroupAttrs("headers",
						slog.String("content-type", "application/json"),
					),
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GroupAttrsRecursive(tt.input)
			if !compareAttrSlices(result, tt.expected) {
				t.Errorf("GroupAttrsRecursive() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// compareAttrSlices compares two slices of slog.Attr for equality.
func compareAttrSlices(a, b []slog.Attr) bool {
	if len(a) != len(b) {
		return false
	}
	mapA := make(map[string]slog.Attr)
	mapB := make(map[string]slog.Attr)
	for _, attr := range a {
		mapA[attr.Key] = attr
	}
	for _, attr := range b {
		mapB[attr.Key] = attr
	}
	for key, attrA := range mapA {
		attrB, exists := mapB[key]
		if !exists || !compareAttrs(attrA, attrB) {
			return false
		}
	}
	return true
}

// compareAttrs compares two slog.Attr for equality.
func compareAttrs(a, b slog.Attr) bool {
	if a.Key != b.Key {
		return false
	}
	return reflect.DeepEqual(a.Value, b.Value)
}

func BenchmarkGroupAttrsRecursive(b *testing.B) {
	testCases := []struct {
		name  string
		attrs []slog.Attr
	}{
		{
			name: "shallow",
			attrs: []slog.Attr{
				slog.String("a>b", "1"),
				slog.String("a>c", "2"),
				slog.String("d>e", "3"),
				slog.String("f", "4"),
			},
		},
		{
			name: "deep",
			attrs: []slog.Attr{
				slog.String("a>b>c>d>e", "1"),
				slog.String("a>b>c>d>f", "2"),
				slog.String("a>b>g>h", "3"),
				slog.String("i>j>k", "4"),
				slog.String("l", "5"),
			},
		},
		{
			name: "many_attributes",
			attrs: func() []slog.Attr {
				attrs := make([]slog.Attr, 100)
				for i := range 100 {
					attrs[i] = slog.String("group>subgroup>attr", "value")
				}
				return attrs
			}(),
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				GroupAttrsRecursive(tc.attrs)
			}
		})
	}
}
