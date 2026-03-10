package store

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"secsim/design/backend/internal/model"
)

type InboundMessage struct {
	Timestamp time.Time
	Stream    int
	Function  int
	WBit      bool
	RCMD      string
	Label     string
	Body      string
	RawSML    string
	Fields    map[string]string
}

type RuntimeResult struct {
	MatchedRuleID string
	MatchedRule   string
	Reply         *model.MessageRecord
	Emitted       []model.MessageRecord
	StateChanges  []StateChange
	Snapshot      model.Snapshot
}

type StateChange struct {
	Path     string
	OldValue string
	NewValue string
}

type scheduledAction struct {
	DueAt    time.Time
	RuleID   string
	RuleName string
	Action   model.RuleAction
}

func (s *Store) ProcessInbound(message InboundMessage, now time.Time) RuntimeResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	occurredAt := message.Timestamp
	if occurredAt.IsZero() {
		occurredAt = now
	}

	result := RuntimeResult{}
	inboundRecord := s.newInboundRecord(message, occurredAt)

	var diagnostics []model.ConditionEvaluation
	for _, rule := range s.config.Rules {
		if !rule.Enabled || !matchesPattern(rule, message) {
			continue
		}

		evaluations, matched := evaluateConditions(s.liveState, message, rule.Conditions)
		if matched {
			inboundRecord.MatchedRule = rule.Name
			inboundRecord.MatchedRuleID = rule.ID
			inboundRecord.Evaluations = evaluations
			result.MatchedRule = rule.Name
			result.MatchedRuleID = rule.ID

			reply := s.newReplyRecord(rule, occurredAt)
			s.messages = append(s.messages, inboundRecord, reply)
			replyCopy := reply
			result.Reply = &replyCopy

			for _, action := range rule.Actions {
				s.pending = append(s.pending, scheduledAction{
					DueAt:    occurredAt.Add(time.Duration(action.DelayMS) * time.Millisecond),
					RuleID:   rule.ID,
					RuleName: rule.Name,
					Action:   action,
				})
			}
			sortScheduledActions(s.pending)
			result.Snapshot = s.snapshotLocked()
			return result
		}

		if len(diagnostics) == 0 {
			diagnostics = evaluations
		}
	}

	inboundRecord.Evaluations = diagnostics
	s.messages = append(s.messages, inboundRecord)
	result.Snapshot = s.snapshotLocked()
	return result
}

func (s *Store) RunScheduled(now time.Time) (RuntimeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := RuntimeResult{
		Emitted:      []model.MessageRecord{},
		StateChanges: []StateChange{},
	}

	remaining := make([]scheduledAction, 0, len(s.pending))
	for _, pending := range s.pending {
		if pending.DueAt.After(now) {
			remaining = append(remaining, pending)
			continue
		}

		switch pending.Action.Type {
		case "event":
			record := s.newEventRecord(pending, pending.DueAt)
			s.messages = append(s.messages, record)
			result.Emitted = append(result.Emitted, record)
		case "mutate":
			change, err := applyMutation(&s.liveState, pending.Action.Target, pending.Action.Value)
			if err != nil {
				result.Snapshot = s.snapshotLocked()
				return result, err
			}
			result.StateChanges = append(result.StateChanges, change)
		default:
			result.Snapshot = s.snapshotLocked()
			return result, fmt.Errorf("unsupported action type: %s", pending.Action.Type)
		}
	}

	s.pending = remaining
	result.Snapshot = s.snapshotLocked()
	return result, nil
}

func matchesPattern(rule model.Rule, message InboundMessage) bool {
	if rule.Match.Stream != message.Stream || rule.Match.Function != message.Function {
		return false
	}
	if rule.Match.RCMD != "" && rule.Match.RCMD != message.RCMD {
		return false
	}

	return true
}

func evaluateConditions(state model.StateSnapshot, message InboundMessage, conditions []model.RuleCondition) ([]model.ConditionEvaluation, bool) {
	if len(conditions) == 0 {
		return []model.ConditionEvaluation{}, true
	}

	evaluations := make([]model.ConditionEvaluation, 0, len(conditions))
	allPassed := true
	for _, condition := range conditions {
		evaluation := evaluateCondition(state, message, condition)
		evaluations = append(evaluations, evaluation)
		if !evaluation.Passed {
			allPassed = false
		}
	}

	return evaluations, allPassed
}

func evaluateCondition(state model.StateSnapshot, message InboundMessage, condition model.RuleCondition) model.ConditionEvaluation {
	actual := ""
	passed := false

	switch condition.Field {
	case "carrier_exists":
		_, passed = state.Carriers[condition.Value]
		if passed {
			actual = "true"
		} else {
			actual = "false"
		}
	case "source_equals":
		actual = firstNonEmpty(
			message.Fields["source"],
			message.Fields["SourcePort"],
			message.Fields["source_port"],
		)
		passed = actual == condition.Value
	default:
		var ok bool
		actual, ok = resolveStatePath(state, condition.Field)
		passed = ok && actual == condition.Value
	}

	return model.ConditionEvaluation{
		Field:    condition.Field,
		Expected: condition.Value,
		Actual:   actual,
		Passed:   passed,
	}
}

