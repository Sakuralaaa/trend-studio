package main

import (
	"context"
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
		return app.Bootstrap(ctx, os.Getenv("BOOTSTRAP_ADMIN_EMAIL"), os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"))
	case "worker":
		return app.Worker(ctx)
	case "api":
		server := &http.Server{Addr: cfg.Addr, Handler: app.Router(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second}
		go func() {
			<-ctx.Done()
			shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdown)
		}()
		slog.Info("api listening", "address", cfg.Addr)
		err := server.ListenAndServe()
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	default:
		return fmt.Errorf("unknown command %s", command)
	}
}
