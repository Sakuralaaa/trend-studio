package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/Sakuralaaa/trend-studio/internal/studio"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := studio.ConfigFromEnv()
	command := "api"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command == "storage-init" {
		return (&studio.App{Cfg: cfg}).StorageInit()
	}
	if command == "healthcheck" {
		client := &http.Client{Timeout: 3 * time.Second}
		response, err := client.Get("http://127.0.0.1:8080/health/ready")
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return fmt.Errorf("service not ready: %d", response.StatusCode)
		}
		return nil
	}
	app, err := studio.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer app.Close()
	switch command {
	case "migrate":
		return app.Migrate(ctx)
	case "bootstrap-admin":
		account := os.Getenv("BOOTSTRAP_ADMIN_USERNAME")
		if account == "" {
			account = os.Getenv("BOOTSTRAP_ADMIN_EMAIL")
		}
		return app.Bootstrap(ctx, account, os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"))
	case "worker":
		return app.Worker(ctx)
	case "serve":
		// A single service keeps the API and worker on the same private volume.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		if err := app.StorageInit(); err != nil {
			return err
		}
		if err := waitDatabase(ctx, app); err != nil {
			return err
		}
		if err := app.Migrate(ctx); err != nil {
			return err
		}
		workerDone := make(chan error, 1)
		go func() {
			workerDone <- app.Worker(ctx)
			cancel()
		}()
		err := serveAPI(ctx, app)
		cancel()
		return errors.Join(err, <-workerDone)
	case "api":
		return serveAPI(ctx, app)
	default:
		return fmt.Errorf("unknown command %s", command)
	}
}

func waitDatabase(ctx context.Context, app *studio.App) error {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	for {
		attempt, stop := context.WithTimeout(ctx, 5*time.Second)
		err := app.DB.Ping(attempt)
		stop()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("database startup: %w", ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}

func serveAPI(ctx context.Context, app *studio.App) error {
	server := &http.Server{Addr: app.Cfg.Addr, Handler: app.Router(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-done:
			return
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdown)
		}
	}()
	slog.Info("api listening", "address", app.Cfg.Addr)
	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
