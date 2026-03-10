package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"secsim/design/backend/internal/model"
)

func newFileBackedStore(t *testing.T) (*Store, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "stocker-sim.yaml")
	if err := writeSnapshotToYAML(path, seedSnapshot()); err != nil {
		t.Fatalf("seed config file: %v", err)
	}

	store, err := NewFromFile(path)
	if err != nil {
		t.Fatalf("create file-backed store: %v", err)
	}

	return store, path
}

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
	store, path := newFileBackedStore(t)

	snapshot := store.NewRule()
	if !snapshot.Runtime.Dirty {
		t.Fatalf("expected dirty snapshot")
	}

	snapshot, err := store.Save()
	if err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	if snapshot.Runtime.Dirty {
		t.Fatalf("expected save to clear dirty flag")
	}

	fileContents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	if !strings.Contains(string(fileContents), "new rule") {
		t.Fatalf("expected saved config to include new rule, got:\n%s", string(fileContents))
	}

	store.NewRule()
	snapshot, err = store.Reload()
	if err != nil {
		t.Fatalf("reload snapshot: %v", err)
	}
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

func TestSaveAndReloadPreservesStructuredEventReports(t *testing.T) {
	store, _ := newFileBackedStore(t)

	rule := store.Snapshot().Rules[0]
	rule.Actions = []model.RuleAction{
		{
			ID:      "action-1",
			DelayMS: 25,
			Type:    "event",
			CEID:    "1001",
			Reports: []model.RuleActionReport{
				{
					RPTID: "5001",
					Variables: []model.RuleActionVariable{
						{VID: "100", Value: "A:LP01"},
					},
				},
			},
		},
	}
	if _, err := store.UpdateRule(rule); err != nil {
		t.Fatalf("update rule: %v", err)
	}

	if _, err := store.Save(); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	reloaded, err := store.Reload()
	if err != nil {
		t.Fatalf("reload snapshot: %v", err)
	}

	savedAction := reloaded.Rules[0].Actions[0]
	if savedAction.CEID != "1001" || len(savedAction.Reports) != 1 {
		t.Fatalf("expected structured event report after reload, got %#v", savedAction)
	}
	if savedAction.Reports[0].RPTID != "5001" || len(savedAction.Reports[0].Variables) != 1 {
		t.Fatalf("expected structured report variables after reload, got %#v", savedAction.Reports[0])
	}
	if savedAction.Reports[0].Variables[0].VID != "100" || savedAction.Reports[0].Variables[0].Value != "A:LP01" {
		t.Fatalf("expected VID/value payload after reload, got %#v", savedAction.Reports[0].Variables[0])
	}
}

func TestReloadRestoresBaselineButKeepsRuntimeState(t *testing.T) {
	store, path := newFileBackedStore(t)

	store.ToggleRuntime()
	store.NewRule()
	store.ClearLog()

	if err := os.WriteFile(path, []byte(`
hsms:
  mode: active
  ip: "127.0.0.1"
  port: 6000
  session_id: 9
  device_id: 7
  timers:
    t3: 30
    t5: 9
    t6: 4
    t7: 11
    t8: 6
device:
  name: stocker-B
  protocol: e87
  mdln: STOCKER-SIM-B
  softrev: 2.0.0
handshake:
  auto_s1f13: false
  auto_s1f1: true
  auto_s2f25: true
  auto_host_startup: false
initial_state:
  mode: online-local
  ports:
    LP09: occupied
  carriers:
    CARR009:
      location: LP09
rules:
  - name: file rule
    enabled: true
    match:
      stream: 2
      function: 41
      rcmd: TRANSFER
    reply:
      stream: 2
      function: 42
      ack: 1
    events:
      - delay_ms: 50
        type: event
        ceid: FILE_EVENT
`), 0o644); err != nil {
		t.Fatalf("overwrite config file: %v", err)
	}

	snapshot, err := store.Reload()
	if err != nil {
		t.Fatalf("reload snapshot: %v", err)
	}

	if !snapshot.Runtime.Listening {
		t.Fatalf("expected runtime listening state to be preserved as true")
	}

	if snapshot.Runtime.HSMSState != "NOT CONNECTED" {
		t.Fatalf("expected HSMS state to be preserved, got %q", snapshot.Runtime.HSMSState)
	}

	if snapshot.HSMS.Mode != "active" || snapshot.HSMS.Port != 6000 {
		t.Fatalf("expected HSMS settings to reload from disk, got %#v", snapshot.HSMS)
	}

	if snapshot.Device.Name != "stocker-B" {
		t.Fatalf("expected device config to reload from disk, got %#v", snapshot.Device)
	}

	if len(snapshot.Rules) != 1 || snapshot.Rules[0].Name != "file rule" {
		t.Fatalf("expected rules to reload from disk, got %#v", snapshot.Rules)
	}

	if len(snapshot.Messages) != 0 {
		t.Fatalf("expected message log to be preserved, got %d messages", len(snapshot.Messages))
	}
}

