// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package text

import (
	"strings"
	"testing"
)

func testGlobMatch(t *testing.T, subj, pattern string) {
	if !Glob(subj, pattern) {
		t.Fatalf("'%s' should match '%s'", pattern, subj)
	}
}

func testGlobNoMatch(t *testing.T, subj, pattern string) {
	if Glob(subj, pattern) {
		t.Fatalf("'%s' should not match '%s'", pattern, subj)
	}
}

func TestEmptyPattern(t *testing.T) {
	testGlobMatch(t, "", "")
	testGlobNoMatch(t, "test", "")
}

func TestEmptySubject(t *testing.T) {
	cases := []string{
		"",
		"*",
		"**",
		"***",
		"****************",
		strings.Repeat("*", 1000000),
	}

	for _, pattern := range cases {
		testGlobMatch(t, "", pattern)
	}

	cases = []string{
		// No globs/non-glob characters.
		"test",
		"*test*",

		// Trailing characters.
		"*x",
		"*****************x",
		strings.Repeat("*", 1000000) + "x",

		// Leading characters.
		"x*",
		"x*****************",
		"x" + strings.Repeat("*", 1000000),

		// Mixed leading/trailing characters.
		"x*x",
		"x****************x",
		"x" + strings.Repeat("*", 1000000) + "x",
	}

	for _, pattern := range cases {
		testGlobNoMatch(t, pattern, "")
	}
}

func TestPatternWithoutGlobs(t *testing.T) {
	testGlobMatch(t, "test", "test")
}

var testsGlob = []string{
	"*test",           // Leading.
	"this*",           // Trailing.
	"this*test",       // Middle.
	"*is *",           // String in between two.
	"*is*a*",          // Lots.
	"**test**",        // Double glob characters.
	"**is**a***test*", // Varying number.
	"* *",             // White space between.
	"*",               // Lone.
	"**********",      // Nothing but globs.
	"*Ѿ*",             // Unicode.
	"*is a ϗѾ *",      // Mixed ASCII/unicode.
}

func FuzzGlob(f *testing.F) {
	for _, tc := range testsGlob {
		f.Add(tc, tc)
	}

	f.Fuzz(func(_ *testing.T, orig, orig2 string) {
		_ = Glob(orig, orig2)
	})
}

func TestGlob(t *testing.T) {
	for _, pattern := range testsGlob {
		testGlobMatch(t, "this is a ϗѾ test", pattern)
	}

	cases := []string{
		"test*", // Implicit substring match.
		"*is",   // Partial match.
		"*no*",  // Globs without a match between them.
		" ",     // Plain white space.
		"* ",    // Trailing white space.
		" *",    // Leading white space.
		"*ʤ*",   // Non-matching unicode.
	}

	// Non-matches
	for _, pattern := range cases {
		testGlobNoMatch(t, "this is a test", pattern)
	}
}
