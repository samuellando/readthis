// Command readthis starts the ReadThis HTTP server.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"readthis/internal/config"
	"readthis/internal/httpapi"
	"readthis/internal/storage"
	"readthis/internal/webui"
)

// shutdownTimeout limits graceful shutdown duration.
const shutdownTimeout = 10 * time.Second

func main() {
	cfg := config.Load()

	addr := flag.String("addr", cfg.Addr, "server address")
	dataPath := flag.String("data", cfg.DataPath, "path to data file")
	flag.Parse()

	cfg.Addr = *addr
	cfg.DataPath = *dataPath

	store := storage.NewFileStore(cfg.DataPath)
	api := httpapi.New(store)
	ui, err := webui.New(store, time.Now)
	if err != nil {
		log.Fatalf("readthis: failed to load templates: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", api.Handler())
	mux.Handle("/", ui.Handler())

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("readthis: starting server on %s (data: %s)", cfg.Addr, cfg.DataPath)
		serverErr <- server.ListenAndServe()
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("readthis: server error: %v", err)
		}
	case sig := <-signalCh:
		log.Printf("readthis: received %s, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("readthis: shutdown error: %v", err)
		}
	}
}
