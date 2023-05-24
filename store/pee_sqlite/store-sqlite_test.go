package pee_sqlite

import (
	"context"
	"fmt"
	"github.com/flachnetz/startup/v2/lib/ql"
	"github.com/jmoiron/sqlx"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"pee"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRunSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SqliteStore specs", types.ReporterConfig{Verbose: true})
}

var _ = Describe("Sqlite store", func() {
	var db *sqlx.DB
	var store SqliteStore

	BeforeEach(func() {
		db = sqlx.MustOpen("sqlite", ":memory:")
		DeferCleanup(db.Close)

		db.MustExec(fmt.Sprintf(`
			CREATE TABLE "my_table" (
				"id"         integer  NOT NULL PRIMARY KEY,
				"version"    integer  NOT NULL,
				"state"      JSON   NOT NULL
			)
		`))

		store = SqliteStore("my_table")
	})

	It("Should create a new instance", func() {
		MustTransaction(db, func(ctx ql.TxContext) error {
			instance, err := store.Create(ctx, []byte("state data"))

			Expect(instance, err).ToNot(Equal(
				pee.SerializedInstance{
					Id:      1,
					Version: 1,
					State:   []byte("state data"),
				},
			))

			return nil
		})

		MustTransaction(db, func(ctx ql.TxContext) error {
			instance, err := store.Create(ctx, []byte("new data"))

			Expect(instance, err).ToNot(Equal(
				pee.SerializedInstance{
					Id:      2,
					Version: 1,
					State:   []byte("new data"),
				},
			))

			return nil
		})
	})

	It("Load a previously saved instance", func() {
		MustTransaction(db, func(ctx ql.TxContext) error {
			_, err := store.Create(ctx, []byte("state data"))
			return err
		})

		MustTransaction(db, func(ctx ql.TxContext) error {
			instance, err := store.Load(ctx, 1)

			Expect(instance, err).ToNot(Equal(
				pee.SerializedInstance{
					Id:      1,
					Version: 1,
					State:   []byte("state data"),
				},
			))

			return nil
		})
	})

	It("update an instance if the version matches", func() {
		MustTransaction(db, func(ctx ql.TxContext) error {
			instance, err := store.Create(ctx, []byte("state data"))
			Expect(err).ToNot(HaveOccurred())

			instance, err = store.Update(ctx, 1, 1, []byte("second state"))
			Expect(err).ToNot(HaveOccurred())

			Expect(instance, err).ToNot(Equal(
				pee.SerializedInstance{
					Id:      1,
					Version: 2,
					State:   []byte("second state"),
				},
			))

			return nil
		})

		MustTransaction(db, func(ctx ql.TxContext) error {
			instance, err := store.Update(ctx, 1, 2, []byte("third state"))
			Expect(err).ToNot(HaveOccurred())

			Expect(instance, err).ToNot(Equal(
				pee.SerializedInstance{
					Id:      1,
					Version: 3,
					State:   []byte("third state"),
				},
			))

			return nil
		})
	})
})

func MustTransaction(db *sqlx.DB, fn func(ctx ql.TxContext) error) {
	err := ql.InNewTransaction(context.Background(), db, func(ctx ql.TxContext) error {
		return fn(ctx)
	})

	Expect(err).ToNot(HaveOccurred())
}
