package pee

import (
	"context"
	"encoding/json"
	"fmt"
)

type Handler[TxContext context.Context, S State] func(ctx TxContext, state S) (*StateTransition[TxContext], error)

type Transform[S State, T any] func(ctx context.Context, state S) (T, error)

type RunInTx[TxContext context.Context, R any] func(ctx context.Context, fn func(ctx TxContext) (R, error)) (R, error)

type Automata[TxContext context.Context, R any] struct {
	store             Store[TxContext]
	states            map[string]Handler[TxContext, State]
	finalStates       map[string]Transform[State, R]
	stateConstructors map[string]func([]byte) (State, error)
}

func (a *Automata[TxContext, T]) validateState(state State) error {
	// verify that the state is known
	name := NameOf(state)

	if _, ok := a.stateConstructors[name]; !ok {
		return fmt.Errorf("unknown state %q", state)
	}

	return nil
}

// Start creates a new Instance of an Automata with the given initial State in the database.
func (a *Automata[TxContext, _]) Start(ctx TxContext, initialState State) (Instance, error) {
	serializedState, err := serializeState(initialState)
	if err != nil {
		return Instance{}, fmt.Errorf("serialize state: %w", err)
	}

	serializedInstance, err := a.store.Create(ctx, serializedState)
	if err != nil {
		return Instance{}, err
	}

	instance := Instance{
		Id:      serializedInstance.Id,
		Version: serializedInstance.Version,
		State:   initialState,
	}

	return instance, nil
}

// Load gets an Instance of this Automata with the given id from the database.
func (a *Automata[TxContext, _]) Load(ctx TxContext, id int) (Instance, error) {
	serializedInstance, err := a.store.Load(ctx, id)
	if err != nil {
		return Instance{}, err
	}

	state, err := a.deserializeState(serializedInstance.State)
	if err != nil {
		return Instance{}, fmt.Errorf("deserialize state: %w", err)
	}

	instance := Instance{
		Id:      serializedInstance.Id,
		Version: serializedInstance.Version,
		State:   state,
	}

	return instance, nil
}

// New creates a new Automata that lives in the given database table.
// The table needs to already exist.
func New[R any, TxContext context.Context](store Store[TxContext]) *Automata[TxContext, R] {
	return &Automata[TxContext, R]{
		store:             store,
		states:            map[string]Handler[TxContext, State]{},
		finalStates:       map[string]Transform[State, R]{},
		stateConstructors: map[string]func([]byte) (State, error){},
	}
}

func (a *Automata[TxContext, R]) Execute(ctx TxContext, runInTx RunInTx[TxContext, Instance], instance Instance) (R, error) {
	var nilT R

	for {
		name := NameOf(instance.State)

		// check if we have reached the final state
		if final, ok := a.finalStates[name]; ok {
			return final(ctx, instance.State)
		}

		// check that we have a state handler
		handler, ok := a.states[name]
		if !ok {
			return nilT, fmt.Errorf("no event handler for state %q", name)
		}

		// execute the handler to get a transition
		transition, err := handler(ctx, instance.State)
		if err != nil {
			return nilT, err
		}

		// run a transaction to execute the state update
		newInstance, err := a.applyTransition(ctx, runInTx, instance, transition)
		if err != nil {
			return nilT, err
		}

		// use the new instance from now on
		instance = newInstance
	}
}

func (a *Automata[TxContext, R]) applyTransition(ctx TxContext, runInTx RunInTx[TxContext, Instance], instance Instance, transition *StateTransition[TxContext]) (Instance, error) {
	return runInTx(ctx, func(ctx TxContext) (Instance, error) {
		// apply transition to get the next state
		nextState, err := transition.applyIn(ctx)
		if err != nil {
			return Instance{}, err
		}

		// update the instance
		return a.updateInstance(ctx, instance, nextState)
	})
}

func (a *Automata[TxContext, _]) updateInstance(ctx TxContext, instance Instance, newState State) (Instance, error) {
	// serialize the new state
	serializedState, err := serializeState(newState)
	if err != nil {
		return Instance{}, fmt.Errorf("serialize state in transition: %w", err)
	}

	serializedInstance, err := a.store.Update(ctx, instance.Id, instance.Version, serializedState)
	if err != nil {
		return Instance{}, err
	}

	newInstance := Instance{
		Id:      serializedInstance.Id,
		Version: serializedInstance.Version,
		State:   newState,
	}

	return newInstance, nil
}

func (a *Automata[TxContext, _]) deserializeState(serializedState []byte) (State, error) {
	var envelope envelopedState

	// unmarshal the envelope to get to the states type
	if err := json.Unmarshal(serializedState, &envelope); err != nil {
		return nil, err
	}

	// get the constructor for this state
	stateConstructor, ok := a.stateConstructors[envelope.Name]
	if !ok {
		return nil, makeErr("unknown state %q", envelope.Name)
	}

	// and unmarshal the actual state
	state, err := stateConstructor(envelope.Data)
	if err != nil {
		return nil, wrap(err, "deserialize state %q", envelope.Name)
	}

	return state, nil
}

func (a *Automata[TxContext, R]) NewTransition(newState State) *StateTransition[TxContext] {
	return Transition[TxContext](newState)
}

// AddState adds a new Handler to the Automata. The Handler is called whenever
// an Instance of this Automata is in the given State. The handlers state argument
// must be a struct of type State.
// Every state can only be registered once, otherwise this method will panic.
func AddState[S State, R any, TxContext context.Context](a *Automata[TxContext, R], handler Handler[TxContext, S]) {
	addStateInternal[S](a, a.states, func(ctx TxContext, state State) (*StateTransition[TxContext], error) {
		return handler(ctx, state.(S))
	})
}

// AddFinalState adds a new Transform to the Automata. The Transform will be called
// whenever the Automata is in the given State. This indicates a final state for the
// Automata.
//
// The result of Transform is then returned by the Automata.Execute method.
func AddFinalState[S State, R any, TxContext context.Context](a *Automata[TxContext, R], transform Transform[S, R]) {
	addStateInternal[S](a, a.finalStates, func(ctx context.Context, state State) (R, error) {
		return transform(ctx, state.(S))
	})
}

func addStateInternal[S State, TxContext context.Context, R any, F any](a *Automata[TxContext, R], target map[string]F, fn F) {
	var stateInstance S
	name := NameOf(stateInstance)

	// state must not be registered yet
	if _, found := a.stateConstructors[name]; found {
		panic(makeErr("state %q already registered", name))
	}

	// register state
	a.stateConstructors[name] = stateConstructor[S]()
	target[name] = fn
}

func DummyRunInTx(ctx context.Context, fn func(ctx context.Context) (Instance, error)) (Instance, error) {
	return fn(ctx)
}
