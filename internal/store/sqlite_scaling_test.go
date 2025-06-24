package store_test

import (
	"context"
	"database/sql"
	"testing"

<<<<<<< HEAD
	"github.com/adalbertjnr/downscaler/internal/store"
=======
	"github.com/adalbertjnr/kubetime-scaler/internal/store"
>>>>>>> main
	"github.com/stretchr/testify/assert"
)

func setSqliteTestDBClient(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to connect to in memory db: %v", err)
	}

	return db
}

func TestSqliteScalingOperationLifecycle(t *testing.T) {
	db := setSqliteTestDBClient(t)
	defer db.Close()

	p := store.NewSqliteScalingOperationStore(db)
	ctx := context.Background()

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
			t.Fatalf("insert sqlite operation failed: %v", err)
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
			t.Fatalf("update sqlite operation failed: %v", err)
		}
	})

	getObject := &store.ScalingOperation{
		ResourceName:  "test-name",
		NamespaceName: "test-namespace",
	}

	t.Run("Get", func(t *testing.T) {
		if err := p.Get(ctx, getObject); err != nil {
			t.Fatalf("get updated object sqlite error: %v", err)
		}
		assert.Equal(t, updateObject.NamespaceName, getObject.NamespaceName)
		assert.Equal(t, updateObject.ResourceName, getObject.ResourceName)
		assert.Equal(t, updateObject.RuleNameDescription, getObject.RuleNameDescription)
		assert.Equal(t, updateObject.ResourceType, getObject.ResourceType)
		assert.Equal(t, updateObject.UpdatedAt, getObject.UpdatedAt)
	})
}
