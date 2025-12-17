package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

type Logger struct {
	level LogLevel
	l     *log.Logger
}

func NewLogger(level LogLevel) *Logger {
	return &Logger{
		level: level,
		l:     log.New(os.Stdout, "", 0),
	}
}

func (lg *Logger) log(level LogLevel, msg string, fields map[string]any) {
	if level < lg.level {
		return
	}

	entry := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": level.String(),
		"msg":   msg,
	}

	for k, v := range fields {
		entry[k] = v
	}

	b, _ := json.Marshal(entry)
	lg.l.Println(string(b))
}

func (lg *Logger) Debug(msg string, f map[string]any) { lg.log(DEBUG, msg, f) }
func (lg *Logger) Info(msg string, f map[string]any)  { lg.log(INFO, msg, f) }
func (lg *Logger) Warn(msg string, f map[string]any)  { lg.log(WARN, msg, f) }
func (lg *Logger) Error(msg string, f map[string]any) { lg.log(ERROR, msg, f) }

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "debug"
	case INFO:
		return "info"
	case WARN:
		return "warn"
	case ERROR:
		return "error"
	default:
		return "unknown"
	}
}

func parseLogLevel(v string) LogLevel {
	switch v {
	case "debug":
		return DEBUG
	case "warn":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

var reqCounter uint64

func requestID() string {
	id := atomic.AddUint64(&reqCounter, 1)
	return strconv.FormatUint(id, 10) + "-" + strconv.FormatInt(rand.Int63(), 36)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	level := parseLogLevel(os.Getenv("LOG_LEVEL"))
	logger := NewLogger(level)

	startupDelay, _ := time.ParseDuration(os.Getenv("STARTUP_DELAY"))
	if startupDelay > 0 {
		logger.Info("startup delay", map[string]any{
			"delay": startupDelay.String(),
		})
		time.Sleep(startupDelay)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello from containerized go service\n"))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := withMiddleware(mux, logger)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	go func() {
		logger.Info("http server starting", map[string]any{
			"port": port,
		})

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", map[string]any{
				"error": err.Error(),
			})
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	logger.Warn("shutdown signal received", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", map[string]any{
			"error": err.Error(),
		})
	} else {
		logger.Info("server stopped", nil)
	}
}

func withMiddleware(next http.Handler, logger *Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}

		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			rid = requestID()
		}
		w.Header().Set("X-Request-Id", rid)

		logger.Debug("request started", map[string]any{
			"request_id": rid,
			"method":     r.Method,
			"path":       r.URL.Path,
		})

		next.ServeHTTP(rec, r)

		logger.Info("request completed", map[string]any{
			"request_id": rid,
			"method":     r.Method,
			"path":       r.URL.Path,
			"status":     rec.status,
			"duration":   time.Since(start).Milliseconds(),
		})
	})
}