func TestDeleteRuleReturnsNotFoundForUnknownID(t *testing.T) {
	store := New()

	_, err := store.DeleteRule("missing")
	if !errors.Is(err, ErrRuleNotFound) {
		t.Fatalf("expected ErrRuleNotFound, got %v", err)
	}
}

func TestRuntimeErrorClearsOnRecovery(t *testing.T) {
	store := New()

	store.SetRuntimeError("connection refused")
	if got := store.Snapshot().Runtime.LastError; got != "connection refused" {
		t.Fatalf("expected runtime error to be stored, got %q", got)
	}

	store.SetRuntime(true, "CONNECTING")
	if got := store.Snapshot().Runtime.LastError; got != "connection refused" {
		t.Fatalf("expected runtime error to persist while reconnecting, got %q", got)
	}

	store.SetRuntime(true, "SELECTED")
	if got := store.Snapshot().Runtime.LastError; got != "" {
		t.Fatalf("expected runtime error to clear on recovery, got %q", got)
	}

	store.SetRuntimeError("connection dropped")
	store.SetRuntime(false, "NOT CONNECTED")
	if got := store.Snapshot().Runtime.LastError; got != "" {
		t.Fatalf("expected runtime error to clear when stopped, got %q", got)
	}
}

func TestDirtyTrackingClearsWhenConfigMatchesBaselineAgain(t *testing.T) {
	store := New()

	baseline := store.Snapshot().Device
	changed := baseline
	changed.Name = "different"

	snapshot := store.UpdateDevice(changed)
	if !snapshot.Runtime.Dirty {
		t.Fatalf("expected dirty after changing device config")
	}

	snapshot = store.UpdateDevice(baseline)
	if snapshot.Runtime.Dirty {
		t.Fatalf("expected dirty flag to clear once config matches baseline again")
	}
}

func TestRestartRequiredTracksPendingConnectionChanges(t *testing.T) {
	store := New()

	applied := store.Snapshot().HSMS
	store.RecordAppliedHSMS(applied)
	store.SetRuntime(true, "SELECTED")

	if store.Snapshot().Runtime.RestartRequired {
		t.Fatalf("expected restartRequired to be false before connection changes")
	}

	changed := applied
	changed.Port++
	snapshot := store.UpdateHSMS(changed)
	if !snapshot.Runtime.RestartRequired {
		t.Fatalf("expected restartRequired after changing HSMS connection settings")
	}

	snapshot, err := store.Save()
	if err != nil {
		t.Fatalf("save changed HSMS config: %v", err)
	}
	if !snapshot.Runtime.RestartRequired {
		t.Fatalf("expected save to preserve restartRequired until runtime is restarted")
	}

	snapshot = store.SetRuntime(false, "NOT CONNECTED")
	if snapshot.Runtime.RestartRequired {
		t.Fatalf("expected stopping runtime to clear restartRequired")
	}
}

