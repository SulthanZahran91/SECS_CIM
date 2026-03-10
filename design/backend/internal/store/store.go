package store

import (
	"errors"
	"fmt"
	"sync"

	"secsim/design/backend/internal/model"
)

var (
	ErrRuleNotFound  = errors.New("rule not found")
	ErrInvalidConfig = errors.New("invalid config")
)

type Store struct {
	mu            sync.RWMutex
	config        model.Snapshot
	baseline      model.Snapshot
	liveState     model.StateSnapshot
	messages      []model.MessageRecord
	pending       []scheduledAction
	nextRuleID    int
	nextActionID  int
	nextMessageID int
	configPath    string
}

func New() *Store {
	snapshot := seedSnapshot()
	return newStore(snapshot, "")
}

func NewFromFile(path string) (*Store, error) {
	snapshot := seedSnapshot()
	if path != "" {
		snapshot.Runtime.ConfigFile = path
	}

	store := newStore(snapshot, path)
	if path == "" {
		return store, nil
	}

	loaded, err := loadSnapshotFromYAML(path, snapshot)
	if err != nil {
		if errors.Is(err, ErrInvalidConfig) {
			return nil, err
		}
		return store, nil
	}

	store.config = cloneConfigSnapshot(loaded)
	store.baseline = cloneConfigSnapshot(loaded)
	store.liveState = normalizeState(loaded.State)
	store.messages = cloneMessages(loaded.Messages)
	store.pending = nil
	store.updateDirtyLocked()
	store.resetIDCountersLocked()

	return store, nil
}

func newStore(snapshot model.Snapshot, configPath string) *Store {
	store := &Store{
		config:     cloneConfigSnapshot(snapshot),
		baseline:   cloneConfigSnapshot(snapshot),
		liveState:  normalizeState(snapshot.State),
		messages:   cloneMessages(snapshot.Messages),
		pending:    nil,
		configPath: configPath,
	}
	store.updateDirtyLocked()
	store.resetIDCountersLocked()

	return store
}

func (s *Store) Snapshot() model.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.snapshotLocked()
}

func (s *Store) ConfigSnapshot() model.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return model.CloneSnapshot(s.config)
}

func (s *Store) ToggleRuntime() model.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	nextListening := !s.config.Runtime.Listening
	s.setRuntimeLocked(nextListening, "NOT CONNECTED")

	return s.snapshotLocked()
}

func (s *Store) SetRuntime(listening bool, hsmsState string) model.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.setRuntimeLocked(listening, hsmsState)
	return s.snapshotLocked()
}

func (s *Store) SetHSMSState(hsmsState string) model.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.Runtime.HSMSState = hsmsState
	return s.snapshotLocked()
}

func (s *Store) Save() (model.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.configPath != "" {
		if err := writeSnapshotToYAML(s.configPath, s.config); err != nil {
			return model.Snapshot{}, err
		}
	}

	s.baseline = cloneConfigSnapshot(s.config)
	s.updateDirtyLocked()

	return s.snapshotLocked(), nil
}

func (s *Store) Reload() (model.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentRuntime := s.config.Runtime
	currentMessages := cloneMessages(s.messages)

	var nextConfig model.Snapshot
	if s.configPath != "" {
		loaded, err := loadSnapshotFromYAML(s.configPath, s.baseline)
		if err != nil {
			return model.Snapshot{}, err
		}
		nextConfig = loaded
	} else {
		nextConfig = model.CloneSnapshot(s.baseline)
	}

	s.config = cloneConfigSnapshot(nextConfig)
	s.baseline = cloneConfigSnapshot(nextConfig)
	s.config.Runtime.Listening = currentRuntime.Listening
	s.config.Runtime.HSMSState = currentRuntime.HSMSState
	s.config.Runtime.ConfigFile = currentRuntime.ConfigFile
	s.liveState = normalizeState(nextConfig.State)
	s.messages = currentMessages
	s.pending = nil
	s.updateDirtyLocked()
	s.resetIDCountersLocked()

	return s.snapshotLocked(), nil
}

func (s *Store) ClearLog() model.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = []model.MessageRecord{}
	s.resetIDCountersLocked()

	return s.snapshotLocked()
}

func (s *Store) UpdateHSMS(config model.HsmsConfig) model.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.HSMS = config
	s.updateDirtyLocked()

	return s.snapshotLocked()
}

func (s *Store) UpdateDevice(device model.DeviceConfig) model.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.Device = device
	s.updateDirtyLocked()

	return s.snapshotLocked()
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
		Conditions: []model.RuleCondition{},
		Reply: model.RuleReply{
			Stream:   0,
			Function: 0,
			Ack:      0,
		},
		Actions: []model.RuleAction{},
	}
	s.config.Rules = append(s.config.Rules, newRule)
	s.updateDirtyLocked()

	return s.snapshotLocked()
}

