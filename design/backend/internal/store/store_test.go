package store

import (
	"errors"
	"testing"

	"secsim/design/backend/internal/model"
)

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

func TestUpdateRuleSortsActionsAndDefaultsBlankName(t *testing.T) {
	store := New()
	rule := store.Snapshot().Rules[0]
	rule.Name = ""
	rule.Actions = []model.RuleAction{
		{ID: "action-z", DelayMS: 500, Type: "event", CEID: "LATE"},
		{ID: "action-a", DelayMS: 100, Type: "event", CEID: "EARLY"},
		{ID: "action-b", DelayMS: 500, Type: "mutate", Target: "ports.LP01", Value: "empty"},
	}

	snapshot, err := store.UpdateRule(rule)
	if err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	updated := snapshot.Rules[0]
	if updated.Name != "unnamed rule" {
		t.Fatalf("expected blank rule name to default, got %q", updated.Name)
	}

	if updated.Actions[0].ID != "action-a" || updated.Actions[1].ID != "action-b" || updated.Actions[2].ID != "action-z" {
		t.Fatalf("expected actions to be sorted by delay then ID, got %#v", updated.Actions)
	}
}

func TestDuplicateRuleCreatesDistinctRuleAndActionIDs(t *testing.T) {
	store := New()

	snapshot, err := store.DuplicateRule("rule-1")
	if err != nil {
		t.Fatalf("unexpected duplicate error: %v", err)
	}

	if len(snapshot.Rules) != 4 {
		t.Fatalf("expected duplicated rule to increase count, got %d", len(snapshot.Rules))
	}

	original := snapshot.Rules[0]
	duplicate := snapshot.Rules[1]

	if duplicate.ID == original.ID {
		t.Fatalf("expected duplicate rule to have a distinct ID")
	}

	if duplicate.Name != original.Name+" (copy)" {
		t.Fatalf("expected duplicate rule name suffix, got %q", duplicate.Name)
	}

	if len(duplicate.Actions) != len(original.Actions) {
		t.Fatalf("expected duplicate to copy action count")
	}

	for index := range original.Actions {
		if duplicate.Actions[index].ID == original.Actions[index].ID {
			t.Fatalf("expected duplicate action %d to have a distinct ID", index)
		}
		if duplicate.Actions[index].DelayMS != original.Actions[index].DelayMS {
			t.Fatalf("expected duplicate action %d to preserve payload", index)
		}
	}
}

func TestReloadRestoresBaselineButKeepsRuntimeState(t *testing.T) {
	store := New()

	store.ToggleRuntime()
	store.NewRule()
	store.ClearLog()

	snapshot := store.Reload()

	if snapshot.Runtime.Listening {
		t.Fatalf("expected runtime listening state to be preserved as false")
	}

	if snapshot.Runtime.HSMSState != "NOT CONNECTED" {
		t.Fatalf("expected HSMS state to be preserved, got %q", snapshot.Runtime.HSMSState)
	}

	if len(snapshot.Rules) != 3 {
		t.Fatalf("expected baseline rules to be restored, got %d", len(snapshot.Rules))
	}

	if len(snapshot.Messages) != 7 {
		t.Fatalf("expected baseline messages to be restored, got %d", len(snapshot.Messages))
	}
}

func TestDeleteRuleReturnsNotFoundForUnknownID(t *testing.T) {
	store := New()

	_, err := store.DeleteRule("missing")
	if !errors.Is(err, ErrRuleNotFound) {
		t.Fatalf("expected ErrRuleNotFound, got %v", err)
	}
}
