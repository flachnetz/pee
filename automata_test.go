package pee

import (
	"context"
	"fmt"
	"github.com/flachnetz/startup/v2/lib/ql"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRunSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Automata specs")
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

	a := New[string](NewMemoryStore())

	AddState(a, func(ctx context.Context, state StateA) (*StateTransition[context.Context], error) {
		value := strings.ReplaceAll(state.Input, "input", "output")
		return a.NewTransition(StateB{Value: value}).AsTuple()
	})

	AddFinalState(a, func(ctx context.Context, state StateB) (string, error) {
		return state.Value + " (transformed)", nil
	})

	var db *sqlx.DB
	BeforeEach(func() {
		db = GetDatabase("my_table")
	})

	It("can run a simple instance with no errors", func() {
		instance, err := ql.InNewTransactionWithResult(context.TODO(), db, func(ctx ql.TxContext) (Instance, error) {
			return a.Start(ctx, StateA{Input: "my input value"})
		})

		Expect(err).ToNot(HaveOccurred())

		res, err := a.Execute(context.TODO(), DummyRunInTx, instance)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal("my output value (transformed)"))
	})

	It("can run be run even if finished", func() {
		instance, err := ql.InNewTransactionWithResult(context.TODO(), db, func(ctx ql.TxContext) (Instance, error) {
			return a.Start(ctx, StateA{Input: "my input value"})
		})

		Expect(err).ToNot(HaveOccurred())

		for i := 0; i < 3; i++ {
			instance, err := ql.InNewTransactionWithResult(context.Background(), db, func(ctx ql.TxContext) (Instance, error) {
				return a.Load(ctx, instance.Id)
			})

			Expect(err).ToNot(HaveOccurred())

			res, err := a.Execute(context.TODO(), DummyRunInTx, instance)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal("my output value (transformed)"))
		}
	})
})

func GetDatabase(table string) *sqlx.DB {
	db := sqlx.MustOpen("sqlite", ":memory:")
	DeferCleanup(db.Close)

	db.MustExec(fmt.Sprintf(`
			CREATE TABLE %s (
				"id"         integer  NOT NULL PRIMARY KEY,
				"version"    integer  NOT NULL,
				"state"      JSON   NOT NULL,
				"log"        JSON   NOT NULL DEFAULT '[]'
			)
		`, table))

	return db
}
