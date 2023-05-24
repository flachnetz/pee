package pee

import (
	"context"
)

// Action runs some code in a database transaction.
type Action[TxContext context.Context] func(ctx TxContext) error

// InfallibleAction runs some code in a database transaction
// but must not fail and can not produce an error.
type InfallibleAction[TxContext context.Context] func(ctx TxContext)

// StateTransition describes the transition to the next state.
// You can register multiple Action with a transition that are executed in the
// same database transaction that also updates the Automata.
// If any of the actions return an error, it will cancel the transaction
// (and in turn will cancel the transition). The error will be returned without wrapping.
type StateTransition[TxContext context.Context] struct {
	// set by one of the actions
	nextState State

	// actions to run.
	actions []Action[TxContext]

	// a state transition can only be run once and will
	// fail if it is run a second time.
	executed bool
}

// Transition initializes a new transition to the provided next State.
func Transition[TxContext context.Context](nextState State) *StateTransition[TxContext] {
	var tr StateTransition[TxContext]

	return tr.WithAction(func(ctx TxContext) error {
		tr.nextState = nextState
		return nil
	})
}

// TransitionInTx will execute the given code block within the update transaction
// to lazily compute the next state. This might be useful if the next State depends
// on some database value that needs to be read within the same transaction as the
// the update of the Automata.
func TransitionInTx[TxContext context.Context](block func(ctx TxContext) (State, error)) *StateTransition[TxContext] {
	var tr StateTransition[TxContext]

	return tr.WithAction(func(ctx TxContext) error {
		// run the code to get the next state
		state, err := block(ctx)
		if err != nil {
			return err
		}

		// code must return a state, if thats not the case, something went wrong.
		if state == nil {
			return ErrNoNextState
		}

		// need to remember this one for later
		tr.nextState = state
		return nil
	})
}

// WithAction adds another Action to this StateTransition.
// See Action for more details.
func (t *StateTransition[TxContext]) WithAction(action Action[TxContext]) *StateTransition[TxContext] {
	t.actions = append(t.actions, action)
	return t
}

// WithInfallibleAction adds an InfallibleAction to this StateTransition.
// See WithAction for more details.
func (t *StateTransition[TxContext]) WithInfallibleAction(action InfallibleAction[TxContext]) *StateTransition[TxContext] {
	return t.WithAction(
		func(ctx TxContext) error {
			action(ctx)
			return nil
		},
	)
}

// AsTuple provides some convenience when you need to return a StateTransition and an error.
// It is the same as returning `return self, nil`
func (t *StateTransition[TxContext]) AsTuple() (*StateTransition[TxContext], error) {
	return t, nil
}

// applyIn runs the state transition in the given Transaction. This method returns
// the new state if any.
func (t *StateTransition[TxContext]) applyIn(ctx TxContext) (State, error) {
	if t.executed {
		return nil, ErrTransitionReused
	}

	t.executed = true

	// apply the actions of this transition.
	for _, action := range t.actions {
		if err := action(ctx); err != nil {
			return nil, err
		}
	}

	// after applying the actions we can get the next state from the transition.
	return t.nextState, nil
}
