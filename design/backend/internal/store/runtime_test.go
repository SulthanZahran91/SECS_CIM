package store

import (
	"testing"
	"time"

	"secsim/design/backend/internal/hsms"
	"secsim/design/backend/internal/model"
)

func awaitSnapshotUpdate(t *testing.T, updates <-chan model.Snapshot) model.Snapshot {
	t.Helper()

	select {
	case snapshot := <-updates:
		return snapshot
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for snapshot update")
		return model.Snapshot{}
	}
}

func TestProcessInboundMatchesFirstRuleAndSchedulesActions(t *testing.T) {
	store := New()
	store.ClearLog()

	now := time.Date(2026, time.March, 10, 16, 0, 0, 0, time.UTC)
	result := store.ProcessInbound(InboundMessage{
		Stream:   2,
		Function: 41,
		WBit:     true,
		RCMD:     "TRANSFER",
		Fields: map[string]string{
			"SourcePort": "LP01",
		},
		Body:   "TRANSFER from LP01",
		RawSML: "S2F41 W TRANSFER LP01",
	}, now)

	if result.MatchedRuleID != "rule-1" || result.MatchedRule != "accept transfer" {
		t.Fatalf("expected rule-1 match, got %#v", result)
	}
	if result.Reply == nil {
		t.Fatalf("expected immediate reply record")
	}
	if len(store.pending) != 4 {
		t.Fatalf("expected 4 scheduled actions, got %d", len(store.pending))
	}
	if result.Snapshot.Runtime.Dirty {
		t.Fatalf("expected runtime processing to leave config clean")
	}
	if result.Snapshot.State.Ports["LP01"] != "occupied" {
		t.Fatalf("expected state mutation to be deferred until scheduled actions run")
	}
	if len(result.Snapshot.Messages) != 2 {
		t.Fatalf("expected inbound and reply messages, got %d", len(result.Snapshot.Messages))
	}

	inbound := result.Snapshot.Messages[0]
	if inbound.MatchedRuleID != "rule-1" || len(inbound.Evaluations) != 2 {
		t.Fatalf("expected inbound message diagnostics for matched rule, got %#v", inbound)
	}
	for _, evaluation := range inbound.Evaluations {
		if !evaluation.Passed {
			t.Fatalf("expected all matched rule evaluations to pass, got %#v", inbound.Evaluations)
		}
	}

	if result.Reply.SF != "S2F42" {
		t.Fatalf("expected S2F42 reply, got %#v", result.Reply)
	}
}

func TestProcessInboundFallsThroughToLaterRule(t *testing.T) {
	store := New()
	store.ClearLog()
	store.liveState.Ports["LP01"] = "blocked"

	now := time.Date(2026, time.March, 10, 16, 5, 0, 0, time.UTC)
	result := store.ProcessInbound(InboundMessage{
		Stream:   2,
		Function: 41,
		WBit:     true,
		RCMD:     "TRANSFER",
		Fields: map[string]string{
			"SourcePort": "LP02",
		},
	}, now)

	if result.MatchedRuleID != "rule-2" || result.MatchedRule != "reject when blocked" {
		t.Fatalf("expected fallback match against rule-2, got %#v", result)
	}
	if result.Reply == nil || result.Reply.Detail.Body == "" {
		t.Fatalf("expected reply from fallback rule, got %#v", result.Reply)
	}
	if result.Snapshot.Messages[0].MatchedRuleID != "rule-2" {
		t.Fatalf("expected inbound log to reference rule-2, got %#v", result.Snapshot.Messages[0])
	}
}

func TestProcessInboundCapturesDiagnosticsForNearMiss(t *testing.T) {
	store := New()
	store.ClearLog()

	now := time.Date(2026, time.March, 10, 16, 10, 0, 0, time.UTC)
	result := store.ProcessInbound(InboundMessage{
		Stream:   2,
		Function: 41,
		WBit:     true,
		RCMD:     "TRANSFER",
		Fields: map[string]string{
			"SourcePort": "LP99",
		},
	}, now)

	if result.MatchedRuleID != "" {
		t.Fatalf("expected no full rule match, got %#v", result)
	}
	if len(result.Snapshot.Messages) != 1 {
		t.Fatalf("expected unmatched inbound log only, got %d", len(result.Snapshot.Messages))
	}

	inbound := result.Snapshot.Messages[0]
	if len(inbound.Evaluations) != 2 {
		t.Fatalf("expected diagnostics from first near-miss rule, got %#v", inbound)
	}
	if inbound.Evaluations[0].Field != "carrier_exists" || !inbound.Evaluations[0].Passed {
		t.Fatalf("expected carrier_exists diagnostic to pass, got %#v", inbound.Evaluations)
	}
	if inbound.Evaluations[1].Field != "source_equals" || inbound.Evaluations[1].Passed {
		t.Fatalf("expected source_equals diagnostic to fail, got %#v", inbound.Evaluations)
	}
}

