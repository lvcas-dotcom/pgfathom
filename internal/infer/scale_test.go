package infer_test

import (
	"fmt"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/infer"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/profile"
)

// Generation is the one stage whose cost is paid entirely in this process: the
// catalog read is one round trip, and validation is one query per surviving
// candidate. So it is the stage that decides whether this tool is usable on a
// schema with thousands of tables, and the only way to know is to build one.
//
// The single-column pass is a hash lookup per column and scales linearly. The
// composite pass compares every table against every table carrying a composite
// key, which is quadratic in the worst case — these benchmarks are what keeps
// that honest.
func benchSchema(tables, columnsPer int, compositeEvery int) []model.Schema {
	s := model.Schema{Name: "public", Tables: make([]model.Table, 0, tables)}

	for i := range tables {
		t := model.Table{
			Schema:  "public",
			Name:    fmt.Sprintf("entity_%d", i),
			Columns: make([]model.Column, 0, columnsPer),
			Stats:   model.TableStats{EstimatedRows: 10_000},
		}

		t.Columns = append(t.Columns, model.Column{
			Name: "id", Type: "bigint", BaseType: "int8", Position: 1,
		})
		t.PrimaryKey = []string{"id"}

		// Every nth table is keyed on a pair, which is what gives the composite
		// pass something to work against.
		if compositeEvery > 0 && i%compositeEvery == 0 {
			t.Columns = append(t.Columns, model.Column{
				Name: "seq", Type: "integer", BaseType: "int4", Position: 2,
			})
			t.PrimaryKey = []string{"id", "seq"}
		}

		// The rest are references to other tables by name, which is what the
		// generator is meant to find.
		for c := len(t.Columns); c < columnsPer; c++ {
			t.Columns = append(t.Columns, model.Column{
				Name:     fmt.Sprintf("entity_%d_id", (i+c)%tables),
				Type:     "bigint",
				BaseType: "int8",
				Position: c + 1,
			})
		}

		s.Tables = append(s.Tables, t)
	}
	return []model.Schema{s}
}

func benchOptions() infer.Options {
	p, err := profile.Load("en")
	if err != nil {
		panic(err)
	}
	return infer.Options{Profile: p}
}

func BenchmarkGenerate(b *testing.B) {
	for _, tables := range []int{100, 1_000, 5_000} {
		schemas := benchSchema(tables, 12, 0)
		opts := benchOptions()

		b.Run(fmt.Sprintf("tables=%d/single", tables), func(b *testing.B) {
			for b.Loop() {
				infer.Generate(schemas, opts)
			}
		})
	}
}

func BenchmarkGenerateComposite(b *testing.B) {
	// One table in ten carries a composite key, which is heavy for a real
	// schema and therefore the right thing to measure.
	for _, tables := range []int{100, 1_000, 5_000} {
		schemas := benchSchema(tables, 12, 10)
		opts := benchOptions()

		b.Run(fmt.Sprintf("tables=%d/composite", tables), func(b *testing.B) {
			for b.Loop() {
				infer.Generate(schemas, opts)
			}
		})
	}
}

// benchSchemaNoProfileMatch is the worst case for the lexical fallback: every
// reference column is named so that no profile form resolves it, which is what
// sends each one down the path that compares against every table in scope.
func benchSchemaNoProfileMatch(tables, columnsPer int) []model.Schema {
	s := model.Schema{Name: "public", Tables: make([]model.Table, 0, tables)}

	for i := range tables {
		t := model.Table{
			Schema:  "public",
			Name:    fmt.Sprintf("entity_%d", i),
			Columns: make([]model.Column, 0, columnsPer),
			Stats:   model.TableStats{EstimatedRows: 10_000},
		}
		t.Columns = append(t.Columns, model.Column{
			Name: "id", Type: "bigint", BaseType: "int8", Position: 1,
		})
		t.PrimaryKey = []string{"id"}

		for c := 1; c < columnsPer; c++ {
			t.Columns = append(t.Columns, model.Column{
				Name:     fmt.Sprintf("zzq%d_ref%d", i, c),
				Type:     "bigint",
				BaseType: "int8",
				Position: c + 1,
			})
		}
		s.Tables = append(s.Tables, t)
	}
	return []model.Schema{s}
}

func BenchmarkGenerateNoProfileMatch(b *testing.B) {
	for _, tables := range []int{100, 1_000, 5_000} {
		schemas := benchSchemaNoProfileMatch(tables, 12)
		opts := benchOptions()
		b.Run(fmt.Sprintf("tables=%d/fallback", tables), func(b *testing.B) {
			for b.Loop() {
				infer.Generate(schemas, opts)
			}
		})
	}
}