func TestRestartRequiredIgnoresNonConnectionHSMSChanges(t *testing.T) {
	store := New()

	applied := store.Snapshot().HSMS
	store.RecordAppliedHSMS(applied)
	store.SetRuntime(true, "SELECTED")

	changed := applied
	changed.Timers.T8++
	changed.SessionID++

	snapshot := store.UpdateHSMS(changed)
	if snapshot.Runtime.RestartRequired {
		t.Fatalf("expected non-connection HSMS changes to avoid restartRequired")
	}
}

func TestNewFromFileLoadsYAMLConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sim.yaml")
	if err := os.WriteFile(path, []byte(`
hsms:
  mode: active
  ip: "10.0.0.9"
  port: 7000
  session_id: 13
  device_id: 3
  timers:
    t3: 33
    t5: 8
    t6: 4
    t7: 12
    t8: 6
device:
  name: load-test
  protocol: e88
  mdln: LOAD-SIM
  softrev: 9.9.9
handshake:
  auto_s1f13: false
  auto_s1f1: false
  auto_s2f25: true
  auto_host_startup: true
initial_state:
  mode: online-local
  ports:
    LP01: empty
  carriers: {}
rules:
  - name: yaml rule
    match:
      stream: 1
      function: 1
      rcmd: STATUS
    reply:
      stream: 1
      function: 2
      ack: 0
    events:
      - delay_ms: 10
        type: event
        ceid: "1001"
        reports:
          - rptid: "5001"
            variables:
              - vid: "100"
                value: "A:LP01"
`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	store, err := NewFromFile(path)
	if err != nil {
		t.Fatalf("create store from file: %v", err)
	}

	snapshot := store.Snapshot()
	if snapshot.Runtime.ConfigFile != path {
		t.Fatalf("expected config file path %q, got %q", path, snapshot.Runtime.ConfigFile)
	}
	if snapshot.HSMS.Mode != "active" || snapshot.HSMS.IP != "10.0.0.9" {
		t.Fatalf("expected HSMS config from file, got %#v", snapshot.HSMS)
	}
	if !snapshot.HSMS.Handshake.AutoS2F25 || snapshot.HSMS.Handshake.AutoS1F13 || !snapshot.HSMS.Handshake.AutoHostStartup {
		t.Fatalf("expected handshake config from file, got %#v", snapshot.HSMS.Handshake)
	}
	if len(snapshot.Rules) != 1 || snapshot.Rules[0].ID != "rule-1" {
		t.Fatalf("expected file rules to load with generated IDs, got %#v", snapshot.Rules)
	}
	if len(snapshot.Rules[0].Actions) != 1 || snapshot.Rules[0].Actions[0].ID != "action-1" {
		t.Fatalf("expected file actions to load with generated IDs, got %#v", snapshot.Rules[0].Actions)
	}
	if len(snapshot.Rules[0].Actions[0].Reports) != 1 || snapshot.Rules[0].Actions[0].Reports[0].RPTID != "5001" {
		t.Fatalf("expected structured event reports from file, got %#v", snapshot.Rules[0].Actions[0])
	}
	if len(snapshot.Rules[0].Actions[0].Reports[0].Variables) != 1 || snapshot.Rules[0].Actions[0].Reports[0].Variables[0].VID != "100" {
		t.Fatalf("expected structured event variables from file, got %#v", snapshot.Rules[0].Actions[0].Reports[0].Variables)
	}
}

func TestReloadReturnsErrorAndKeepsCurrentConfigWhenYAMLIsInvalid(t *testing.T) {
	store, path := newFileBackedStore(t)
	baseline := store.Snapshot()

	if err := os.WriteFile(path, []byte("rules:\n  - name: broken\n    extra: ["), 0o644); err != nil {
		t.Fatalf("write invalid config file: %v", err)
	}

	if _, err := store.Reload(); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}

	current := store.Snapshot()
	if !configEquals(current, baseline) {
		t.Fatalf("expected current config to remain unchanged after failed reload")
	}
}
