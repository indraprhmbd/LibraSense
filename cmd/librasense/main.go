package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"librasense/internal/config"
	"librasense/internal/httpapi"
	"librasense/internal/store"
	webui "librasense/web"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("konfigurasi: %v", err)
	}

	database, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer database.Close()

	if err := database.EnsureDevice(cfg.DeviceID, cfg.DeviceToken, "Perpustakaan Pintar"); err != nil {
		log.Fatalf("daftar perangkat: %v", err)
	}

	assets, err := webui.FS()
	if err != nil {
		log.Fatalf("aset web: %v", err)
	}

	server := httpapi.New(cfg, database, assets)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("LibraSense mendengarkan di http://0.0.0.0:%d (akses bypass: %v)", cfg.Port, cfg.BypassAccess)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdown := make(chan struct{})
	go func() {
		_ = httpServer.Close()
		close(shutdown)
	}()

	select {
	case <-shutdown:
	case <-time.After(5 * time.Second):
	}

	log.Print("berhenti")
}