func (s *Store) UpdateRule(updated model.Rule) (model.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index := range s.config.Rules {
		if s.config.Rules[index].ID != updated.ID {
			continue
		}

		if updated.Conditions == nil {
			updated.Conditions = []model.RuleCondition{}
		}
		if updated.Actions == nil {
			updated.Actions = []model.RuleAction{}
		}
		model.SortActions(updated.Actions)
		if updated.Name == "" {
			updated.Name = "unnamed rule"
		}
		s.config.Rules[index] = updated
		s.updateDirtyLocked()

		return s.snapshotLocked(), nil
	}

	return model.Snapshot{}, ErrRuleNotFound
}

func (s *Store) DuplicateRule(id string) (model.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index, rule := range s.config.Rules {
		if rule.ID != id {
			continue
		}

		duplicate := model.Rule{
			ID:         s.nextRuleIDValue(),
			Name:       fmt.Sprintf("%s (copy)", rule.Name),
			Enabled:    rule.Enabled,
			Match:      rule.Match,
			Conditions: append(make([]model.RuleCondition, 0, len(rule.Conditions)), rule.Conditions...),
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

		nextRules := append([]model.Rule(nil), s.config.Rules[:index+1]...)
		nextRules = append(nextRules, duplicate)
		nextRules = append(nextRules, s.config.Rules[index+1:]...)
		s.config.Rules = nextRules
		s.updateDirtyLocked()

		return s.snapshotLocked(), nil
	}

	return model.Snapshot{}, ErrRuleNotFound
}

func (s *Store) DeleteRule(id string) (model.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index, rule := range s.config.Rules {
		if rule.ID != id {
			continue
		}

		s.config.Rules = append(s.config.Rules[:index], s.config.Rules[index+1:]...)
		s.updateDirtyLocked()

		return s.snapshotLocked(), nil
	}

	return model.Snapshot{}, ErrRuleNotFound
}

func (s *Store) MoveRule(id string, direction string) (model.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index, rule := range s.config.Rules {
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

		if target < 0 || target >= len(s.config.Rules) {
			return s.snapshotLocked(), nil
		}

		s.config.Rules[index], s.config.Rules[target] = s.config.Rules[target], s.config.Rules[index]
		s.updateDirtyLocked()

		return s.snapshotLocked(), nil
	}

	return model.Snapshot{}, ErrRuleNotFound
}

func (s *Store) snapshotLocked() model.Snapshot {
	snapshot := model.CloneSnapshot(s.config)
	snapshot.State = normalizeState(s.liveState)
	snapshot.Messages = cloneMessages(s.messages)
	return snapshot
}

func (s *Store) updateDirtyLocked() {
	s.config.Runtime.Dirty = !configEquals(s.config, s.baseline)
}

func (s *Store) setRuntimeLocked(listening bool, hsmsState string) {
	s.config.Runtime.Listening = listening
	s.config.Runtime.HSMSState = hsmsState
	if !listening {
		s.pending = nil
	}
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

func (s *Store) nextMessageIDValue() string {
	id := fmt.Sprintf("msg-%d", s.nextMessageID)
	s.nextMessageID++
	return id
}

func (s *Store) resetIDCountersLocked() {
	s.nextRuleID = nextIdentifierValue("rule-", ruleIDs(s.config.Rules))
	s.nextActionID = nextIdentifierValue("action-", actionIDs(s.config.Rules))
	s.nextMessageID = nextIdentifierValue("msg-", messageIDs(s.messages))
}

func cloneConfigSnapshot(snapshot model.Snapshot) model.Snapshot {
	cloned := model.CloneSnapshot(snapshot)
	cloned.Messages = []model.MessageRecord{}
	cloned.Runtime.Dirty = false
	return cloned
}

func cloneMessages(messages []model.MessageRecord) []model.MessageRecord {
	cloned := make([]model.MessageRecord, 0, len(messages))
	for _, message := range messages {
		cloned = append(cloned, model.MessageRecord{
			ID:            message.ID,
			Timestamp:     message.Timestamp,
			Direction:     message.Direction,
			SF:            message.SF,
			Label:         message.Label,
			MatchedRule:   message.MatchedRule,
			MatchedRuleID: message.MatchedRuleID,
			Detail:        message.Detail,
			Evaluations:   append(make([]model.ConditionEvaluation, 0, len(message.Evaluations)), message.Evaluations...),
		})
	}

	return cloned
}

func messageIDs(messages []model.MessageRecord) []string {
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}

	return ids
}
