// Package main boots the Product Update Service Simulator HTTP server.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fairyhunter13/product-update-service-simulator/internal/config"
	httpapi "github.com/fairyhunter13/product-update-service-simulator/internal/http"
	"github.com/fairyhunter13/product-update-service-simulator/internal/obs"
	"github.com/fairyhunter13/product-update-service-simulator/internal/queue"
	"github.com/fairyhunter13/product-update-service-simulator/internal/store"
)

func main() {
	cfg := config.Load()
	obs.InitLogger()
	obs.Logger.Info("service_starting")

	st := store.New()
	q := queue.New(128)
	mgr := queue.NewManager(cfg, q, st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Start(ctx)

	app := httpapi.NewApp(cfg, st, mgr)
	mux := httpapi.NewRouter(app)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		obs.Logger.Info("http_listen", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			obs.Logger.Error("http_server_error", "error", err)
			os.Exit(1)
		}
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	s := <-sigc
	obs.Logger.Info("shutdown_signal", "signal", s.String())

	app.StartShutdown()
	obs.Logger.Info("shutdown_drain_begin", "backlog_size", mgr.BacklogSize(), "worker_count", mgr.WorkerCount())

	ctxDrain, cancelDrain := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelDrain()
	if drained := mgr.DrainUntil(ctxDrain); !drained {
		obs.Logger.Warn("shutdown_drain_timeout")
	} else {
		obs.Logger.Info("shutdown_drain_complete")
	}

	ctxSrv, cancelSrv := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelSrv()
	if err := srv.Shutdown(ctxSrv); err != nil {
		obs.Logger.Error("http_shutdown_error", "error", err)
	}
	mgr.Stop()
	obs.Logger.Info("service_stopped")
}
