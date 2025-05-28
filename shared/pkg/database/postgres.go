package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

type Config struct {
	DbUri string
}

type DB struct {
	*sql.DB
	logger *logrus.Logger
}

func NewConnection(dbUri string, logger *logrus.Logger) (*DB, error) {
	db, err := sql.Open("postgres", dbUri)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(30 * time.Second)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Database connection established successfully")

	return &DB{
		DB:     db,
		logger: logger,
	}, nil
}

func (db *DB) Close() error {
	db.logger.Info("Closing database connection")
	return db.DB.Close()
}

func (db *DB) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return db.PingContext(ctx)
}
