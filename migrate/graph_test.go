// migrate/graph_test.go
package migrate_test

import (
	"testing"

	"github.com/ocomsoft/makemigrations/migrate"
)

// buildLinearRegistry creates a registry with a simple linear chain for testing.
func buildLinearRegistry() *migrate.Registry {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001_initial", Dependencies: []string{}})
	reg.Register(&migrate.Migration{Name: "0002_add_phone", Dependencies: []string{"0001_initial"}})
	reg.Register(&migrate.Migration{Name: "0003_add_slug", Dependencies: []string{"0002_add_phone"}})
	return reg
}

func TestGraph_Linearize_Simple(t *testing.T) {
	g, err := migrate.BuildGraph(buildLinearRegistry())
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	order, err := g.Linearize()
	if err != nil {
		t.Fatalf("Linearize: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(order))
	}
	if order[0].Name != "0001_initial" {
		t.Fatalf("expected first to be 0001_initial, got %q", order[0].Name)
	}
}

func TestGraph_Leaves_Linear(t *testing.T) {
	g, _ := migrate.BuildGraph(buildLinearRegistry())
	leaves := g.Leaves()
	if len(leaves) != 1 || leaves[0] != "0003_add_slug" {
		t.Fatalf("expected single leaf '0003_add_slug', got %v", leaves)
	}
}

func TestGraph_Roots(t *testing.T) {
	g, _ := migrate.BuildGraph(buildLinearRegistry())
	roots := g.Roots()
	if len(roots) != 1 || roots[0] != "0001_initial" {
		t.Fatalf("expected single root '0001_initial', got %v", roots)
	}
}

func TestGraph_DetectBranches_Linear(t *testing.T) {
	g, _ := migrate.BuildGraph(buildLinearRegistry())
	branches := g.DetectBranches()
	if len(branches) != 0 {
		t.Fatalf("expected no branches, got %v", branches)
	}
}

func TestGraph_DetectBranches_WithBranch(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001_initial", Dependencies: []string{}})
	reg.Register(&migrate.Migration{Name: "0002_base", Dependencies: []string{"0001_initial"}})
	reg.Register(&migrate.Migration{Name: "0003_feature_a", Dependencies: []string{"0002_base"}})
	reg.Register(&migrate.Migration{Name: "0003_feature_b", Dependencies: []string{"0002_base"}})

	g, _ := migrate.BuildGraph(reg)
	branches := g.DetectBranches()
	if len(branches) == 0 {
		t.Fatal("expected branches to be detected")
	}
}

func TestGraph_CycleDetection(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001", Dependencies: []string{"0002"}})
	reg.Register(&migrate.Migration{Name: "0002", Dependencies: []string{"0001"}})

	_, err := migrate.BuildGraph(reg)
	if err == nil {
		t.Fatal("expected error for cyclic dependency")
	}
}

func TestGraph_MissingDependency(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0002_add_phone", Dependencies: []string{"0001_missing"}})

	_, err := migrate.BuildGraph(reg)
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}
}

func TestGraph_ReconstructState(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_initial",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.CreateTable{
				Name:   "users",
				Fields: []migrate.Field{{Name: "id", Type: "uuid", PrimaryKey: true}},
			},
		},
	})
	reg.Register(&migrate.Migration{
		Name:         "0002_add_phone",
		Dependencies: []string{"0001_initial"},
		Operations: []migrate.Operation{
			&migrate.AddField{
				Table: "users",
				Field: migrate.Field{Name: "phone", Type: "varchar", Length: 20, Nullable: true},
			},
		},
	})

	g, err := migrate.BuildGraph(reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	state, err := g.ReconstructState()
	if err != nil {
		t.Fatalf("ReconstructState: %v", err)
	}
	users, ok := state.Tables["users"]
	if !ok {
		t.Fatal("expected 'users' table in reconstructed state")
	}
	if len(users.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(users.Fields))
	}
}

func TestGraph_ToDAGOutput(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_initial",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.CreateTable{
				Name:   "users",
				Fields: []migrate.Field{{Name: "id", Type: "uuid", PrimaryKey: true}},
			},
		},
	})
	g, err := migrate.BuildGraph(reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	out, err := g.ToDAGOutput()
	if err != nil {
		t.Fatalf("ToDAGOutput: %v", err)
	}
	if len(out.Migrations) != 1 {
		t.Fatalf("expected 1 migration summary, got %d", len(out.Migrations))
	}
	if out.Migrations[0].Name != "0001_initial" {
		t.Fatalf("expected '0001_initial', got %q", out.Migrations[0].Name)
	}
	if len(out.Migrations[0].Operations) != 1 {
		t.Fatalf("expected 1 operation summary, got %d", len(out.Migrations[0].Operations))
	}
	if out.Migrations[0].Operations[0].Type != "create_table" {
		t.Fatalf("expected type 'create_table', got %q", out.Migrations[0].Operations[0].Type)
	}
	if out.SchemaState == nil {
		t.Fatal("expected non-nil SchemaState in DAGOutput")
	}
	if _, ok := out.SchemaState.Tables["users"]; !ok {
		t.Fatal("expected 'users' in SchemaState")
	}
}
