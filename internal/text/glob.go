// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package text

import "strings"

const GlobChar = "*"

// Glob will test a string pattern, potentially containing globs, against a
// string. The glob character is *.
func Glob(input, match string) bool {
	// Empty pattern.
	if match == "" {
		return input == match
	}

	// If a glob, match all.
	if match == GlobChar {
		return true
	}

	parts := strings.Split(match, GlobChar)

	if len(parts) == 1 {
		// No globs, test for equality.
		return input == match
	}

	leadingGlob, trailingGlob := strings.HasPrefix(match, GlobChar), strings.HasSuffix(match, GlobChar)
	last := len(parts) - 1

	// Check prefix first.
	if !leadingGlob && !strings.HasPrefix(input, parts[0]) {
		return false
	}

	// Check middle section.
	for i := 1; i < last; i++ {
		if !strings.Contains(input, parts[i]) {
			return false
		}

		// Trim already-evaluated text from input during loop over match
		// text.
		idx := strings.Index(input, parts[i]) + len(parts[i])
		input = input[idx:]
	}

	// Check suffix last.
	return trailingGlob || strings.HasSuffix(input, parts[last])
}
