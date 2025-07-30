package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/samzong/gofs/internal/config"
	"github.com/samzong/gofs/internal/filesystem"
	"github.com/samzong/gofs/internal/handler"
	"github.com/samzong/gofs/internal/middleware"
	"github.com/samzong/gofs/internal/server"
)

var (
	// Version contains the build version information set by linker flags.
	Version = "dev"
	// GitCommit contains the git commit hash set by linker flags.
	GitCommit = "unknown"
	// BuildTime contains the build timestamp set by linker flags.
	BuildTime = "unknown"
)

func main() {
	// Parse command line flags using Go's standard flag package
	var (
		port         = flag.Int("port", getEnvInt("GOFS_PORT", 8000), "Server port")
		portShort    = flag.Int("p", getEnvInt("GOFS_PORT", 8000), "Server port (shorthand)")
		host         = flag.String("host", getEnvString("GOFS_HOST", "127.0.0.1"), "Server host")
		dir          = flag.String("dir", getEnvString("GOFS_DIR", "."), "Root directory")
		dirShort     = flag.String("d", getEnvString("GOFS_DIR", "."), "Root directory (shorthand)")
		theme        = flag.String("theme", getEnvString("GOFS_THEME", "default"), "UI theme (default, classic)")
		showHidden   = flag.Bool("show-hidden", getEnvBool("GOFS_SHOW_HIDDEN", false), "Show hidden files")
		hiddenShort  = flag.Bool("H", getEnvBool("GOFS_SHOW_HIDDEN", false), "Show hidden files (shorthand)")
		auth         = flag.String("auth", getEnvString("GOFS_AUTH", ""), "Basic auth (user:password)")
		authShort    = flag.String("a", getEnvString("GOFS_AUTH", ""), "Basic auth (shorthand)")
		help         = flag.Bool("help", false, "Show help")
		helpShort    = flag.Bool("h", false, "Show help (shorthand)")
		version      = flag.Bool("version", false, "Show version")
		versionShort = flag.Bool("v", false, "Show version (shorthand)")
		healthCheck  = flag.Bool("health-check", false, "Perform health check and exit")
	)

	flag.Parse()

	// Handle special flags first
	if *help || *helpShort {
		showHelp()
		return
	}

	if *version || *versionShort {
		showVersion()
		return
	}

	if *healthCheck {
		if err := performHealthCheck(); err != nil {
			fmt.Fprintf(os.Stderr, "Health check FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Health check: OK")
		os.Exit(0)
		return
	}

	// Resolve flag precedence (shorthand flags override long flags if both are provided)
	finalPort := *port
	if flag.Lookup("p").Value.String() != flag.Lookup("p").DefValue {
		finalPort = *portShort
	}

	finalDir := *dir
	if flag.Lookup("d").Value.String() != flag.Lookup("d").DefValue {
		finalDir = *dirShort
	}

	finalShowHidden := *showHidden || *hiddenShort

	finalAuth := *auth
	if *authShort != "" {
		finalAuth = *authShort
	}

	// Create configuration
	cfg, err := config.New(finalPort, *host, finalDir, *theme, finalShowHidden)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Setup structured logging with slog
	logger := setupLogger()
	logger.Info("Starting gofs server",
		slog.String("version", Version),
		slog.String("address", cfg.Address()),
		slog.String("directory", cfg.Dir),
		slog.Bool("auth_enabled", finalAuth != ""),
	)

	// Create authentication middleware if needed
	var authMiddleware *middleware.BasicAuth
	if finalAuth != "" {
		authMiddleware, err = middleware.NewBasicAuthFromCredentials(finalAuth)
		if err != nil {
			logger.Error("Authentication setup failed", slog.Any("error", err))
			fmt.Fprintf(os.Stderr, "Authentication error: %v\n", err)
			os.Exit(1)
		}
		logger.Info("HTTP Basic Authentication enabled")
	}

	// Initialize server components
	fs := filesystem.NewLocal(cfg.Dir, cfg.ShowHidden)
	fileHandler := handler.NewFile(fs, cfg)
	srv := server.New(cfg, fileHandler, authMiddleware, logger)

	// Start server
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("Server starting", slog.String("address", cfg.Address()))
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// Wait for shutdown signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Error("Server failed to start", slog.Any("error", err))
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	case sig := <-shutdown:
		logger.Info("Shutdown signal received", slog.String("signal", sig.String()))

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("Server shutdown failed", slog.Any("error", err))
			fmt.Fprintf(os.Stderr, "Server shutdown error: %v\n", err)
			os.Exit(1)
		}

		logger.Info("Server stopped gracefully")
	}
}

func showHelp() {
	fmt.Println("gofs - A lightweight HTTP file server written in Go")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gofs [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -a, --auth string   Enable HTTP Basic Authentication with user:password format")
	fmt.Println("  -d, --dir string    Root directory to serve files from (default \".\")")
	fmt.Println("  -h, --help          Show this help message and exit")
	fmt.Println("  -H, --show-hidden   Show hidden files and directories")
	fmt.Println("      --host string   Server host address to bind to (default \"127.0.0.1\")")
	fmt.Println("  -p, --port int      Server port number to listen on (default 8000)")
	fmt.Println("      --theme string  UI theme: default (minimal), classic (Windows-style) (default \"default\")")
	fmt.Println("  -v, --version       Show version information and exit")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  GOFS_PORT           Server port (default: 8000)")
	fmt.Println("  GOFS_HOST           Server host (default: 127.0.0.1)")
	fmt.Println("  GOFS_DIR            Root directory (default: .)")
	fmt.Println("  GOFS_THEME          UI theme (default: default)")
	fmt.Println("  GOFS_SHOW_HIDDEN    Show hidden files (default: false)")
	fmt.Println("  GOFS_AUTH           Basic auth credentials (user:password)")
	fmt.Println()
	fmt.Println("Note: Command line flags override environment variables")
}

func showVersion() {
	fmt.Printf("gofs %s\n", Version)
	fmt.Printf("Git commit: %s\n", GitCommit)
	fmt.Printf("Build time: %s\n", BuildTime)
}

// Environment variable helper functions
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// performHealthCheck performs a comprehensive health check
func performHealthCheck() error {
	// Test configuration loading
	testCfg, err := config.New(8000, "127.0.0.1", ".", "default", false)
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Test filesystem access
	testFS := filesystem.NewLocal(testCfg.Dir, false)
	if _, err := testFS.ReadDir("/"); err != nil {
		return fmt.Errorf("filesystem access error: %w", err)
	}

	// Test logger initialization
	logger := setupLogger()
	if logger == nil {
		return errors.New("logger initialization failed")
	}

	return nil
}

// setupLogger creates a simple, effective logger using Go's slog
func setupLogger() *slog.Logger {
	// Use JSON format for production, text for development
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	// Override level if specified
	if level := os.Getenv("GOFS_LOG_LEVEL"); level != "" {
		switch level {
		case "debug":
			handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
		case "warn":
			handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn})
		case "error":
			handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})
		}
	}

	// Use JSON format in production
	if os.Getenv("GOFS_ENV") == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	return slog.New(handler)
}
