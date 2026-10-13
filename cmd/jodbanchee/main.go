// Command jodbanchee runs the LINE webhook server.
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
	_ "time/tzdata" // embed time zones so the binary works in minimal containers

	"github.com/Jppnp/jodbanchee-bot-group/internal/bot"
	"github.com/Jppnp/jodbanchee-bot-group/internal/config"
	"github.com/Jppnp/jodbanchee-bot-group/internal/ledger"
	"github.com/Jppnp/jodbanchee-bot-group/internal/line"
	"github.com/Jppnp/jodbanchee-bot-group/internal/store/jsonfile"
	"github.com/Jppnp/jodbanchee-bot-group/internal/store/postgres"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	store, storeName, closeStore, err := openStore(cfg)
	if err != nil {
		return err
	}
	defer closeStore()

	svc := ledger.NewService(store, cfg.Location)
	b := bot.New(svc, log)
	client := line.NewClient(cfg.ChannelAccessToken, cfg.LineAPIBase)

	health := func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) }
	mux := http.NewServeMux()
	mux.Handle("POST /webhook", bot.NewWebhookHandler(cfg.ChannelSecret, b, client, log))
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /healthz", health) // Cloud Run reserves /healthz; use /health there

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr, "store", storeName, "timezone", cfg.Location.String())
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}

// openStore picks Postgres when DATABASE_URL is set, otherwise the JSON file.
func openStore(cfg config.Config) (store ledger.Store, name string, closeFn func(), err error) {
	if cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pg, err := postgres.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, "", nil, err
		}
		return pg, "postgres", func() { pg.Close() }, nil
	}
	fs, err := jsonfile.Open(cfg.DataFile)
	if err != nil {
		return nil, "", nil, err
	}
	return fs, "jsonfile:" + cfg.DataFile, func() {}, nil
}
