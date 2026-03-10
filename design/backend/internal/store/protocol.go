package store

import (
	"time"

	"secsim/design/backend/internal/model"
)

type ProtocolMessage struct {
	Timestamp     time.Time
	Direction     string
	Stream        int
	Function      int
	WBit          bool
	Label         string
	Body          string
	RawSML        string
	MatchedRule   string
	MatchedRuleID string
	Evaluations   []model.ConditionEvaluation
}

func (s *Store) AppendProtocolMessage(message ProtocolMessage) (model.MessageRecord, model.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	occurredAt := message.Timestamp
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	label := message.Label
	if label == "" {
		label = formatSF(message.Stream, message.Function)
	}

	rawSML := message.RawSML
	if rawSML == "" {
		rawSML = defaultRawSML(message.Stream, message.Function, message.WBit, message.Body)
	}

	record := model.MessageRecord{
		ID:            s.nextMessageIDValue(),
		Timestamp:     formatTimestamp(occurredAt),
		Direction:     message.Direction,
		SF:            formatSF(message.Stream, message.Function),
		Label:         label,
		MatchedRule:   message.MatchedRule,
		MatchedRuleID: message.MatchedRuleID,
		Detail: model.MessageDetail{
			Stream:   message.Stream,
			Function: message.Function,
			WBit:     message.WBit,
			Body:     message.Body,
			RawSML:   rawSML,
		},
		Evaluations: append(make([]model.ConditionEvaluation, 0, len(message.Evaluations)), message.Evaluations...),
	}

	s.messages = append(s.messages, record)
	return record, s.snapshotAndPublishLocked()
}
