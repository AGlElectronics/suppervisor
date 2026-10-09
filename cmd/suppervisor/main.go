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

	"github.com/AGlElectronics/suppervisor/internal/app"
	"github.com/AGlElectronics/suppervisor/internal/config"
	"github.com/AGlElectronics/suppervisor/internal/domain"
	"github.com/AGlElectronics/suppervisor/internal/httpapi"
	"github.com/AGlElectronics/suppervisor/internal/llm"
	"github.com/AGlElectronics/suppervisor/internal/store"
	"github.com/AGlElectronics/suppervisor/internal/telegram"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/sync/errgroup"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("suppervisor stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		client := &http.Client{Timeout: 2 * time.Second}
		response, err := client.Get("http://127.0.0.1:8091/health/ready")
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return errors.New("health endpoint is not ready")
		}
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := store.Open(rootCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(rootCtx); err != nil {
		return err
	}
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		logger.Info("migrations complete")
		return nil
	}

	var provider llm.Provider
	if cfg.OllamaURL != "" {
		provider = llm.Ollama{BaseURL: cfg.OllamaURL, Model: cfg.OllamaModel}
	}
	service := app.Service{Store: db, Generator: llm.Generator{Provider: provider}, Planner: domain.Planner{}}
	var verify httpapi.VerifyToken
	if !cfg.AuthDevBypass {
		oidcProvider, err := oidc.NewProvider(rootCtx, cfg.OIDCIssuer)
		if err != nil {
			return err
		}
		verifier := oidcProvider.Verifier(&oidc.Config{ClientID: cfg.OIDCClientID})
		verify = func(ctx context.Context, raw string) error {
			_, err := verifier.Verify(ctx, raw)
			return err
		}
	}
	api := httpapi.Server{Service: service, Store: db, Config: cfg, VerifyToken: verify, Logger: logger}
	rootMux := http.NewServeMux()
	rootMux.Handle("/", api.Handler())

	bot := telegram.Bot{Config: cfg, Service: service, Store: db, Logger: logger}
	if cfg.TelegramToken != "" && cfg.TelegramMode == "webhook" {
		rootMux.Handle("/telegram/webhook", bot.WebhookHandler())
		if err := bot.ConfigureWebhook(rootCtx); err != nil {
			return err
		}
	}
	server := &http.Server{
		Addr: cfg.Addr, Handler: rootMux, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 95 * time.Second, IdleTimeout: 60 * time.Second,
	}
	group, ctx := errgroup.WithContext(rootCtx)
	group.Go(func() error {
		logger.Info("Suppervisor is serving", "addr", cfg.Addr, "environment", cfg.Environment)
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})
	if cfg.TelegramToken != "" && cfg.TelegramMode == "polling" {
		group.Go(func() error {
			err := bot.RunPolling(ctx)
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		})
	}
	group.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	})
	return group.Wait()
}
