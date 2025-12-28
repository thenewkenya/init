package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	requestCount uint64
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	logger.Println("service starting")

	// Periodic background logs
	go heartbeat(logger, 5*time.Second)
	go backgroundWorker(logger, 7*time.Second)

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint64(&requestCount, 1)

		logger.Printf(
			"request received method=%s path=%s remote=%s",
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
		)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello\n"))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:    ":8090",
		Handler: mux,
	}

	go func() {
		logger.Println("http server listening on :8090")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("server error: %v", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	logger.Println("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
	} else {
		logger.Println("server stopped cleanly")
	}
}

func heartbeat(logger *log.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for t := range ticker.C {
		logger.Printf(
			"heartbeat alive=true uptime=%s requests=%d",
			time.Since(t.Add(-interval)).Truncate(time.Second),
			atomic.LoadUint64(&requestCount),
		)
	}
}

func backgroundWorker(logger *log.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		logger.Println("background job started")

		// Simulated work
		time.Sleep(500 * time.Millisecond)

		logger.Println("background job completed")
	}
}
