package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var debug bool

func main() {
	debug = os.Getenv("DEBUG") == "true"

	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	logger.Println("service starting")
	logger.Printf("debug mode: %v\n", debug)

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(logger, r)

		if debug {
			logger.Println("[DEBUG] handling root request")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello from Go on port 8080!\n"))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		logRequest(logger, r)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Run server in background
	go func() {
		logger.Println("http server listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("listen error: %v", err)
		}
	}()

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	<-shutdown
	logger.Println("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Printf("server shutdown error: %v\n", err)
	} else {
		logger.Println("server shutdown complete")
	}
}

func logRequest(logger *log.Logger, r *http.Request) {
	logger.Printf(
		`request method=%s path=%s remote=%s user_agent=%q`,
		r.Method,
		r.URL.Path,
		r.RemoteAddr,
		r.UserAgent(),
	)
}
