package main

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

var (
	RedisClient  *redis.Client
	redis_source = slog.Any("source", "redis-cache")
)

func initRedis(c context.Context) (shutdown func(ctx context.Context) error, err error) {
	ctx, span := Tracer.Start(c, "initRedis")
	defer span.End()

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		err = errors.New("env variable REDIS_URL is empty")
		return
	}

	// Parse the Redis URL
	parsedURL, err := url.Parse(redisURL)
	if err != nil {
		err = errors.New("invalid REDIS_URL format")
		return
	}

	// Extract password from URL
	password := ""
	if parsedURL.User != nil {
		password, _ = parsedURL.User.Password()
	}

	// Get host and port
	addr := parsedURL.Host
	if !strings.Contains(addr, ":") {
		addr = addr + ":6379" // Default Redis port
	}

	Logger.InfoContext(ctx, "Setting up redis with Opentelemetry", slog.String("Addr", addr),
		slog.String("Password", password), slog.Int("DB", 0), redis_source)

	var redisShutDownFunc func() error
	shutdown = func(ctx context.Context) (err error) {
		if redisShutDownFunc != nil {
			done := make(chan error, 1)
			goRoutineCopy := redisShutDownFunc
			go func() {
				done <- goRoutineCopy()
			}()
			redisShutDownFunc = nil
			select {
			case err = <-done:
				return
			case <-ctx.Done():
				Logger.ErrorContext(context.Background(), "Redis shutdown context timed out", slog.Any("Error", ctx.Err()), redis_source)
				return ctx.Err()
			}
		}
		return
	}

	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})
	if RedisClient == nil {
		Logger.ErrorContext(ctx, "New client function returned nil", slog.String("error", "client returned nil"), redis_source)
		err = errors.New("new client function returned nil")
		return
	}

	redisShutDownFunc = RedisClient.Close
	if err = redisotel.InstrumentTracing(RedisClient); err != nil {
		Logger.ErrorContext(ctx, "Closing redis client since failed to instrument Redis tracing", slog.Any("error", err), redis_source)
		handleErr(err)
		return
	}

	if err = redisotel.InstrumentMetrics(RedisClient); err != nil {
		Logger.ErrorContext(ctx, "Closing redis client since failed to instrument Redis metrics", slog.Any("error", err), redis_source)
		handleErr(err)
		return
	}

	if err = RedisClient.Ping(ctx).Err(); err != nil {
		Logger.ErrorContext(ctx, "Redis ping test failed", slog.Any("error", err), redis_source)
		handleErr(err)
		return
	}

	Logger.InfoContext(ctx, "Connected Successfully to Redis", slog.String("Ping", "Success"), redis_source)
	return
}
