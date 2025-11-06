// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package text

import "strings"

func Map[T ~string](s []T, fns ...func(T) T) []T {
	if len(fns) == 0 {
		return s
	}

	for _, fn := range fns {
		for i, v := range s {
			s[i] = fn(v)
		}
	}
	return s
}

func SplitM(s []string, sep string) []string {
	out := make([]string, 0, len(s))
	for i := range s {
		if strings.Contains(s[i], sep) {
			out = append(out, strings.Split(s[i], sep)...)
		} else {
			out = append(out, s[i])
		}
	}
	return out
}