func TestProcessInboundSupportsGenericMessageFieldConditions(t *testing.T) {
	store := New()
	store.ClearLog()

	rule := store.Snapshot().Rules[0]
	rule.Conditions = []model.RuleCondition{
		{Field: "DATA.RCMD", Value: "TRANSFER"},
		{Field: "fields.SourcePort", Value: "LP01"},
		{Field: "CarrierID", Value: "CARR001"},
		{Field: "state.ports.LP01", Value: "occupied"},
	}
	if _, err := store.UpdateRule(rule); err != nil {
		t.Fatalf("update rule: %v", err)
	}

	now := time.Date(2026, time.March, 10, 16, 12, 0, 0, time.UTC)
	result := store.ProcessInbound(InboundMessage{
		Stream:   2,
		Function: 41,
		WBit:     true,
		RCMD:     "TRANSFER",
		Fields: map[string]string{
			"SourcePort": "LP01",
			"CarrierID":  "CARR001",
		},
	}, now)

	if result.MatchedRuleID != "rule-1" {
		t.Fatalf("expected generic field conditions to match rule-1, got %#v", result)
	}

	for _, evaluation := range result.Snapshot.Messages[0].Evaluations {
		if !evaluation.Passed {
			t.Fatalf("expected generic field evaluation to pass, got %#v", result.Snapshot.Messages[0].Evaluations)
		}
	}
}

func TestRunScheduledAppliesMutationsWithoutDirtyingConfig(t *testing.T) {
	store := New()
	store.ClearLog()

	now := time.Date(2026, time.March, 10, 16, 15, 0, 0, time.UTC)
	store.ProcessInbound(InboundMessage{
		Stream:   2,
		Function: 41,
		WBit:     true,
		RCMD:     "TRANSFER",
		Fields: map[string]string{
			"SourcePort": "LP01",
		},
	}, now)

	early, err := store.RunScheduled(now.Add(300 * time.Millisecond))
	if err != nil {
		t.Fatalf("run early scheduled actions: %v", err)
	}
	if len(early.Emitted) != 1 || early.Emitted[0].Label != "TRANSFER_INITIATED" {
		t.Fatalf("expected first event at +300ms, got %#v", early.Emitted)
	}
	if early.Snapshot.State.Ports["LP01"] != "occupied" {
		t.Fatalf("expected no mutation before +1200ms, got %#v", early.Snapshot.State)
	}

	late, err := store.RunScheduled(now.Add(1200 * time.Millisecond))
	if err != nil {
		t.Fatalf("run late scheduled actions: %v", err)
	}
	if len(late.Emitted) != 1 || late.Emitted[0].Label != "TRANSFER_COMPLETED" {
		t.Fatalf("expected completion event at +1200ms, got %#v", late.Emitted)
	}
	if late.Snapshot.State.Ports["LP01"] != "empty" {
		t.Fatalf("expected LP01 mutation to apply, got %#v", late.Snapshot.State.Ports)
	}
	if late.Snapshot.State.Carriers["CARR001"].Location != "SHELF_A01" {
		t.Fatalf("expected carrier mutation to apply, got %#v", late.Snapshot.State.Carriers)
	}
	if late.Snapshot.Runtime.Dirty {
		t.Fatalf("expected runtime mutations to leave config clean")
	}
	if len(store.pending) != 0 {
		t.Fatalf("expected scheduled queue to drain, got %d", len(store.pending))
	}
}

