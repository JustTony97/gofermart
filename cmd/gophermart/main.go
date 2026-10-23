package main

import (
	"log"
	"net/http"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/handler"
	"github.com/JustTony97/gofermart/internal/logger"
	"github.com/JustTony97/gofermart/internal/middlewares"
	"github.com/JustTony97/gofermart/internal/repository"
	"github.com/JustTony97/gofermart/internal/service"
	"github.com/go-chi/chi/v5"
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
		logger.Log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUsersRepository(db)
	userService := service.NewUserService(userRepo, cfg)

	userHandler := handler.NewUserHandler(userService, cfg)
	ordersHandler := handler.OrdersHandler{}
	balanceHandler := handler.BalanceHandler{}

	r := chi.NewRouter()
	r.Use(middlewares.WithLogging)

	r.Route("/api/user", func(r chi.Router) {
		r.Post("/register", userHandler.Register)
		r.Post("/login", userHandler.Login)

		r.Post("/orders", ordersHandler.UploadOrder)
		r.Get("/orders", ordersHandler.GetOrders)

		r.Get("/balance", balanceHandler.GetBalance)
		r.Post("/balance/withdraw", balanceHandler.Withdraw)
		r.Get("/withdrawals", balanceHandler.GetWithdrawals)
	})

	log.Printf("Server started on %s", cfg.ServerAddress)
	log.Fatal(http.ListenAndServe(cfg.ServerAddress, r))
}
