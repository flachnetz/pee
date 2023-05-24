package pee_sqlite

import (
	"fmt"
	"github.com/flachnetz/startup/v2/lib/ql"
	"pee"
	"pee/store/pee_pg"
)

type SqliteStore string

func (s SqliteStore) Update(ctx ql.TxContext, id, version int, newState []byte) (*pee.SerializedInstance, error) {
	stmt := "UPDATE " + string(s) + " SET state=$3, version=$2+1 WHERE id=$1 AND version=$2"
	affected, err := ql.ExecAffected(ctx, stmt, id, version, newState)

	if err != nil {
		return nil, fmt.Errorf("update automata %d@%d in database: %w", id, version, err)
	}

	if affected == 0 {
		return nil, pee.ErrOptimisticLocking
	}

	serializedInstance := &pee.SerializedInstance{
		Id:      id,
		Version: version + 1,
		State:   newState,
	}

	return serializedInstance, nil
}

func (s SqliteStore) Create(ctx ql.TxContext, state []byte) (*pee.SerializedInstance, error) {
	return pee_pg.PostgresStore(s).Create(ctx, state)
}

func (s SqliteStore) Load(ctx ql.TxContext, id int) (*pee.SerializedInstance, error) {
	return pee_pg.PostgresStore(s).Load(ctx, id)
}
