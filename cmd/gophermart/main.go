package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/JustTony97/gofermart/internal/clients"
	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/handler"
	"github.com/JustTony97/gofermart/internal/logger"
	"github.com/JustTony97/gofermart/internal/middlewares"
	"github.com/JustTony97/gofermart/internal/repository"
	"github.com/JustTony97/gofermart/internal/service"
	"github.com/JustTony97/gofermart/internal/worker"
	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

func main() {
	logger.Init()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := sqlx.Connect("pgx", cfg.DatabaseDSN)
	if err != nil {
		logger.Log.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := runMigrations(cfg.MigrationsPath, cfg.DatabaseDSN); err != nil {
		logger.Log.Error("Failed to run migrations", "error", err)
		os.Exit(1)
	}

	tx := repository.NewTxManager(db)
	userRepo := repository.NewUsersRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	txRepo := repository.NewTransactionRepository(db)

	userService := service.NewUserService(userRepo, &cfg)
	orderService := service.NewOrderService(orderRepo, &cfg, tx)
	txService := service.NewTransactionService(txRepo, tx)

	accrualClient := clients.NewAccrualServiceClient(cfg.AccrualBaseURL)

	r := initRouter(cfg, userService, orderService, txService)
	srv := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: r,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	startWorkers(ctx, &wg, &cfg, orderService, accrualClient)

	go func() {
		logger.Log.Info("Server started", "address", cfg.ServerAddress)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Error("HTTP server ListenAndServe error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Log.Info("Shutdown signal received, starting graceful shutdown...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("HTTP server Shutdown failed", "error", err)
	}

	wg.Wait()
	logger.Log.Info("Application stopped completely")
}

func initRouter(cfg config.Config, userService *service.UserService, orderService *service.OrderService, txService *service.TransactionService) http.Handler {
	userHandler := handler.NewUserHandler(userService, &cfg)
	ordersHandler := handler.NewOrderHandler(orderService, &cfg)
	balanceHandler := handler.NewBalanceHandler(txService, &cfg)
	authMiddleware := middlewares.NewAuthMiddleware(cfg)

	r := chi.NewRouter()
	r.Use(middlewares.WithLogging)

	r.Route("/api/user", func(r chi.Router) {
		r.Post("/register", userHandler.Register)
		r.Post("/login", userHandler.Login)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.RequireAuth)

			r.Post("/orders", ordersHandler.UploadOrder)
			r.Get("/orders", ordersHandler.GetOrders)

			r.Get("/balance", balanceHandler.GetBalance)
			r.Post("/balance/withdraw", balanceHandler.Withdraw)
			r.Get("/withdrawals", balanceHandler.GetWithdrawals)
		})
	})

	return r
}

func startWorkers(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config, orderService *service.OrderService, accrualClient *clients.AccrualServiceClient) {
	ordersPoller := worker.NewOrderPoller(cfg, orderService)
	for i := 0; i < cfg.OrderRegisterWorkers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			ordersPoller.StartPolling(ctx, i)
		}()
	}

	accrualPoller := worker.NewAccrualPoller(cfg, orderService, accrualClient)
	for i := 0; i < cfg.AccrualPollWorkers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			accrualPoller.StartPolling(ctx, i)
		}()
	}
}

func runMigrations(migrationsPath, databaseDSN string) error {
	m, err := migrate.New(migrationsPath, databaseDSN)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
