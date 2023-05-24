package pee_pg

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/flachnetz/startup/v2/lib/ql"
	"pee"
)

type PostgresStore string

func (s PostgresStore) Update(ctx ql.TxContext, id, version int, newState []byte) (*pee.SerializedInstance, error) {
	stmt := fmt.Sprintf(`UPDATE %q SET "log"=("log"::jsonb || "state"::jsonb), "state"=$3, "version"=$2+1 WHERE "id"=$1 AND "version"=$2`, string(s))
	affected, err := ql.ExecAffected(ctx, stmt, id, version, newState)

	if err != nil {
		return nil, fmt.Errorf("update automat %d@%d in database: %w", id, version, err)
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

func (s PostgresStore) Create(ctx ql.TxContext, state []byte) (*pee.SerializedInstance, error) {
	stmt := fmt.Sprintf(`INSERT INTO %q ("version", "state") VALUES (1, $1) RETURNING id`, string(s))

	id, err := ql.Get[int](ctx, stmt, state)
	if err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}

	serializedInstance := &pee.SerializedInstance{
		Id:      *id,
		Version: 1,
		State:   state,
	}

	return serializedInstance, nil
}

func (s PostgresStore) Load(ctx ql.TxContext, id int) (*pee.SerializedInstance, error) {
	query := fmt.Sprintf(`SELECT "id", "version", "state" FROM %q WHERE "id"=$1`, string(s))

	type dbInstance struct {
		Id      int    `db:"id"`
		Version int    `db:"version"`
		State   []byte `db:"state"`
	}

	row, err := ql.Get[dbInstance](ctx, query, id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, fmt.Errorf("loading instance id=%d: %w", id, pee.ErrNoSuchInstance)

	case err != nil:
		return nil, fmt.Errorf("loading automat: %w", err)
	}

	instance := &pee.SerializedInstance{
		Id:      row.Id,
		Version: row.Version,
		State:   row.State,
	}

	return instance, nil
}
