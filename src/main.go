package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, closeStore, err := newStoreFromConfig(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer closeStore()

	server := NewServer(store)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("shopping cart API listening on :%s using backend=%s", cfg.Port, cfg.StoreBackend)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
}
