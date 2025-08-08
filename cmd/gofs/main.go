package main

import (
	"context"
	"encoding/json"
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
	// version contains the build version information set by linker flags.
	version = "dev"
	// gitCommit contains the git commit hash set by linker flags.
	gitCommit = "unknown"
	// buildTime contains the build timestamp set by linker flags.
	buildTime = "unknown"
	// goVersion contains the Go version used to build the binary.
	goVersion = "unknown"
)

func main() {
	// Parse command line flags with simplified approach
	flags := parseFlags()

	// Handle special flags first
	if flags.Help {
		showHelp()
		return
	}

	if flags.Version {
		showVersion()
		return
	}

	if flags.HealthCheck {
		if err := performHealthCheckAndExit(); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Create configuration
	cfg, err := config.New(flags.Port, flags.Host, flags.Dir, flags.Theme, flags.ShowHidden)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}
	cfg.EnableWebDAV = flags.EnableWebDAV

	// Setup structured logging with slog
	logger := setupLogger()

	logger.Info("Starting gofs server",
		slog.String("version", version),
		slog.String("address", cfg.Address()),
		slog.String("directory", cfg.Dir),
		slog.Bool("auth_enabled", flags.Auth != ""),
		slog.Bool("webdav_enabled", cfg.EnableWebDAV),
	)

	// Create authentication middleware if needed
	var authMiddleware *middleware.BasicAuth
	if flags.Auth != "" {
		authMiddleware, err = middleware.NewBasicAuthFromCredentials(flags.Auth)
		if err != nil {
			logger.Error("Authentication setup failed", slog.Any("error", err))
			fmt.Fprintf(os.Stderr, "Authentication error: %v\n", err)
			os.Exit(1)
		}
		logger.Info("HTTP Basic Authentication enabled")
	}

	// Initialize server components
	fs := filesystem.NewLocal(cfg.Dir, cfg.ShowHidden)

	// Use advanced handler for advanced theme, regular handler otherwise
	var fileHandler http.Handler
	if cfg.Theme == "advanced" {
		fileHandler = handler.NewAdvancedFile(fs, cfg)
	} else {
		fileHandler = handler.NewFile(fs, cfg, logger)
	}

	// Create WebDAV handler if enabled
	var webdavHandler http.Handler
	if cfg.EnableWebDAV {
		webdavHandler = handler.NewWebDAV(fs, cfg, logger)
		logger.Info("WebDAV server enabled on /dav path (read-only)")
	}

	srv := server.New(cfg, fileHandler, webdavHandler, authMiddleware, logger)

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

		if err := srv.Shutdown(ctx); err != nil {
			cancel()
			logger.Error("Server shutdown failed", slog.Any("error", err))
			fmt.Fprintf(os.Stderr, "Server shutdown error: %v\n", err)
			os.Exit(1)
		}
		cancel()

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
	fmt.Println("      --theme string  UI theme: default, classic, advanced (default \"default\")")
	fmt.Println("      --enable-webdav Enable WebDAV server on /dav path (read-only)")
	fmt.Println("  -v, --version       Show version information and exit")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  GOFS_PORT           Server port (default: 8000)")
	fmt.Println("  GOFS_HOST           Server host (default: 127.0.0.1)")
	fmt.Println("  GOFS_DIR            Root directory (default: .)")
	fmt.Println("  GOFS_THEME          UI theme (default: default)")
	fmt.Println("  GOFS_SHOW_HIDDEN    Show hidden files (default: false)")
	fmt.Println("  GOFS_AUTH           Basic auth credentials (user:password)")
	fmt.Println("  GOFS_ENABLE_WEBDAV  Enable WebDAV server (default: false)")
	fmt.Println()
	fmt.Println("Note: Command line flags override environment variables")
}

func showVersion() {
	fmt.Printf("gofs %s\n", version)
	fmt.Printf("Git commit: %s\n", gitCommit)
	fmt.Printf("Build time: %s\n", buildTime)
	fmt.Printf("Go version: %s\n", goVersion)
}

// cmdFlags represents all command line flags and their values
type cmdFlags struct {
	Port         int
	Host         string
	Dir          string
	Theme        string
	ShowHidden   bool
	Auth         string
	Help         bool
	Version      bool
	HealthCheck  bool
	EnableWebDAV bool
}

