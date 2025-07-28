package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/samzong/gofs/internal/config"
	"github.com/samzong/gofs/internal/filesystem"
	"github.com/samzong/gofs/internal/handler"
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
	// Parse command line flags
	var (
		port     = flag.Int("port", 8000, "Server port")
		portP    = flag.Int("p", 8000, "Server port (shorthand)")
		host     = flag.String("host", "127.0.0.1", "Server host")
		dir      = flag.String("dir", ".", "Root directory")
		dirD     = flag.String("d", ".", "Root directory (shorthand)")
		help     = flag.Bool("help", false, "Show help")
		helpH    = flag.Bool("h", false, "Show help (shorthand)")
		version  = flag.Bool("version", false, "Show version")
		versionV = flag.Bool("v", false, "Show version (shorthand)")
	)
	flag.Parse()

	if *help || *helpH {
		showHelp()
		return
	}

	if *version || *versionV {
		showVersion()
		return
	}

	// Use short flag values if provided, otherwise use long flag values
	finalPort := *port
	if *portP != 8000 {
		finalPort = *portP
	}

	finalDir := *dir
	if *dirD != "." {
		finalDir = *dirD
	}

	cfg, err := config.New(finalPort, *host, finalDir)
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	fs := filesystem.NewLocal(cfg.Dir)
	fileHandler := handler.NewFile(fs)
	srv := server.New(cfg, fileHandler)

	go func() {
		fmt.Printf("Starting gofs server...\n")
		fmt.Printf("Serving directory: %s\n", cfg.Dir)
		fmt.Printf("Server running at: http://%s\n", cfg.Address())

		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server start error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	if err := srv.Shutdown(ctx); err != nil {
		cancel()
		log.Fatalf("Server shutdown error: %v", err)
	}
	cancel()

	fmt.Println("Server stopped")
}

func showHelp() {
	fmt.Println("gofs - A lightweight HTTP file server written in Go")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gofs [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -d, -dir string")
	fmt.Println("        Root directory to serve files from (default \".\")")
	fmt.Println("  -h, -help")
	fmt.Println("        Show this help message and exit")
	fmt.Println("  -host string")
	fmt.Println("        Server host address to bind to (default \"127.0.0.1\")")
	fmt.Println("  -p, -port int")
	fmt.Println("        Server port number to listen on (default 8000)")
	fmt.Println("  -v, -version")
	fmt.Println("        Show version information and exit")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  gofs                          Serve current directory on port 8000")
	fmt.Println("  gofs -port 3000               Serve current directory on port 3000")
	fmt.Println("  gofs -dir /tmp                Serve /tmp directory on port 8000")
	fmt.Println("  gofs -host 0.0.0.0 -port 8080 Serve on all interfaces, port 8080")
	fmt.Println("  gofs -dir /var/www -port 80   Serve /var/www on port 80")
	fmt.Println()
	fmt.Println("For more information, visit: https://github.com/samzong/gofs")
}

func showVersion() {
	fmt.Printf("gofs %s\n", Version)
	fmt.Printf("Git commit: %s\n", GitCommit)
	fmt.Printf("Build time: %s\n", BuildTime)
}
