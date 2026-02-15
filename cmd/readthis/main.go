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
)

const shutdownTimeout = 10 * time.Second

func main() {
	cfg := config.Load()

	addr := flag.String("addr", cfg.Addr, "server address")
	dataPath := flag.String("data", cfg.DataPath, "path to data file")
	flag.Parse()

	cfg.Addr = *addr
	cfg.DataPath = *dataPath

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ReadThis server running"))
	})

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
