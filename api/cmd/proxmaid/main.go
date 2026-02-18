package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/proxmaid/proxmaid/internal/api"
	"github.com/proxmaid/proxmaid/internal/app"
	"github.com/proxmaid/proxmaid/internal/array"
	"github.com/proxmaid/proxmaid/internal/auth"
	"github.com/proxmaid/proxmaid/internal/cache"
	"github.com/proxmaid/proxmaid/internal/disk"
	"github.com/proxmaid/proxmaid/internal/notify"
	"github.com/proxmaid/proxmaid/internal/scheduler"
	"github.com/proxmaid/proxmaid/internal/share"
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

	// Initialize the cache manager (mergerfs + mover)
	cacheMgr := cache.NewManager(sysMgr.MockMode, diskMgr)

	// Initialize the share manager
	shareMgr := share.NewManager(sysMgr.MockMode)

	// Initialize the app manager (Docker)
	appMgr := app.NewManager(sysMgr.MockMode)

	// Initialize the notification manager
	notifyMgr := notify.NewManager(sysMgr.MockMode)

	// Initialize the auth manager
	authMgr := auth.NewManager(sysMgr.MockMode)

	// Initialize the scheduler
	schedMgr := scheduler.NewManager(sysMgr.MockMode)

	// Register built-in scheduled tasks
	schedMgr.RegisterTask(scheduler.ScheduledTask{
		Name:     "parity_check",
		Schedule: "0 0 1 * *", // 1st of each month at midnight
		Enabled:  true,
		Action:   func() { arrayMgr.Check("CORRECT") },
	})
	schedMgr.RegisterTask(scheduler.ScheduledTask{
		Name:     "smart_short",
		Schedule: "0 3 * * 0", // Sundays at 3am
		Enabled:  true,
		Action:   func() { fmt.Println("[SCHEDULER] Running short SMART test on all disks") },
	})
	schedMgr.RegisterTask(scheduler.ScheduledTask{
		Name:     "smart_long",
		Schedule: "0 2 1 * *", // 1st of month at 2am
		Enabled:  false,
		Action:   func() { fmt.Println("[SCHEDULER] Running long SMART test on all disks") },
	})
	schedMgr.RegisterTask(scheduler.ScheduledTask{
		Name:     "recycle_purge",
		Schedule: "0 4 * * *", // Daily at 4am
		Enabled:  true,
		Action:   func() { fmt.Println("[SCHEDULER] Purging recycle bin (30+ day old files)") },
	})

	// Initialize API router
	router := api.NewRouter(arrayMgr, sysMgr, diskMgr, cacheMgr, shareMgr, appMgr, notifyMgr, authMgr, schedMgr)

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
