package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/glanceapp/glance/pkg/storage/postgres/ent"

	entsql "entgo.io/ent/dialect/sql"

	_ "github.com/jackc/pgx/v5/stdlib" // required for sql.Open to recognize pgx
)

type DB struct {
	cfg    *Config
	client *ent.Client
}

func NewDB(cfg *Config) *DB {
	return &DB{cfg: cfg}
}

func (d *DB) Client() *ent.Client {
	if d.client == nil {
		panic("db db not connected, call DB.Connect() first")
	}
	return d.client
}

// Connect connects to Postgres and optionally creates the schema.
func (d *DB) Connect(ctx context.Context) error {
	db, err := sql.Open("pgx", d.cfg.DSN())
	if err != nil {
		return fmt.Errorf("pgx connect to database: %w", err)
	}

	driver := entsql.OpenDB("postgres", db)
	client := ent.NewClient(ent.Driver(driver))

	// Optional schema creation for local/dev environments.
	if d.cfg.AutoMigrate {
		if err = client.Schema.Create(ctx); err != nil {
			return fmt.Errorf("create schema resources: %w", err)
		}
	}

	d.client = client

	return nil
}