// parseFlags parses command line flags with both long and short forms
func parseFlags() *cmdFlags {
	f := &cmdFlags{}

	// Define both long and short forms
	port := flag.Int("port", getEnv("GOFS_PORT", 8000), "Server port")
	portShort := flag.Int("p", 0, "Server port (shorthand)")
	host := flag.String("host", getEnv("GOFS_HOST", "127.0.0.1"), "Server host")
	dir := flag.String("dir", getEnv("GOFS_DIR", "."), "Root directory")
	dirShort := flag.String("d", "", "Root directory (shorthand)")
	theme := flag.String("theme", getEnv("GOFS_THEME", "default"), "UI theme")
	showHidden := flag.Bool("show-hidden", getEnv("GOFS_SHOW_HIDDEN", false), "Show hidden files")
	hiddenShort := flag.Bool("H", false, "Show hidden files (shorthand)")
	auth := flag.String("auth", getEnv("GOFS_AUTH", ""), "Basic auth (user:password)")
	authShort := flag.String("a", "", "Basic auth (shorthand)")
	help := flag.Bool("help", false, "Show help")
	helpShort := flag.Bool("h", false, "Show help (shorthand)")
	versionFlag := flag.Bool("version", false, "Show version")
	versionShort := flag.Bool("v", false, "Show version (shorthand)")
	healthCheck := flag.Bool("health-check", false, "Perform health check and exit")
	enableWebDAV := flag.Bool("enable-webdav", getEnv("GOFS_ENABLE_WEBDAV", false), "Enable WebDAV server on /dav path")

	flag.Parse()

	// Resolve values with shorthand precedence
	f.Port = *port
	if *portShort != 0 {
		f.Port = *portShort
	}

	f.Host = *host
	f.Dir = *dir
	if *dirShort != "" {
		f.Dir = *dirShort
	}

	f.Theme = *theme
	f.ShowHidden = *showHidden || *hiddenShort

	f.Auth = *auth
	if *authShort != "" {
		f.Auth = *authShort
	}

	f.Help = *help || *helpShort
	f.Version = *versionFlag || *versionShort
	f.HealthCheck = *healthCheck
	f.EnableWebDAV = *enableWebDAV

	return f
}

// getEnv is a generic function to get environment variables with type conversion
func getEnv[T any](key string, defaultValue T) T {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	// Type switch to handle different types
	var result any
	switch any(defaultValue).(type) {
	case string:
		result = value
	case int:
		if intVal, err := strconv.Atoi(value); err == nil {
			result = intVal
		} else {
			return defaultValue
		}
	case bool:
		if boolVal, err := strconv.ParseBool(value); err == nil {
			result = boolVal
		} else {
			return defaultValue
		}
	default:
		return defaultValue
	}

	return result.(T)
}

// performHealthCheck performs a lightweight health check via HTTP
func performHealthCheck() error {
	// Default health check endpoint
	healthURL := "http://127.0.0.1:8000/healthz"

	// Check if custom host/port is configured via environment
	if host := os.Getenv("GOFS_HOST"); host != "" {
		port := os.Getenv("GOFS_PORT")
		if port == "" {
			port = "8000"
		}
		healthURL = fmt.Sprintf("http://%s:%s/healthz", host, port)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Perform health check request
	resp, err := client.Get(healthURL)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// setupLogger creates a logger with environment-based configuration
func setupLogger() *slog.Logger {
	level := parseLogLevel(os.Getenv("GOFS_LOG_LEVEL"))
	opts := &slog.HandlerOptions{Level: level}

	// Use JSON format in production, text otherwise
	if os.Getenv("GOFS_ENV") == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

// parseLogLevel converts string to slog.Level
func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type HealthResult struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

func performHealthCheckAndExit() error {
	if err := performHealthCheck(); err != nil {
		result := HealthResult{
			Status:    "FAILED",
			Timestamp: time.Now(),
			Error:     err.Error(),
		}
		_ = json.NewEncoder(os.Stdout).Encode(result)
		return err
	}

	result := HealthResult{
		Status:    "OK",
		Timestamp: time.Now(),
	}
	_ = json.NewEncoder(os.Stdout).Encode(result)
	return nil
}
