// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package logging

import (
	"bytes"
	"io"
)

var (
	trunc     = []byte("[...truncated]")
	truncSize = len(trunc)
)

// LimitedBuffer is a buffer that limits the amount of bytes writeable to it. It
// is designed to be used for logging purposes, so the exact bytes may be exceeded,
// as a truncated message will be appended after the limit is reached. It will
// silently drop any additional bytes once the limit is reached. If the max bytes
// is <= 0, no limit will be applied, and the buffer will behave normally.
type LimitedBuffer struct {
	*bytes.Buffer
	maxSize   int
	remaining int
}

// NewLimitedBuffer creates a new [LimitedBuffer] with the given maximum size.
func NewLimitedBuffer(maxSize int) *LimitedBuffer {
	return &LimitedBuffer{
		Buffer:    &bytes.Buffer{},
		maxSize:   maxSize,
		remaining: maxSize,
	}
}

func (b *LimitedBuffer) Reset() {
	b.Buffer.Reset()
	b.remaining = b.maxSize
	if b.maxSize <= 0 {
		return
	}
}

// Write writes the given bytes to the buffer, until the limit is reached, and
// a truncated message is appended. From there, it will silently drop any additional
// bytes.
func (b *LimitedBuffer) Write(p []byte) (int, error) {
	if b.maxSize <= 0 {
		return b.Buffer.Write(p)
	}

	pl := len(p)

	if b.remaining <= 0 {
		return pl, nil
	}

	if b.remaining < pl {
		_, _ = b.Buffer.Write(p[:max(0, b.remaining-truncSize)])
		_, _ = b.Buffer.Write(trunc)
		b.remaining = 0
		return pl + truncSize, io.EOF
	}

	n, _ := b.Buffer.Write(p)
	b.remaining -= n
	return n, nil
}
