package db

import (
	"context"
	"testing"

	"github.com/aperturerobotics/inca"
)

func TestDbPrefixer(t *testing.T) {
	ctx := context.Background()
	db := NewInmemDb()

	key := []byte("/key")
	prefix := []byte("/prefix1")
	prefix2 := []byte("/prefix2")

	dbp := WithPrefix(db, []byte(prefix))
	dbp = WithPrefix(dbp, []byte(prefix2))

	obj := &inca.Genesis{ChainId: "test"}
	if err := dbp.Set(ctx, key, obj); err != nil {
		t.Fatal(err.Error())
	}

	objb, err := dbp.Get(ctx, key)
	if err != nil {
		t.Fatal(err.Error())
	}

	if err := dbp.Set(ctx, key, obj); err != nil {
		t.Fatal(err.Error())
	}

	if objb != obj {
		t.Fatal("wrong object returned")
	}
}
