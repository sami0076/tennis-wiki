package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Serve runs the HTTP server until ctx is cancelled, then drains.
//
// Kubernetes sends SIGTERM and removes the pod from its endpoints at roughly
// the same moment, so a request already in flight has to be allowed to finish;
// closing the listener immediately would turn a routine rollout into visible
// errors.
func Serve(ctx context.Context, cfg Config, handler http.Handler, log *slog.Logger) error {
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.Addr, err)
	}
	return serve(ctx, cfg, listener, handler, log)
}

// serve is Serve with the listener already open, so a test can bind port zero
// and still know where to send a request.
func serve(ctx context.Context, cfg Config, listener net.Listener, handler http.Handler, log *slog.Logger) error {
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
		// Slow-loris protection: a client that dribbles out headers should not
		// hold a connection for the whole read timeout.
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("api: listening", "addr", listener.Addr().String())
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("serve: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("api: draining", "timeout", cfg.ShutdownTimeout)
	// A fresh context: the one that just expired cannot govern the drain.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	log.Info("api: stopped")
	return <-errCh
}