func TestRunScheduledBuildsStructuredEventReports(t *testing.T) {
	store := New()
	store.ClearLog()

	rule := store.Snapshot().Rules[0]
	rule.Actions = []model.RuleAction{
		{
			ID:      "action-1",
			DelayMS: 0,
			Type:    "event",
			DataID:  "U4:0",
			CEID:    "U4:1001",
			Reports: []model.RuleActionReport{
				{
					RPTID:  "U4:5001",
					Values: []string{"L:[U4:1, A:\"LP01\"]", "7"},
				},
			},
		},
	}
	if _, err := store.UpdateRule(rule); err != nil {
		t.Fatalf("update rule: %v", err)
	}

	now := time.Date(2026, time.March, 10, 16, 17, 0, 0, time.UTC)
	store.ProcessInbound(InboundMessage{
		Stream:   2,
		Function: 41,
		WBit:     true,
		RCMD:     "TRANSFER",
		Fields: map[string]string{
			"SourcePort": "LP01",
		},
	}, now)

	result, err := store.RunScheduled(now)
	if err != nil {
		t.Fatalf("run scheduled actions: %v", err)
	}

	if len(result.Emitted) != 1 || len(result.Outbound) != 1 {
		t.Fatalf("expected one structured event, got emitted=%d outbound=%d", len(result.Emitted), len(result.Outbound))
	}

	event := result.Outbound[0]
	if ceid, ok := hsms.ExtractS6F11CEID(event); !ok || ceid != "1001" {
		t.Fatalf("expected structured CEID 1001, got %#v", event)
	}
	if event.Body == nil || len(event.Body.Children) != 3 {
		t.Fatalf("expected structured S6F11 body, got %#v", event.Body)
	}

	reports := event.Body.Children[2]
	if reports.Type != hsms.ItemList || len(reports.Children) != 1 {
		t.Fatalf("expected one report list, got %#v", reports)
	}
	report := reports.Children[0]
	if report.Type != hsms.ItemList || len(report.Children) != 2 {
		t.Fatalf("expected report pair, got %#v", report)
	}
	if got := report.Children[0].ScalarValue(); got != "5001" {
		t.Fatalf("expected RPTID 5001, got %q", got)
	}
	values := report.Children[1]
	if values.Type != hsms.ItemList || len(values.Children) != 2 {
		t.Fatalf("expected two report values, got %#v", values)
	}
	if values.Children[0].Type != hsms.ItemList || len(values.Children[0].Children) != 2 {
		t.Fatalf("expected first report value to be a nested list item, got %#v", values.Children[0])
	}
	if got := values.Children[0].Children[0].ScalarValue(); got != "1" {
		t.Fatalf("expected nested list first value 1, got %q", got)
	}
	if got := values.Children[0].Children[1].ScalarValue(); got != "LP01" {
		t.Fatalf("expected nested list second value LP01, got %q", got)
	}
	if got := values.Children[1].ScalarValue(); got != "7" {
		t.Fatalf("expected second report value 7, got %q", got)
	}
	if result.Emitted[0].Detail.RawSML != event.RawSML() {
		t.Fatalf("expected logged raw SML to match outbound message, got %q vs %q", result.Emitted[0].Detail.RawSML, event.RawSML())
	}
}

func TestRuntimeMutationsDoNotPersistAsConfig(t *testing.T) {
	store, _ := newFileBackedStore(t)
	store.ClearLog()

	now := time.Date(2026, time.March, 10, 16, 20, 0, 0, time.UTC)
	store.ProcessInbound(InboundMessage{
		Stream:   2,
		Function: 41,
		WBit:     true,
		RCMD:     "TRANSFER",
		Fields: map[string]string{
			"SourcePort": "LP01",
		},
	}, now)

	if _, err := store.RunScheduled(now.Add(1200 * time.Millisecond)); err != nil {
		t.Fatalf("run scheduled actions: %v", err)
	}

	mutated := store.Snapshot()
	if mutated.State.Ports["LP01"] != "empty" {
		t.Fatalf("expected live state to mutate before save, got %#v", mutated.State.Ports)
	}
	if mutated.Runtime.Dirty {
		t.Fatalf("expected runtime-only state changes not to dirty config")
	}

	if _, err := store.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}
	reloaded, err := store.Reload()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if reloaded.State.Ports["LP01"] != "occupied" {
		t.Fatalf("expected reload to restore configured initial state, got %#v", reloaded.State.Ports)
	}
	if reloaded.State.Carriers["CARR001"].Location != "LP01" {
		t.Fatalf("expected reload to restore configured carrier location, got %#v", reloaded.State.Carriers)
	}
}

func TestRuntimePublishesSnapshotUpdatesForInboundAndScheduledActions(t *testing.T) {
	store := New()
	store.ClearLog()

	updates, initial, unsubscribe := store.SubscribeSnapshots()
	defer unsubscribe()

	if len(initial.Messages) != 0 {
		t.Fatalf("expected subscription to start from cleared log, got %d messages", len(initial.Messages))
	}

	now := time.Date(2026, time.March, 10, 16, 25, 0, 0, time.UTC)
	store.ProcessInbound(InboundMessage{
		Stream:   2,
		Function: 41,
		WBit:     true,
		RCMD:     "TRANSFER",
		Fields: map[string]string{
			"SourcePort": "LP01",
		},
	}, now)

	afterInbound := awaitSnapshotUpdate(t, updates)
	if len(afterInbound.Messages) != 2 {
		t.Fatalf("expected inbound update to include request and reply, got %d", len(afterInbound.Messages))
	}

	if _, err := store.RunScheduled(now.Add(1200 * time.Millisecond)); err != nil {
		t.Fatalf("run scheduled actions: %v", err)
	}

	afterScheduled := awaitSnapshotUpdate(t, updates)
	if afterScheduled.State.Ports["LP01"] != "empty" {
		t.Fatalf("expected scheduled mutation to publish updated state, got %#v", afterScheduled.State.Ports)
	}
	if len(afterScheduled.Messages) != 4 {
		t.Fatalf("expected scheduled update to append emitted events, got %d", len(afterScheduled.Messages))
	}
}
