// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package logging

import (
	"context"
	"log/slog"
	"slices"
)

var _ slog.Handler = (*MockHandler)(nil)

type MockHandler struct {
	Attrs   []slog.Attr
	Groups  []string
	Store   bool
	Records [][]slog.Attr
}

func (h *MockHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *MockHandler) Handle(_ context.Context, or slog.Record) error {
	if !h.Store {
		return nil
	}

	attrs := slices.Clone(h.Attrs)
	or.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	for i := range h.Groups {
		attrs = []slog.Attr{
			slog.GroupAttrs(h.Groups[len(attrs)-1-i], attrs...),
		}
	}
	h.Records = append(h.Records, attrs)
	return nil
}

func (h *MockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &MockHandler{
		Attrs:  appendAttrsToGroup(h.Groups, h.Attrs, attrs...),
		Groups: h.Groups,
	}
}

func (h *MockHandler) WithGroup(name string) slog.Handler {
	// https://cs.opensource.google/go/x/exp/+/46b07846:slog/handler.go;l=247
	if name == "" {
		return h
	}
	return &MockHandler{
		Attrs:  h.Attrs,
		Groups: append(h.Groups, name),
	}
}

func appendAttrsToGroup(groups []string, actualAttrs []slog.Attr, newAttrs ...slog.Attr) []slog.Attr {
	actualAttrs = slices.Clone(actualAttrs)

	if len(groups) == 0 {
		return append(actualAttrs, newAttrs...)
	}

	for i := range actualAttrs {
		attr := actualAttrs[i]
		if attr.Key == groups[0] && attr.Value.Kind() == slog.KindGroup {
			actualAttrs[i] = slog.GroupAttrs(groups[0], appendAttrsToGroup(groups[1:], attr.Value.Group(), newAttrs...)...)
			return actualAttrs
		}
	}

	return append(
		actualAttrs,
		slog.GroupAttrs(
			groups[0],
			appendAttrsToGroup(groups[1:], []slog.Attr{}, newAttrs...)...,
		),
	)
}
