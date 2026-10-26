package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/juanjo850/device-mock/internal/config"
	"github.com/juanjo850/device-mock/internal/controlapi"
	"github.com/juanjo850/device-mock/internal/device"
	"github.com/juanjo850/device-mock/internal/server"
)

func main() {
	configPath := flag.String("config", "configs/devices.json", "path to JSON config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Warn("config load failed, using default", "path", *configPath, "err", err)
		cfg = config.Default()
	}

	reg := device.NewRegistry()
	for _, dc := range cfg.Devices {
		reg.Add(device.FromConfig(dc))
	}

	srv := server.New(reg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv.StartAll(ctx, reg.List())

	var reloadMu sync.Mutex
	reload := func() error {
		reloadMu.Lock()
		defer reloadMu.Unlock()
		newCfg, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		return srv.Reload(ctx, newCfg)
	}

	api := controlapi.New(reg, srv, cfg.ControlToken, reload)
	httpSrv := &http.Server{Addr: cfg.ControlAddr, Handler: api.Handler()}
	go func() {
		slog.Info("control api listening", "addr", cfg.ControlAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("control api error", "err", err)
		}
	}()

	slog.Info("device-mock started", "devices", len(reg.List()))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	for {
		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGHUP:
				slog.Info("SIGHUP received, reloading config")
				if err := reload(); err != nil {
					slog.Error("reload failed", "err", err)
				} else {
					slog.Info("reload complete")
				}
			default:
				slog.Info("shutting down", "signal", sig)
				cancel()
				srv.Stop()
				_ = httpSrv.Shutdown(context.Background())
				fmt.Println("bye")
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
