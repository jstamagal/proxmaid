package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/proxmaid/proxmaid/internal/api"
	"github.com/proxmaid/proxmaid/internal/array"
	"github.com/proxmaid/proxmaid/internal/disk"
	"github.com/proxmaid/proxmaid/internal/system"
)

const (
	defaultPort = "8484"
	version     = "0.1.0"
)

func main() {
	fmt.Printf("Proxmaid v%s starting...\n", version)

	// Initialize the system manager (kernel module, mounts)
	sysMgr := system.NewManager()

	// Initialize the array manager (wraps nmdctl / proc interface)
	arrayMgr := array.NewManager(sysMgr)

	// Initialize the disk manager
	diskMgr := disk.NewManager(sysMgr.MockMode)

	// Initialize API router
	router := api.NewRouter(arrayMgr, sysMgr, diskMgr)

	port := defaultPort
	if p := os.Getenv("PROXMAID_PORT"); p != "" {
		port = p
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nShutting down Proxmaid...")
		srv.Close()
	}()

	fmt.Printf("Proxmaid API listening on http://0.0.0.0:%s\n", port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
