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
	"strings"
	"syscall"
	"time"

	"github.com/samzong/gofs/internal/config"
	"github.com/samzong/gofs/internal/filesystem"
	"github.com/samzong/gofs/internal/handler"
	"github.com/samzong/gofs/internal/middleware"
	"github.com/samzong/gofs/internal/server"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	flags := parseFlags()

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

	cfg, err := config.New(flags.Port, flags.Host, "", flags.Theme, flags.ShowHidden, flags.Dirs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}
	cfg.EnableWebDAV = flags.EnableWebDAV

	logger := setupLogger()
	logStartupInfo(logger, cfg, flags.Auth != "")

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

	fileHandler := createFileHandler(cfg, logger)
	webdavHandler := createWebDAVHandler(cfg, logger)

	srv := server.New(cfg, fileHandler, webdavHandler, authMiddleware, logger)

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("Server starting", slog.String("address", cfg.Address()))
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

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
	fmt.Println("  -d, --dir string    Directory mount (can be used multiple times)")
	fmt.Println("                      Format: [path:]dir[:ro][:name]")
	fmt.Println("                      Examples: -d \"/config:/etc/app:ro:Configuration\"")
	fmt.Println("                                -d \"/logs:/var/log::Application Logs\"")
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
	fmt.Println("  GOFS_DIR            Directory specification - single or semicolon-separated")
	fmt.Println("                      Examples: \"/srv/files\" or \"/config:/etc:ro;/logs:/var/log\"")
	fmt.Println("  GOFS_THEME          UI theme (default: default)")
	fmt.Println("  GOFS_SHOW_HIDDEN    Show hidden files (default: false)")
	fmt.Println("  GOFS_AUTH           Basic auth credentials (user:password)")
	fmt.Println("  GOFS_ENABLE_WEBDAV  Enable WebDAV server (default: false)")
	fmt.Println()
	fmt.Println("Note: Command line flags override environment variables")
}

func showVersion() {
	if buildTime != "" && buildTime != "unknown" {
		fmt.Printf("gofs version %s (built at %s)\n", version, buildTime)
		return
	}
	fmt.Printf("gofs version %s\n", version)
}

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ";")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type cmdFlags struct {
	Port         int
	Host         string
	Dirs         []string // Directory mounts
	Theme        string
	ShowHidden   bool
	Auth         string
	Help         bool
	Version      bool
	HealthCheck  bool
	EnableWebDAV bool
}

func parseFlags() *cmdFlags {
	f := &cmdFlags{}
	var dirs stringSlice

	flag.IntVar(&f.Port, "port", getEnv("GOFS_PORT", 8000), "Server port")
	flag.IntVar(&f.Port, "p", getEnv("GOFS_PORT", 8000), "Server port (shorthand)")
	flag.StringVar(&f.Host, "host", getEnv("GOFS_HOST", "127.0.0.1"), "Server host")
	flag.Var(&dirs, "d", "Directory mount (shorthand). Format: [path:]dir[:ro][:name]")
	flag.Var(&dirs, "dir", "Directory mount. Format: [path:]dir[:ro][:name]")
	flag.StringVar(&f.Theme, "theme", getEnv("GOFS_THEME", "default"), "UI theme")
	flag.BoolVar(&f.ShowHidden, "show-hidden", getEnv("GOFS_SHOW_HIDDEN", false), "Show hidden files")
	flag.BoolVar(&f.ShowHidden, "H", getEnv("GOFS_SHOW_HIDDEN", false), "Show hidden files (shorthand)")
	flag.StringVar(&f.Auth, "auth", getEnv("GOFS_AUTH", ""), "Basic auth (user:password)")
	flag.StringVar(&f.Auth, "a", getEnv("GOFS_AUTH", ""), "Basic auth (shorthand)")
	flag.BoolVar(&f.Help, "help", false, "Show help")
	flag.BoolVar(&f.Help, "h", false, "Show help (shorthand)")
	flag.BoolVar(&f.Version, "version", false, "Show version")
	flag.BoolVar(&f.Version, "v", false, "Show version (shorthand)")
	flag.BoolVar(&f.HealthCheck, "health-check", false, "Perform health check and exit")
	flag.BoolVar(&f.EnableWebDAV, "enable-webdav", getEnv("GOFS_ENABLE_WEBDAV", false), "Enable WebDAV server")

	flag.Parse()

	f.Dirs = parseDirConfig(dirs, "")
	return f
}

func parseDirConfig(cmdDirs []string, _ string) []string {
	if len(cmdDirs) > 0 {
		return cmdDirs
	}
	envDirs := getEnv("GOFS_DIR", "")
	if envDirs == "" {
		return []string{"."}
	}
	if strings.Contains(envDirs, ";") {
		return strings.Split(envDirs, ";")
	}
	return []string{envDirs}
}

func getEnv[T any](key string, defaultValue T) T {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

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

	if converted, ok := result.(T); ok {
		return converted
	}
	return defaultValue
}

func createFileHandler(cfg *config.Config, logger *slog.Logger) http.Handler {
	if len(cfg.Dirs) > 1 {
		return handler.NewMultiDir(cfg.Dirs, cfg, logger)
	}

	fs := filesystem.NewLocal(getRootDir(cfg), cfg.ShowHidden)
	if cfg.Theme == "advanced" {
		return handler.NewAdvancedFile(fs, cfg)
	}
	return handler.NewFile(fs, cfg, logger)
}

func createWebDAVHandler(cfg *config.Config, logger *slog.Logger) http.Handler {
	if !cfg.EnableWebDAV {
		return nil
	}

	fs := filesystem.NewLocal(getRootDir(cfg), cfg.ShowHidden)
	if len(cfg.Dirs) > 1 {
		logger.Warn("WebDAV only serves the first mounted directory",
			slog.String("webdav_root", cfg.Dirs[0].Dir),
			slog.String("webdav_mount", cfg.Dirs[0].Path))
	}
	logger.Info("WebDAV server enabled on /dav path (read-only)")
	return handler.NewWebDAV(fs, cfg, logger)
}

func getRootDir(cfg *config.Config) string {
	if len(cfg.Dirs) > 0 {
		return cfg.Dirs[0].Dir
	}
	return "."
}

func logStartupInfo(logger *slog.Logger, cfg *config.Config, authEnabled bool) {
	baseAttrs := []slog.Attr{
		slog.String("version", version),
		slog.String("address", cfg.Address()),
		slog.Bool("auth_enabled", authEnabled),
		slog.Bool("webdav_enabled", cfg.EnableWebDAV),
	}

	if len(cfg.Dirs) > 1 {
		dirInfo := make([]string, len(cfg.Dirs))
		for i, d := range cfg.Dirs {
			dirInfo[i] = fmt.Sprintf("%s->%s", d.Path, d.Dir)
		}
		baseAttrs = append(baseAttrs, slog.Any("directories", dirInfo))
	} else {
		baseAttrs = append(baseAttrs, slog.String("directory", getRootDir(cfg)))
	}

	logger.LogAttrs(context.Background(), slog.LevelInfo, "Starting gofs server", baseAttrs...)
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
