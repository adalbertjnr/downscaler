package store_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/adalbertjnr/downscaler/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setPostgresTestDBClient(t *testing.T, connString string) *sql.DB {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		t.Fatalf("failed to connect to postgres db: %v", err)
	}

	return db
}

func TestPostgresScalingOperationLifecycle(t *testing.T) {
	ctx := context.Background()

	const (
		postgresCredentials = "postgres"
		ctrImage            = "postgres:14.15-alpine3.20"
	)

	ctr, err := postgres.Run(ctx,
		ctrImage,
		postgres.WithDatabase(postgresCredentials),
		postgres.WithUsername(postgresCredentials),
		postgres.WithPassword(postgresCredentials),
		testcontainers.WithHostPortAccess(31555),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(1).
				WithStartupTimeout(5*time.Second),
			wait.ForExposedPort(),
		),
	)

	defer func() {
		if err := ctr.Terminate(ctx); err != nil {
			t.Log("error terminating the container: ", err)
		}
	}()

	if err != nil {
		t.Fatalf("unexpected error while initializing db container: %v", err)
	}

	conn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("error fetching container connection string: %v", err)
	}

	db := setPostgresTestDBClient(t, conn)
	if err := db.Ping(); err != nil {
		t.Fatalf("database ping fail: %v", err)
	}
	defer db.Close()

	p := store.NewPostgresScalingOperationStore(db)

	t.Run("Bootstrap", func(t *testing.T) {
		if err := p.Bootstrap(ctx); err != nil {
			t.Fatalf("bootstrap database scaling operation table should not return an error: %v", err)
		}
	})

	scalingObject := &store.ScalingOperation{
		NamespaceName:       "test-namespace",
		RuleNameDescription: "test-rule",
		ResourceName:        "test-name",
		ResourceType:        "test-deployment",
		Replicas:            5,
	}

	t.Run("Insert", func(t *testing.T) {
		firstID := 1

		if err := p.Insert(ctx, scalingObject); err != nil {
			t.Fatalf("insert postgres operation failed: %v", err)
		}
		assert.Equal(t, firstID, scalingObject.ID)
		assert.NotEmpty(t, scalingObject.CreatedAt)
	})

	updateObject := &store.ScalingOperation{
		NamespaceName:       "test-namespace",
		RuleNameDescription: "test-rule-updated",
		ResourceName:        "test-name",
		ResourceType:        "test-deployment",
		Replicas:            10,
	}

	t.Run("Update", func(t *testing.T) {
		if err := p.Update(ctx, updateObject); err != nil {
			t.Fatalf("update postgres operation failed: %v", err)
		}
	})

	getObject := &store.ScalingOperation{
		ResourceName:  "test-name",
		NamespaceName: "test-namespace",
	}

	t.Run("Get", func(t *testing.T) {
		if err := p.Get(ctx, getObject); err != nil {
			t.Fatalf("get updated object postgres error: %v", err)
		}
		assert.Equal(t, updateObject.NamespaceName, getObject.NamespaceName)
		assert.Equal(t, updateObject.ResourceName, getObject.ResourceName)
		assert.Equal(t, updateObject.RuleNameDescription, getObject.RuleNameDescription)
		assert.Equal(t, updateObject.ResourceType, getObject.ResourceType)
		assert.Equal(t, updateObject.UpdatedAt, getObject.UpdatedAt)
	})

}
