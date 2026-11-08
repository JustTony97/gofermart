package config

import (
	"flag"
	"fmt"
	"time"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	ServerAddress         string        `env:"SERVER_ADDRESS"`
	DatabaseDSN           string        `env:"DATABASE_DSN"`
	MigrationsPath        string        `env:"MIGRATIONS_PATH"`
	JWTSecret             string        `env:"JWT_SECRET"`
	JWTExp                time.Duration `env:"JWT_EXP"`
	OrderRegisterInterval time.Duration `env:"ORDER_REGISTER_INTERVAL"`
	OrderRegisterWorkers  int           `env:"ORDER_REGISTER_WORKERS"`
	AccrualPollInterval   time.Duration `env:"ACCRUAL_POLL_INTERVAL"`
	AccrualPollWorkers    int           `env:"ACCRUAL_POLL_WORKERS"`
	AccrualBaseURL        string        `env:"ACCRUAL_BASE_URL"`
}

func Load() (Config, error) {
	var cfg Config

	flag.StringVar(&cfg.ServerAddress, "a", ":8000", "Server address")
	flag.StringVar(&cfg.DatabaseDSN, "d", "postgresql://postgres:password@localhost:5432/postgres?sslmode=disable", "Database dsn")
	flag.StringVar(&cfg.MigrationsPath, "m", "file://./migrations", "Migrations path")
	flag.StringVar(&cfg.JWTSecret, "jwt-secret", "supersecretkey", "JWT Secret key")
	flag.DurationVar(&cfg.JWTExp, "jwt-exp", 3*time.Hour, "JWT Token expiration duration")
	flag.DurationVar(&cfg.OrderRegisterInterval, "r", 1*time.Second, "New orders registration interval")
	flag.IntVar(&cfg.OrderRegisterWorkers, "rw", 3, "Order registration workers count")
	flag.DurationVar(&cfg.AccrualPollInterval, "p", 5*time.Second, "Accrual status poll interval")
	flag.IntVar(&cfg.AccrualPollWorkers, "pw", 3, "Accrual poll workers count")
	flag.StringVar(&cfg.AccrualBaseURL, "ac", "http://localhost:8080", "Accrual base url")

	flag.Parse()

	if err := env.Parse(&cfg); err != nil {
		return cfg, fmt.Errorf("parse env config: %w", err)
	}

	return cfg, nil
}
