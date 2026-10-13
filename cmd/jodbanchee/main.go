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

	"github.com/jodbanchee/jodbanchee/internal/bot"
	"github.com/jodbanchee/jodbanchee/internal/config"
	"github.com/jodbanchee/jodbanchee/internal/ledger"
	"github.com/jodbanchee/jodbanchee/internal/line"
	"github.com/jodbanchee/jodbanchee/internal/store/jsonfile"
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

	store, err := jsonfile.Open(cfg.DataFile)
	if err != nil {
		return err
	}

	svc := ledger.NewService(store, cfg.Location)
	b := bot.New(svc, log)
	client := line.NewClient(cfg.ChannelAccessToken, cfg.LineAPIBase)

	mux := http.NewServeMux()
	mux.Handle("POST /webhook", bot.NewWebhookHandler(cfg.ChannelSecret, b, client, log))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})

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
		log.Info("listening", "addr", srv.Addr, "data_file", cfg.DataFile, "timezone", cfg.Location.String())
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
