package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	gocqlastra "github.com/datastax/gocql-astra"
	"github.com/gocql/gocql"
)

// AstraConfig holds the configuration for connecting to Astra DB.
type AstraConfig struct {
	Username string
	Path     string
	Token    string
}

// AstraMethods defines the methods for interacting with Astra DB.
type AstraMethods interface {
	Connect(ctx context.Context, cfg *AstraConfig, timeout time.Duration) (*gocql.Session, error)
}

// AstraDB represents a connection to Astra DB.
type AstraDB struct{}

// NewAstraDB initializes and returns an AstraDB instance that implements AstraMethods.
func NewAstraDB() AstraMethods {
	return &AstraDB{}
}

// Connect establishes a connection to Astra DB and returns a session.
func (db *AstraDB) Connect(ctx context.Context, cfg *AstraConfig, timeout time.Duration) (*gocql.Session, error) {
	// Create a new Astra DB cluster configuration using the provided credentials and timeout.
	cluster, err := gocqlastra.NewClusterFromBundle(cfg.Path, cfg.Username, cfg.Token, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create Astra DB cluster from bundle: %w", err)
	}

	// Open a new session using the cluster configuration.
	session, err := gocql.NewSession(*cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	slog.Info("Successfully connected to Astra DB")

	return session, nil
}
