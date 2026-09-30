package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"mini-device-fleet/internal/device"
)

type Config struct {
	Port             string
	HeartbeatTimeout time.Duration
}

func loadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	timeoutStr := os.Getenv("HEARTBEAT_TIMEOUT")
	timeout := 30 * time.Second
	if timeoutStr != "" {
		if parsed, err := time.ParseDuration(timeoutStr); err == nil && parsed > 0 {
			timeout = parsed
		} else {
			slog.Warn("invalid HEARTBEAT_TIMEOUT provided, defaulting to 30s", "val", timeoutStr)
		}
	}

	return Config{
		Port:             port,
		HeartbeatTimeout: timeout,
	}
}

// loggingMiddleware logs incoming HTTP requests with method, path, and duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Debug("handled request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start).String(),
		)
	})
}

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := loadConfig()

	// Dependency Injection: Repo -> Service -> Handler
	repo := device.NewMemoryRepository()
	service := device.NewService(repo, cfg.HeartbeatTimeout)
	handler := device.NewHandler(service)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Serve built React frontend from frontend/dist if available
	distDir := filepath.Join("frontend", "dist")
	if info, err := os.Stat(distDir); err == nil && info.IsDir() {
		distFS := http.FileServer(http.Dir(distDir))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			target := filepath.Join(distDir, filepath.Clean(r.URL.Path))
			if fInfo, err := os.Stat(target); err == nil && !fInfo.IsDir() {
				distFS.ServeHTTP(w, r)
				return
			}
			http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
		})
		slog.Info("Serving React frontend from frontend/dist", "path", distDir)
	}

	serverAddr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to catch interrupt / terminate signals for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("Starting Mini Device Fleet Monitor server",
			"port", cfg.Port,
			"heartbeat_timeout", cfg.HeartbeatTimeout.String(),
		)
		fmt.Printf("Mini Device Fleet Monitor running at http://localhost:%s\n", cfg.Port)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed to listen and serve", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-stop
	slog.Info("Shutdown signal received, shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("Server stopped cleanly")
}
