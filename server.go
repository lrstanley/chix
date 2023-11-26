// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/apex/log"
	"golang.org/x/sync/errgroup"
)

const (
	srvDefaultReadTimeout    = 15 * time.Second
	srvDefaultWriteTimeout   = 15 * time.Second
	srvDefaultMaxHeaderBytes = 1 << 20
	srvCancelTimeout         = 10 * time.Second
)

type Runner func(ctx context.Context) error

func (r Runner) Invoke(ctx context.Context) func() error {
	fn := func() error {
		return r(ctx)
	}
	return fn
}

func RunnerInterval(name string, r Runner, frequency time.Duration, runImmediately, exitOnError bool) Runner {
	return func(ctx context.Context) error {
		logEntry := log.FromContext(ctx).WithField("runner", name)
		ctx = log.NewContext(ctx, logEntry)

		var lastRun time.Time

		if runImmediately {
			lastRun = time.Now()
			logEntry.Info("invoking runner")
			if err := r(ctx); err != nil {
				logEntry.WithError(err).WithDuration(time.Since(lastRun)).Error("invocation failed")
				return err
			}
			logEntry.WithDuration(time.Since(lastRun)).Info("invocation complete")
		}

		ticker := time.NewTicker(frequency)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				lastRun = time.Now()
				logEntry.Info("invoking runner")
				if err := r(ctx); err != nil {
					logEntry.WithError(err).WithDuration(time.Since(lastRun)).Error("invocation failed")

					if exitOnError {
						return err
					}

					logEntry.WithDuration(time.Since(lastRun)).Info("invocation complete")
					continue
				}
			}
		}
	}
}

// Run runs the provided http server, and listens for any termination signals
// (SIGINT, SIGTERM, SIGQUIT, etc). If runners are provided, those will run
// concurrently.
//
// If the http server, or any runners return an error, all runners will
// terminate (assuming they listen to the provided context), and the first
// known error will be returned. The http server will be gracefully shut down,
// with a timeout of 10 seconds.
func Run(srv *http.Server, runners ...Runner) error {
	return RunCtx(context.Background(), srv, runners...)
}

// RunTLS is the same as Run, but allows for TLS to be used.
func RunTLS(srv *http.Server, certFile, keyFile string, runners ...Runner) error {
	return RunTLSContext(context.Background(), srv, certFile, keyFile, runners...)
}

// Deprecated: Use [RunContext] instead.
func RunCtx(ctx context.Context, srv *http.Server, runners ...Runner) error {
	return RunContext(ctx, srv, runners...)
}

// RunContext is the same as Run, but with the provided context that can be used
// to externally cancel all runners and the http server.
func RunContext(ctx context.Context, srv *http.Server, runners ...Runner) error {
	serverSetDefaults(srv)

	var g *errgroup.Group
	g, ctx = errgroup.WithContext(ctx)

	g.Go(func() error {
		return signalListener(ctx)
	})

	g.Go(func() error {
		return httpServer(ctx, srv, "", "")
	})

	for _, runner := range runners {
		g.Go(runner.Invoke(ctx))
	}

	return g.Wait()
}

// RunTLSContext is the same as Run, but with the provided context that can be used
// to externally cancel all runners and the http server, and also allows for TLS
// to be used.
func RunTLSContext(ctx context.Context, srv *http.Server, certFile, keyFile string, runners ...Runner) error {
	serverSetDefaults(srv)

	var g *errgroup.Group
	g, ctx = errgroup.WithContext(ctx)

	g.Go(func() error {
		return signalListener(ctx)
	})

	g.Go(func() error {
		return httpServer(ctx, srv, certFile, keyFile)
	})

	for _, runner := range runners {
		g.Go(runner.Invoke(ctx))
	}

	return g.Wait()
}

func serverSetDefaults(srv *http.Server) {
	if srv.ReadTimeout == 0 {
		srv.ReadTimeout = srvDefaultReadTimeout
	}

	if srv.WriteTimeout == 0 {
		srv.WriteTimeout = srvDefaultWriteTimeout
	}

	if srv.MaxHeaderBytes == 0 {
		srv.MaxHeaderBytes = srvDefaultMaxHeaderBytes
	}
}

func signalListener(ctx context.Context) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case sig := <-quit:
		return fmt.Errorf("received signal: %v", sig)
	case <-ctx.Done():
		return nil
	}
}

func httpServer(ctx context.Context, srv *http.Server, certFile, keyFile string) error {
	ch := make(chan error)
	go func() {
		var err error

		if certFile != "" && keyFile != "" {
			err = srv.ListenAndServeTLS(certFile, keyFile)
		} else {
			err = srv.ListenAndServe()
		}

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			ch <- err
		}
		close(ch)
	}()

	select {
	case <-ctx.Done():
	case err := <-ch:
		return err
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), srvCancelTimeout)
	defer cancel()

	return srv.Shutdown(ctxTimeout)
}
