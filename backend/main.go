package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	log.Println("Starting the Ordelo Go backend")
	if err := run(); err != nil {
		log.Fatalf("Fatal error in server → %v", err)
	}
	log.Println("Server shutdown successfully with no errors")
}

func run() (err error) {
	// Handle graceful shutdown on Ctrl+C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Initialize Mongo and Redis
	log.Println("Initializing Mongo and Redis")
	mongoShutdown, err := initDB(ctx)
	if err != nil {
		return err
	}
	redisShutdown, err := initRedis(ctx)
	if err != nil {
		return err
	}

	// Cleanup resources on exit
	defer func() {
		log.Println("Cleaning up resources")
		err = errors.Join(err, mongoShutdown(context.Background()))
		err = errors.Join(err, redisShutdown(context.Background()))
	}()

	// Initialize cached repositories and auth service
	log.Println("Initializing cached repositories and auth service")
	if err = InitCachedMongoRepositories(ctx, RedisClient, MongoClient, 15*time.Minute); err != nil {
		return err
	}
	if err = InitAuthService(ctx, Repos, RedisClient, 15*time.Hour, 7*24*time.Hour); err != nil {
		return err
	}

	// Start HTTP server with simple health handler
	log.Println("Starting HTTP server")
	port := os.Getenv("PORT")
	if port == "" {
		return errors.New("environment variable PORT must be set")
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{
		Addr:         ":" + port,
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      handler,
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.ListenAndServe()
	}()

	select {
	case err = <-errChan:
	case <-ctx.Done():
		stop()
		err = srv.Shutdown(context.Background())
	}

	return err
}
