package main

import (
	"context"
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
		port    = flag.Int("port", 8000, "Server port")
		host    = flag.String("host", "127.0.0.1", "Server host")
		dir     = flag.String("dir", ".", "Root directory")
		help    = flag.Bool("help", false, "Show help")
		version = flag.Bool("version", false, "Show version")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	if *version {
		showVersion()
		return
	}

	cfg, err := config.New(*port, *host, *dir)
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

		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server start error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	fmt.Println("Server stopped")
}

func showHelp() {
	fmt.Println("gofs - A lightweight HTTP file server")
	fmt.Println("\nUsage:")
	flag.PrintDefaults()
	fmt.Println("\nExamples:")
	fmt.Println("  gofs                          # Serve current directory on port 8000")
	fmt.Println("  gofs -port 3000 -dir /tmp     # Serve /tmp on port 3000")
}

func showVersion() {
	fmt.Printf("gofs %s\n", Version)
	fmt.Printf("Git commit: %s\n", GitCommit)
	fmt.Printf("Build time: %s\n", BuildTime)
}
