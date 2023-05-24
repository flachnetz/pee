package pee

import (
	"fmt"
)

// Instance represents the state of an instance of an Automata. The instance is
// immutable. Furthermore it is versioned to handle optimistic locking.
type Instance struct {
	Id      int
	Version int
	State   State
}

func (i Instance) String() string {
	return fmt.Sprintf("Instance(id=%d, version=%d)", i.Id, i.Version)
}