func resolveStatePath(state model.StateSnapshot, path string) (string, bool) {
	segments := strings.Split(path, ".")
	switch {
	case len(segments) == 1 && segments[0] == "mode":
		return state.Mode, true
	case len(segments) == 2 && segments[0] == "ports":
		value, ok := state.Ports[segments[1]]
		return value, ok
	case len(segments) == 3 && segments[0] == "carriers" && segments[2] == "location":
		carrier, ok := state.Carriers[segments[1]]
		if !ok {
			return "", false
		}
		return carrier.Location, true
	default:
		return "", false
	}
}

func applyMutation(state *model.StateSnapshot, target string, value string) (StateChange, error) {
	segments := strings.Split(target, ".")
	switch {
	case len(segments) == 1 && segments[0] == "mode":
		change := StateChange{
			Path:     target,
			OldValue: state.Mode,
			NewValue: value,
		}
		state.Mode = value
		return change, nil
	case len(segments) == 2 && segments[0] == "ports":
		if state.Ports == nil {
			state.Ports = map[string]string{}
		}
		change := StateChange{
			Path:     target,
			OldValue: state.Ports[segments[1]],
			NewValue: value,
		}
		state.Ports[segments[1]] = value
		return change, nil
	case len(segments) == 3 && segments[0] == "carriers" && segments[2] == "location":
		if state.Carriers == nil {
			state.Carriers = map[string]model.CarrierState{}
		}
		carrier := state.Carriers[segments[1]]
		change := StateChange{
			Path:     target,
			OldValue: carrier.Location,
			NewValue: value,
		}
		carrier.Location = value
		state.Carriers[segments[1]] = carrier
		return change, nil
	default:
		return StateChange{}, fmt.Errorf("unsupported mutate target: %s", target)
	}
}

func (s *Store) newInboundRecord(message InboundMessage, occurredAt time.Time) model.MessageRecord {
	label := message.Label
	if label == "" {
		switch {
		case message.RCMD != "":
			label = fmt.Sprintf("Remote Command: %s", message.RCMD)
		default:
			label = formatSF(message.Stream, message.Function)
		}
	}

	body := message.Body
	if body == "" && message.RCMD != "" {
		body = fmt.Sprintf("<A %q>", message.RCMD)
	}
	rawSML := message.RawSML
	if rawSML == "" {
		rawSML = defaultRawSML(message.Stream, message.Function, message.WBit, body)
	}

	return model.MessageRecord{
		ID:        s.nextMessageIDValue(),
		Timestamp: formatTimestamp(occurredAt),
		Direction: "IN",
		SF:        formatSF(message.Stream, message.Function),
		Label:     label,
		Detail: model.MessageDetail{
			Stream:   message.Stream,
			Function: message.Function,
			WBit:     message.WBit,
			Body:     body,
			RawSML:   rawSML,
		},
	}
}

func (s *Store) newReplyRecord(rule model.Rule, occurredAt time.Time) model.MessageRecord {
	body := fmt.Sprintf("L:2\n  <B 0x%02X>\n  L:0", rule.Reply.Ack)
	return model.MessageRecord{
		ID:            s.nextMessageIDValue(),
		Timestamp:     formatTimestamp(occurredAt),
		Direction:     "OUT",
		SF:            formatSF(rule.Reply.Stream, rule.Reply.Function),
		Label:         "Remote Cmd Ack",
		MatchedRule:   rule.Name,
		MatchedRuleID: rule.ID,
		Detail: model.MessageDetail{
			Stream:   rule.Reply.Stream,
			Function: rule.Reply.Function,
			WBit:     false,
			Body:     body,
			RawSML:   defaultRawSML(rule.Reply.Stream, rule.Reply.Function, false, body),
		},
		Evaluations: []model.ConditionEvaluation{},
	}
}

func (s *Store) newEventRecord(action scheduledAction, occurredAt time.Time) model.MessageRecord {
	body := fmt.Sprintf("L:1\n  <A %q>", action.Action.CEID)
	return model.MessageRecord{
		ID:            s.nextMessageIDValue(),
		Timestamp:     formatTimestamp(occurredAt),
		Direction:     "OUT",
		SF:            "S6F11",
		Label:         action.Action.CEID,
		MatchedRule:   action.RuleName,
		MatchedRuleID: action.RuleID,
		Detail: model.MessageDetail{
			Stream:   6,
			Function: 11,
			WBit:     true,
			Body:     body,
			RawSML:   defaultRawSML(6, 11, true, body),
		},
		Evaluations: []model.ConditionEvaluation{},
	}
}

func sortScheduledActions(actions []scheduledAction) {
	sort.SliceStable(actions, func(i, j int) bool {
		if actions[i].DueAt.Equal(actions[j].DueAt) {
			return actions[i].Action.ID < actions[j].Action.ID
		}

		return actions[i].DueAt.Before(actions[j].DueAt)
	})
}

func formatTimestamp(ts time.Time) string {
	return ts.UTC().Format("15:04:05.000")
}

func formatSF(stream int, function int) string {
	return fmt.Sprintf("S%dF%d", stream, function)
}

func defaultRawSML(stream int, function int, wbit bool, body string) string {
	sml := formatSF(stream, function)
	if wbit {
		sml += " W"
	}
	if body != "" {
		sml += " " + strings.ReplaceAll(body, "\n", " ")
	}

	return sml
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
