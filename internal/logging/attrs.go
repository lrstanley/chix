// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package logging

import (
	"log/slog"
	"net/http"
	"sort"
	"strings"
)

const AttrGroupDelimiter = ">"

// GroupAttrsRecursive takes a list of attributes, and for each attribute that has
// a key that contains the group delimiter, it will be split into a nested group,
// replacing all of the original attributes with the same nested group.
//
// Example:
//   - keys: ["foo>bar>baz", "foo>bar>qux", "foo>baz>qux", "abc"]
//   - effective result: [{"foo": {"bar": {"baz": "value", "qux": "value"}, "baz": {"qux": "value"}}, "abc": "value"}]
func GroupAttrsRecursive(attrs []slog.Attr) []slog.Attr {
	if len(attrs) == 0 {
		return attrs
	}

	// Use a map to efficiently group attributes by their root key.
	groups := make(map[string][]slog.Attr)
	var ungrouped []slog.Attr

	// First pass: separate grouped and ungrouped attributes.
	for _, attr := range attrs {
		if attr.Key == "" {
			continue
		}

		if strings.Contains(attr.Key, AttrGroupDelimiter) {
			rootKey, _, _ := strings.Cut(attr.Key, AttrGroupDelimiter)
			groups[rootKey] = append(groups[rootKey], attr)
		} else {
			ungrouped = append(ungrouped, attr)
		}
	}

	// If no groups found, return original attributes.
	if len(groups) == 0 {
		return ungrouped
	}

	// Second pass: recursively process each group.
	result := make([]slog.Attr, 0, len(ungrouped)+len(groups))

	// Add ungrouped attributes first
	result = append(result, ungrouped...)

	// Process each group recursively in sorted order for consistent output
	// Get sorted keys for consistent ordering
	keys := make([]string, 0, len(groups))
	for rootKey := range groups {
		keys = append(keys, rootKey)
	}
	sort.Strings(keys)

	for _, rootKey := range keys {
		groupAttrs := groups[rootKey]
		// Strip the root key prefix from all attributes in this group
		strippedAttrs := make([]slog.Attr, 0, len(groupAttrs))
		for _, attr := range groupAttrs {
			_, remainingKey, _ := strings.Cut(attr.Key, AttrGroupDelimiter)
			strippedAttrs = append(strippedAttrs, slog.Attr{
				Key:   remainingKey,
				Value: attr.Value,
			})
		}

		// Recursively group the stripped attributes
		recursivelyGrouped := GroupAttrsRecursive(strippedAttrs)

		// Create the group value
		result = append(result, slog.Any(rootKey, slog.GroupValue(recursivelyGrouped...)))
	}

	return result
}

func GetHeaderAttrs(header http.Header, headers []string) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(headers))
	for _, h := range headers {
		vals := header.Values(h)
		switch len(vals) {
		case 0:
			// no value, skip
		case 1:
			attrs = append(attrs, slog.String(h, vals[0]))
		default:
			attrs = append(attrs, slog.Any(h, vals))
		}
	}
	return attrs
}
