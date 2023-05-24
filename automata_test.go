package pee

import (
	"context"
	"github.com/onsi/ginkgo/v2/types"
	_ "modernc.org/sqlite"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRunSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Automata specs", types.ReporterConfig{Verbose: true})
}

var _ = Describe("Automata", func() {
	type StateA struct {
		State `name:"A"`
		Input string
	}

	type StateB struct {
		State `name:"B"`
		Value string
	}

	Context("when given a valid automata", func() {

		var a *Automata[context.Context, string]

		BeforeEach(func() {
			a = New[string](NewMemoryStore())

			AddState(a, func(ctx context.Context, state StateA) (*StateTransition[context.Context], error) {
				value := strings.ReplaceAll(state.Input, "input", "output")
				return a.NewTransition(StateB{Value: value}).AsTuple()
			})

			AddFinalState(a, func(ctx context.Context, state StateB) (string, error) {
				return state.Value + " (transformed)", nil
			})
		})

		ctx := context.Background()

		It("can run a simple instance with no errors", func() {
			instance, err := a.Start(ctx, StateA{Input: "my input value"})
			Expect(err).ToNot(HaveOccurred())

			res, err := a.Execute(ctx, DummyRunInTx, instance)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal("my output value (transformed)"))
		})

		It("can be run even if finished", func() {
			instance, err := a.Start(ctx, StateA{Input: "my input value"})
			Expect(err).ToNot(HaveOccurred())

			for i := 0; i < 3; i++ {
				instance, err := a.Load(ctx, instance.Id)
				Expect(err).ToNot(HaveOccurred())

				res, err := a.Execute(ctx, DummyRunInTx, instance)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal("my output value (transformed)"))
			}
		})

		It("correctly handles concurrent updates", func() {
			instance, err := a.Start(ctx, StateA{Input: "my input value"})
			Expect(err).ToNot(HaveOccurred())

			_, err = a.Execute(ctx, DummyRunInTx, instance)
			Expect(err).ToNot(HaveOccurred())

			_, err = a.Execute(ctx, DummyRunInTx, instance)
			Expect(err).To(And(HaveOccurred(), Equal(ErrOptimisticLocking)))
		})
	})

	It("fails for unknown states", func() {
		type UnknownState struct {
			State `name:"Unknown"`
		}

		a := New[any](NewMemoryStore())

		AddState(a, func(ctx context.Context, state StateA) (*StateTransition[context.Context], error) {
			return a.NewTransition(UnknownState{}), nil
		})

		instance, err := a.Start(context.Background(), StateA{})
		Expect(err).ToNot(HaveOccurred())

		_, err = a.Execute(context.Background(), DummyRunInTx, instance)
		Expect(err).To(HaveOccurred())
	})

	It("fails when registering a state twice", func() {
		a := New[any](NewMemoryStore())

		AddState(a, func(ctx context.Context, state StateA) (*StateTransition[context.Context], error) {
			return a.NewTransition(StateB{}), nil
		})

		registerStateTwice := func() {
			AddState(a, func(ctx context.Context, state StateA) (*StateTransition[context.Context], error) {
				return a.NewTransition(StateB{}), nil
			})
		}

		Expect(registerStateTwice).To(Panic())
	})
})
