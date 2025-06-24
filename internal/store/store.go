package store

import (
	"strings"

<<<<<<< HEAD
	"github.com/adalbertjnr/downscaler/internal/db"
=======
	"github.com/adalbertjnr/kubetime-scaler/internal/db"
>>>>>>> main
	"github.com/go-logr/logr"
)

const (
	sqliteDriver   = "sqlite"
	postgresDriver = "postgres"

	sqlitePersistencePath = "/home/nonroot/data_db"
)

type Persistence struct {
	ScalingOperation ScalingOperationStorer
}

func New(log logr.Logger, enableDatabase bool, c db.Config) *Persistence {
	if !enableDatabase {
		return nil
	}

	switch strings.ToLower(c.Driver) {
	case sqliteDriver:
		dbClient := db.MustCreateClient(sqliteDriver, db.Config{
			Driver: c.Driver,
			DSN:    sqlitePersistencePath,
		})

		log.Info("database", "initializing db client with", sqliteDriver, "ensure to persist the path", sqlitePersistencePath)
		return &Persistence{ScalingOperation: NewSqliteScalingOperationStore(dbClient)}

	case postgresDriver:
		dbClient := db.MustCreateClient(postgresDriver, db.Config{
			Driver: c.Driver,
			DSN:    c.DSN,
		})

		log.Info("database", "initializing db client with", postgresDriver)
		return &Persistence{ScalingOperation: NewPostgresScalingOperationStore(dbClient)}

	default:
		log.Info("database", "database flag is set to true but none of sqlite or postgres driver were configured", c.Driver, "fallback to", "memory_store")
		return nil
	}
}
