package pee

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
)

// State is a marker interface for states of an Automata. Every state of your Automata needs to
// embed a State field. The State field needs to be tagged with a stable and unique (per Automata) `name` tag,
// that identifies the State within that Automata.
//
// The validity of a State is checked during construction time as well as after a StateTransition
// and the Automata will panic in case of an invalid State.
type State interface {
	// marker method
	__isState()
}

var nameCache sync.Map

// NameOf returns the name of the given State. If the State is invalid
// and does not have a name, this method will panic, as this is an error
// in the applications code.
//
// It is good practice to call NameOf for all your states in an init method
// to verify all your State instances are correctly defined.
func NameOf(st State) string {
	t := reflect.TypeOf(st)

	// lookup state name in cache first
	if name, ok := nameCache.Load(t); ok {
		return name.(string)
	}

	if t.Kind() != reflect.Struct {
		panic(makeErr("state must be a struct, got %s for %T", t.Kind(), st))
	}

	f, ok := t.FieldByName("State")
	if !ok {
		panic(fmt.Errorf("type %T is missing the 'State' field", st))
	}

	if !f.Anonymous {
		panic(fmt.Errorf("field 'State' on %T must be embedded", st))
	}

	name := f.Tag.Get("name")
	if name == "" {
		panic(fmt.Errorf("field 'State' on %T has no 'name' tag set or value is empty", st))
	}

	// put state name into cache
	nameCache.Store(t, name)

	return name
}

type envelopedState struct {
	Name string          `json:"state"`
	Data json.RawMessage `json:"data"`
}

// stateConstructor returns a deserializer function for a given State type.
// The type must be a struct, otherwise this method will panic.
func stateConstructor[S State]() func([]byte) (State, error) {
	var stateInstance S

	stateType := reflect.TypeOf(stateInstance)

	// must be a struct
	if stateType.Kind() != reflect.Struct {
		panic(makeErr("state must of type (pointer to) struct"))
	}

	return func(serializedState []byte) (State, error) {
		var state S
		err := json.Unmarshal(serializedState, &state)
		return state, wrap(err, "deserialize state %T", state)
	}
}

// serializeState serializes the state into a byte array.
func serializeState(state State) ([]byte, error) {
	name := NameOf(state)

	// serialize the state directly to json
	inner, err := json.Marshal(state)
	if err != nil {
		return nil, wrap(err, "serialize")
	}

	// then deserialize back into a map
	mapState, err := deserializeToMap(inner)
	if err != nil {
		return nil, err
	}

	// remove the 'State' field from the map
	delete(mapState, "State")

	// now serialize the map back to json again
	inner, err = json.Marshal(mapState)
	if err != nil {
		return nil, wrap(err, "serialize state")
	}

	// wrap into an envelope
	envelope := envelopedState{
		Name: name,
		Data: json.RawMessage(inner),
	}

	// and serialize together with the envelope
	return json.Marshal(envelope)
}

func deserializeToMap(inner []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(inner))
	dec.UseNumber()

	var asmap map[string]any
	if err := dec.Decode(&asmap); err != nil {
		return nil, wrap(err, "deserialize to map")
	}

	return asmap, nil
}
