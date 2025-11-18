// Package app configures and runs application.
package app

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/evrone/go-clean-template/config"
	http "github.com/evrone/go-clean-template/internal/controller/http"
	pgrepo "github.com/evrone/go-clean-template/internal/repo/postgres"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/httpserver"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/evrone/go-clean-template/pkg/postgres"
)

func Run(cfg *config.Config) {
	l := logger.New(cfg.Log.Level)

	pg, err := postgres.New(cfg.PG.URL, postgres.MaxPoolSize(cfg.PG.PoolMax))
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - postgres.New: %w", err))
	}
	defer pg.Close()

	pgRepo, err := pgrepo.NewWithPool(pg.Pool)
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - postgres.NewWithPool: %w", err))
	}

	userRepo := pgRepo.UserRepo()
	teamRepo := pgRepo.TeamRepo()
	prRepo := pgRepo.PRRepo()

	// Usecase
	prUC := usecase.NewPRUseCase(prRepo, userRepo, teamRepo)

	// HTTP Server
	httpServer := httpserver.New(l, httpserver.Port(cfg.HTTP.Port), httpserver.Prefork(cfg.HTTP.UsePreforkMode))

	// Register routes
	http.NewRouter(httpServer.App, cfg, prUC, userRepo, teamRepo, prRepo, l)

	httpServer.Start()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	select {
	case <-interrupt:
		l.Info("app - Run - signal received")
	case err := <-httpServer.Notify():
		l.Error(fmt.Errorf("app - Run - httpServer.Notify: %w", err))
	}

	if err := httpServer.Shutdown(); err != nil {
		l.Error(fmt.Errorf("app - Run - httpServer.Shutdown: %w", err))
	}
}
