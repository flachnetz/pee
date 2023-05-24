package pee

import (
	"context"
)

type MemoryStore struct {
	instances map[int]SerializedInstance
}

var _ Store[context.Context] = MemoryStore{}

func (m MemoryStore) Update(ctx context.Context, id, version int, newState []byte) (*SerializedInstance, error) {
	instance, ok := m.instances[id]
	if !ok {
		return nil, ErrNoSuchInstance
	}

	if instance.Version != version {
		return nil, ErrOptimisticLocking
	}

	instance.Version = version + 1
	instance.State = newState

	m.instances[id] = instance

	return &instance, nil
}

func (m MemoryStore) Create(ctx context.Context, state []byte) (*SerializedInstance, error) {
	id := len(m.instances) + 1

	instance := SerializedInstance{
		Id:      id,
		Version: 1,
		State:   state,
	}

	m.instances[id] = instance

	return &instance, nil
}

func (m MemoryStore) Load(ctx context.Context, id int) (*SerializedInstance, error) {
	instance, ok := m.instances[id]
	if !ok {
		return nil, ErrNoSuchInstance
	}

	return &instance, nil
}

func NewMemoryStore() Store[context.Context] {
	return MemoryStore{
		instances: map[int]SerializedInstance{},
	}
}
