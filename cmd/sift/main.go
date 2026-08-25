package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/oauth2"

	"github.com/prashantkoirala465/sift/internal/api"
	"github.com/prashantkoirala465/sift/internal/classify"
	"github.com/prashantkoirala465/sift/internal/config"
	"github.com/prashantkoirala465/sift/internal/gmail"
	"github.com/prashantkoirala465/sift/internal/match"
	"github.com/prashantkoirala465/sift/internal/storage/postgres"
	"github.com/prashantkoirala465/sift/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("applying migrations")
	if err := postgres.Migrate(cfg.DatabaseURL); err != nil {
		return err
	}

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	store := postgres.NewStore(pool, cfg.EncryptionKey)

	var oauthCfg *oauth2.Config
	if cfg.GoogleConfigured() {
		oauthCfg = gmail.NewOAuthConfig(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
	} else {
		logger.Warn("Google OAuth not configured, /auth/google routes will 503 until SIFT_GOOGLE_CLIENT_ID/SECRET/REDIRECT_URL are set")
	}

	var llmClassifier classify.Classifier
	if cfg.AnthropicAPIKey != "" {
		llmClassifier = classify.NewLLMClassifier(cfg.AnthropicAPIKey)
	} else {
		logger.Warn("SIFT_ANTHROPIC_API_KEY not set, classifying with rules only")
	}
	classifier := classify.NewTieredClassifier(llmClassifier, logger)
	matcher := match.NewMatcher(store, logger)

	syncWorker := worker.NewSyncWorker(store, oauthCfg, classifier, matcher, logger)
	go syncWorker.Run(ctx, cfg.SyncInterval)

	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: api.NewRouter(api.Deps{
			OAuthConfig: oauthCfg,
			TokenStore:  store,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("shutting down")
	return srv.Shutdown(shutdownCtx)
}
