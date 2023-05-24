package pee

import (
	"context"
)

var ErrOptimisticLocking = makeErr("optimistic locking failed")

type SerializedInstance struct {
	Id      int
	Version int
	State   []byte
}

type Store[TxContext context.Context] interface {
	// Update needs to update the state of the Instance identified by the given id and version.
	// Implementations should use optimistic locking and only update the instance,
	// if the version matches. The implementation needs to return the new version of the entity.
	// If optimistic locking fails this method should return ErrOptimisticLocking
	Update(ctx TxContext, id, version int, newState []byte) (*SerializedInstance, error)

	// Create needs to store create a new entity for the given serialized state.
	// It needs to return the created SerializedInstance.
	Create(ctx TxContext, state []byte) (*SerializedInstance, error)

	// Load needs to load the state of the Instance identified by the given id
	Load(ctx TxContext, id int) (*SerializedInstance, error)
}
