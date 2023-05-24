package pee

import "fmt"

var ErrNoNextState = makeErr("transition did neither fail nor return a next state")
var ErrTransitionReused = makeErr("transition must only run once and can not be reused")
var ErrNoSuchInstance = makeErr("no such instance")

type Error struct {
	error
}

func makeErr(message string, args ...interface{}) error {
	return Error{fmt.Errorf(message, args...)}
}

func wrap(err error, message string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	err = fmt.Errorf("%s: %w", fmt.Sprintf(message, args...), err)
	return Error{err}
}
