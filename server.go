// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/lrstanley/x/sync/scheduler"
)

// Run runs the HTTP server (with sane/safe defaults set), including with graceful
// termination. When the context is cancelled, the server will be gracefully
// terminated (with a max timeout of 60 seconds), waiting for all requests to
// complete. You can pass additional scheduler jobs, which allows additional
// asynchronous tasks/cron-jobs to be run alongside the HTTP server.
//
// This will also listen for OS signals (SIGINT, SIGTERM, SIGQUIT) and gracefully
// terminate the server and all jobs when received.
func Run(
	pctx context.Context,
	logger *slog.Logger,
	srv *http.Server,
	jobs ...scheduler.Job,
) error {
	return scheduler.Run(pctx, append(
		[]scheduler.Job{scheduler.JobFunc(func(ctx context.Context) error {
			return NewServer(ctx, logger, srv, "", "")
		})},
		jobs...,
	)...)
}

// RunTLS runs the HTTP server (with sane/safe defaults set) with TLS support,
// including with graceful termination. When the context is cancelled, the server
// will be gracefully terminated (with a max timeout of 60 seconds), waiting for
// all requests to complete. You can pass additional scheduler jobs, which allows
// additional asynchronous tasks/cron-jobs to be run alongside the HTTP server.
//
// This will also listen for OS signals (SIGINT, SIGTERM, SIGQUIT) and gracefully
// terminate the server and all jobs when received.
func RunTLS(
	pctx context.Context,
	logger *slog.Logger,
	srv *http.Server,
	certFile, keyFile string,
	jobs ...scheduler.Job,
) error {
	return scheduler.Run(pctx, append(
		[]scheduler.Job{scheduler.JobFunc(func(ctx context.Context) error {
			return NewServer(ctx, logger, srv, certFile, keyFile)
		})},
		jobs...,
	)...)
}

// NewServerWithoutDefaults runs the HTTP server, including with graceful termination,
// so when the context is cancelled, the server will be gracefully terminated (with
// a max timeout of 60 seconds), waiting for all requests to complete. If certFile
// and keyFile are provided, the server will be run with TLS enabled.
func NewServerWithoutDefaults(
	ctx context.Context,
	logger *slog.Logger,
	srv *http.Server,
	certFile, keyFile string,
) error {
	if srv == nil {
		panic("srv is nil")
	}
	return withGracefulShutdown(ctx, logger, srv, certFile, keyFile)
}

// NewServer runs the HTTP server (with sane/safe defaults set), including with
// graceful termination, so when the context is cancelled, the server will be
// gracefully terminated (with a max timeout of 60 seconds), waiting for all
// requests to complete. If certFile and keyFile are provided, the server will be
// run with TLS enabled.
func NewServer(
	ctx context.Context,
	logger *slog.Logger,
	srv *http.Server,
	certFile, keyFile string,
) error {
	if srv == nil {
		panic("srv is nil")
	}
	switch {
	case srv.ReadTimeout <= -1:
		srv.ReadTimeout = 0
	case srv.ReadTimeout == 0:
		srv.ReadTimeout = 15 * time.Second
	}

	switch {
	case srv.WriteTimeout <= -1:
		srv.WriteTimeout = 0
	case srv.WriteTimeout == 0:
		srv.WriteTimeout = 15 * time.Second
	}

	switch {
	case srv.MaxHeaderBytes <= -1:
		srv.MaxHeaderBytes = http.DefaultMaxHeaderBytes
	case srv.MaxHeaderBytes == 0:
		srv.MaxHeaderBytes = http.DefaultMaxHeaderBytes
	}

	if srv.BaseContext == nil {
		srv.BaseContext = func(_ net.Listener) context.Context {
			return context.WithoutCancel(ctx)
		}
	}

	return NewServerWithoutDefaults(ctx, logger, srv, certFile, keyFile)
}

func withGracefulShutdown(
	ctx context.Context,
	logger *slog.Logger,
	srv *http.Server,
	certFile, keyFile string,
) error {
	errc := make(chan error)
	go func() {
		if certFile != "" && keyFile != "" {
			logger.LogAttrs(
				ctx,
				slog.LevelInfo,
				"starting tls http server",
				slog.String("cert_file", certFile),
				slog.String("key_file", keyFile),
				slog.String("addr", srv.Addr),
			)
			errc <- srv.ListenAndServeTLS(certFile, keyFile)
		} else {
			logger.LogAttrs(
				ctx,
				slog.LevelInfo,
				"starting http server",
				slog.String("addr", srv.Addr),
			)
			errc <- srv.ListenAndServe()
		}
	}()

	handle := func(err error) error {
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}

	select {
	case <-ctx.Done():
		logger.LogAttrs(
			ctx,
			slog.LevelInfo,
			"context cancelled, gracefully stopping http server",
			slog.String("addr", srv.Addr),
		)
		ctxt, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		return handle(srv.Shutdown(ctxt))
	case err := <-errc:
		return handle(err)
	}
}
