package store

import (
	"errors"
	"fmt"
	"sync"

	"secsim/design/backend/internal/model"
)

var (
	ErrRuleNotFound = errors.New("rule not found")
)

type Store struct {
	mu           sync.RWMutex
	snapshot     model.Snapshot
	baseline     model.Snapshot
	nextRuleID   int
	nextActionID int
}

func New() *Store {
	snapshot := seedSnapshot()
	return &Store{
		snapshot:     model.CloneSnapshot(snapshot),
		baseline:     model.CloneSnapshot(snapshot),
		nextRuleID:   4,
		nextActionID: 6,
	}
}

func (s *Store) Snapshot() model.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return model.CloneSnapshot(s.snapshot)
}

func (s *Store) ToggleRuntime() model.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.Runtime.Listening = !s.snapshot.Runtime.Listening
	if s.snapshot.Runtime.Listening {
		s.snapshot.Runtime.HSMSState = "SELECTED"
	} else {
		s.snapshot.Runtime.HSMSState = "NOT CONNECTED"
	}

	return model.CloneSnapshot(s.snapshot)
}

func (s *Store) Save() model.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.Runtime.Dirty = false
	s.baseline = model.CloneSnapshot(s.snapshot)

	return model.CloneSnapshot(s.snapshot)
}

func (s *Store) Reload() model.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentRuntime := s.snapshot.Runtime
	s.snapshot = model.CloneSnapshot(s.baseline)
	s.snapshot.Runtime.Listening = currentRuntime.Listening
	s.snapshot.Runtime.HSMSState = currentRuntime.HSMSState
	s.snapshot.Runtime.ConfigFile = currentRuntime.ConfigFile
	s.snapshot.Runtime.Dirty = false

	return model.CloneSnapshot(s.snapshot)
}

func (s *Store) ClearLog() model.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.Messages = nil

	return model.CloneSnapshot(s.snapshot)
}

func (s *Store) UpdateHSMS(config model.HsmsConfig) model.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.HSMS = config
	s.touchDirty()

	return model.CloneSnapshot(s.snapshot)
}

func (s *Store) UpdateDevice(device model.DeviceConfig) model.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.Device = device
	s.touchDirty()

	return model.CloneSnapshot(s.snapshot)
}

func (s *Store) NewRule() model.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	newRule := model.Rule{
		ID:      s.nextRuleIDValue(),
		Name:    "new rule",
		Enabled: true,
		Match: model.RuleMatch{
			Stream:   0,
			Function: 0,
			RCMD:     "",
		},
		Conditions: nil,
		Reply: model.RuleReply{
			Stream:   0,
			Function: 0,
			Ack:      0,
		},
		Actions: nil,
	}
	s.snapshot.Rules = append(s.snapshot.Rules, newRule)
	s.touchDirty()

	return model.CloneSnapshot(s.snapshot)
}

func (s *Store) UpdateRule(updated model.Rule) (model.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index := range s.snapshot.Rules {
		if s.snapshot.Rules[index].ID != updated.ID {
			continue
		}

		model.SortActions(updated.Actions)
		if updated.Name == "" {
			updated.Name = "unnamed rule"
		}
		s.snapshot.Rules[index] = updated
		s.touchDirty()

		return model.CloneSnapshot(s.snapshot), nil
	}

	return model.Snapshot{}, ErrRuleNotFound
}

func (s *Store) DuplicateRule(id string) (model.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index, rule := range s.snapshot.Rules {
		if rule.ID != id {
			continue
		}

		duplicate := model.Rule{
			ID:         s.nextRuleIDValue(),
			Name:       fmt.Sprintf("%s (copy)", rule.Name),
			Enabled:    rule.Enabled,
			Match:      rule.Match,
			Conditions: append([]model.RuleCondition(nil), rule.Conditions...),
			Reply:      rule.Reply,
			Actions:    make([]model.RuleAction, 0, len(rule.Actions)),
		}
		for _, action := range rule.Actions {
			duplicate.Actions = append(duplicate.Actions, model.RuleAction{
				ID:      s.nextActionIDValue(),
				DelayMS: action.DelayMS,
				Type:    action.Type,
				CEID:    action.CEID,
				Target:  action.Target,
				Value:   action.Value,
			})
		}

		nextRules := append([]model.Rule(nil), s.snapshot.Rules[:index+1]...)
		nextRules = append(nextRules, duplicate)
		nextRules = append(nextRules, s.snapshot.Rules[index+1:]...)
		s.snapshot.Rules = nextRules
		s.touchDirty()

		return model.CloneSnapshot(s.snapshot), nil
	}

	return model.Snapshot{}, ErrRuleNotFound
}

func (s *Store) DeleteRule(id string) (model.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index, rule := range s.snapshot.Rules {
		if rule.ID != id {
			continue
		}

		s.snapshot.Rules = append(s.snapshot.Rules[:index], s.snapshot.Rules[index+1:]...)
		s.touchDirty()

		return model.CloneSnapshot(s.snapshot), nil
	}

	return model.Snapshot{}, ErrRuleNotFound
}

func (s *Store) MoveRule(id string, direction string) (model.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index, rule := range s.snapshot.Rules {
		if rule.ID != id {
			continue
		}

		target := index
		switch direction {
		case "up":
			target = index - 1
		case "down":
			target = index + 1
		default:
			return model.Snapshot{}, fmt.Errorf("unsupported move direction: %s", direction)
		}

		if target < 0 || target >= len(s.snapshot.Rules) {
			return model.CloneSnapshot(s.snapshot), nil
		}

		s.snapshot.Rules[index], s.snapshot.Rules[target] = s.snapshot.Rules[target], s.snapshot.Rules[index]
		s.touchDirty()

		return model.CloneSnapshot(s.snapshot), nil
	}

	return model.Snapshot{}, ErrRuleNotFound
}

func (s *Store) touchDirty() {
	s.snapshot.Runtime.Dirty = true
}

func (s *Store) nextRuleIDValue() string {
	id := fmt.Sprintf("rule-%d", s.nextRuleID)
	s.nextRuleID++
	return id
}

func (s *Store) nextActionIDValue() string {
	id := fmt.Sprintf("action-%d", s.nextActionID)
	s.nextActionID++
	return id
}
