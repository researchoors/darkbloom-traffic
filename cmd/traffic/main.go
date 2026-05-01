package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/researchoors/darkbloom-traffic/internal/client"
	"github.com/researchoors/darkbloom-traffic/internal/config"
	"github.com/researchoors/darkbloom-traffic/internal/generator"
	"github.com/researchoors/darkbloom-traffic/internal/models"
	"github.com/researchoors/darkbloom-traffic/internal/ratelimit"
	"github.com/researchoors/darkbloom-traffic/internal/stats"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// 1. Load config from env, validate.
	cfg := config.Default()
	cfg.FromEnv()
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}
	slog.Info("config loaded",
		"baseURL", cfg.BaseURL,
		"targetTPS", cfg.TargetTPS,
		"maxRPS", cfg.MaxRPS,
		"modelRefreshInterval", cfg.ModelRefreshInterval,
		"requestTimeout", cfg.RequestTimeout,
		"statsInterval", cfg.StatsInterval,
	)

	// 2. Create discoverer, fetch models, fail if 0.
	discoverer := models.NewDiscoverer(cfg.BaseURL, cfg.APIKey)
	ctx0, cancel0 := context.WithTimeout(context.Background(), 30*time.Second)
	if err := discoverer.Refresh(ctx0); err != nil {
		cancel0()
		slog.Error("initial model discovery failed", "error", err)
		os.Exit(1)
	}
	cancel0()

	modelList := discoverer.Models()
	if len(modelList) == 0 {
		slog.Error("no models discovered, exiting")
		os.Exit(1)
	}
	slog.Info("models discovered", "count", discoverer.Count())

	// 3. Graceful shutdown via signal.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 4. Background goroutine to refresh models.
	go func() {
		ticker := time.NewTicker(cfg.ModelRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := discoverer.Refresh(ctx); err != nil {
					slog.Warn("model refresh failed", "error", err)
				} else {
					slog.Info("models refreshed", "count", discoverer.Count())
				}
			}
		}
	}()

	// 5. Create rate limiter.
	limiter := ratelimit.NewLimiter(cfg.MaxRPS)

	// 6. Create generator.
	gen := generator.NewGenerator(modelList, cfg.TargetTPS, cfg.MaxRPS)

	// 7. Create client.
	cl := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.RequestTimeout)

	// 8. Create stats tracker, start it.
	tracker := stats.NewTracker(cfg.StatsInterval)
	tracker.Start(ctx)

	// 9. Main loop.
	iteration := 0
	slog.Info("traffic generator started")
	for {
		select {
		case <-ctx.Done():
			slog.Info("shutting down")
			// Brief pause to let in-flight requests and stats goroutine wind down.
			time.Sleep(500 * time.Millisecond)
			slog.Info("bye",
				"totalTokens", "see stats output",
				"iterations", iteration,
			)
			return
		default:
		}

		// Rate-limit before each request.
		if err := limiter.Wait(ctx); err != nil {
			// Context cancelled during wait.
			slog.Info("shutting down")
			time.Sleep(500 * time.Millisecond)
			return
		}

		req := gen.Next()
		tokens, latency, err := cl.ChatCompletion(ctx, req)
		if err != nil {
			slog.Warn("request failed", "model", req.Model, "error", err, "latency", latency)
			continue
		}

		tracker.Record(tokens, latency)

		iteration++
		if iteration%10 == 0 {
			gen.AdjustContext(tracker.TPS())
		}
	}
}
