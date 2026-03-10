package store

import "testing"

func TestNewRuleMarksConfigDirty(t *testing.T) {
	store := New()

	snapshot := store.NewRule()

	if len(snapshot.Rules) != 4 {
		t.Fatalf("expected 4 rules after insert, got %d", len(snapshot.Rules))
	}

	if !snapshot.Runtime.Dirty {
		t.Fatalf("expected config to be dirty after rule insert")
	}
}

func TestSaveAndReloadRestoresBaseline(t *testing.T) {
	store := New()

	snapshot := store.NewRule()
	if !snapshot.Runtime.Dirty {
		t.Fatalf("expected dirty snapshot")
	}

	snapshot = store.Save()
	if snapshot.Runtime.Dirty {
		t.Fatalf("expected save to clear dirty flag")
	}

	store.NewRule()
	snapshot = store.Reload()
	if len(snapshot.Rules) != 4 {
		t.Fatalf("expected reload to restore baseline rule count, got %d", len(snapshot.Rules))
	}

	if snapshot.Runtime.Dirty {
		t.Fatalf("expected reload to clear dirty flag")
	}
}

func TestMoveRuleSwapsOrder(t *testing.T) {
	store := New()

	initial := store.Snapshot()
	first := initial.Rules[0].ID
	second := initial.Rules[1].ID

	snapshot, err := store.MoveRule(second, "up")
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}

	if snapshot.Rules[0].ID != second || snapshot.Rules[1].ID != first {
		t.Fatalf("expected rules to swap positions")
	}
}
