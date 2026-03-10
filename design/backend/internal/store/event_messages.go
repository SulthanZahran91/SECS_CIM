package store

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"secsim/design/backend/internal/hsms"
	"secsim/design/backend/internal/model"
)

func buildScheduledEventMessage(sessionID uint16, messageID string, action scheduledAction, occurredAt time.Time) (hsms.Message, model.MessageRecord, error) {
	body, label, err := buildEventBody(action.Action)
	if err != nil {
		return hsms.Message{}, model.MessageRecord{}, err
	}

	message := hsms.Message{
		SessionID:   sessionID,
		Stream:      6,
		Function:    11,
		WBit:        true,
		SystemBytes: 0,
		Body:        &body,
	}
	if label == "" {
		label = message.Label()
	}
	record := model.MessageRecord{
		ID:            messageID,
		Timestamp:     formatTimestamp(occurredAt),
		Direction:     "OUT",
		SF:            "S6F11",
		Label:         label,
		MatchedRule:   action.RuleName,
		MatchedRuleID: action.RuleID,
		Detail: model.MessageDetail{
			Stream:   6,
			Function: 11,
			WBit:     true,
			Body:     message.BodySML(),
			RawSML:   message.RawSML(),
		},
		Evaluations: []model.ConditionEvaluation{},
	}

	return message, record, nil
}

func buildEventBody(action model.RuleAction) (hsms.Item, string, error) {
	if !eventUsesStructuredBody(action) {
		return hsms.List(hsms.ASCII(action.CEID)), action.CEID, nil
	}

	ceidItem, label, err := parseGeneratorScalar(action.CEID)
	if err != nil {
		return hsms.Item{}, "", fmt.Errorf("parse event CEID %q: %w", action.CEID, err)
	}

	reports := make([]hsms.Item, 0, len(action.Reports))
	for _, report := range action.Reports {
		rptidItem, _, err := parseGeneratorScalar(firstNonEmpty(report.RPTID, "0"))
		if err != nil {
			return hsms.Item{}, "", fmt.Errorf("parse event RPTID %q: %w", report.RPTID, err)
		}

		values := make([]hsms.Item, 0, len(report.Variables))
		for _, variable := range report.Variables {
			valueItem, _, err := parseGeneratorScalar(variable.Value)
			if err != nil {
				return hsms.Item{}, "", fmt.Errorf("parse VID %q value %q: %w", variable.VID, variable.Value, err)
			}
			values = append(values, valueItem)
		}

		reports = append(reports, hsms.List(
			rptidItem,
			hsms.List(values...),
		))
	}

	return hsms.List(
		hsms.U4(0),
		ceidItem,
		hsms.List(reports...),
	), label, nil
}

func eventUsesStructuredBody(action model.RuleAction) bool {
	if len(action.Reports) > 0 {
		return true
	}

	value := strings.TrimSpace(action.CEID)
	if value == "" {
		return false
	}
	if _, err := strconv.ParseUint(value, 10, 32); err == nil {
		return true
	}

	for _, prefix := range []string{"A:", "U1:", "U2:", "U4:", "BOOL:", "B:"} {
		if hasGeneratorPrefix(value, prefix) {
			return true
		}
	}

	return false
}

func parseGeneratorScalar(raw string) (hsms.Item, string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return hsms.ASCII(""), "", nil
	}

	switch {
	case hasGeneratorPrefix(value, "A:"):
		payload := strings.TrimSpace(value[len("A:"):])
		return hsms.ASCII(payload), payload, nil
	case hasGeneratorPrefix(value, "U1:"):
		parsed, err := parseUint(value[len("U1:"):], 8)
		if err != nil {
			return hsms.Item{}, "", err
		}
		return hsms.U1(uint8(parsed)), strconv.FormatUint(parsed, 10), nil
	case hasGeneratorPrefix(value, "U2:"):
		parsed, err := parseUint(value[len("U2:"):], 16)
		if err != nil {
			return hsms.Item{}, "", err
		}
		return hsms.U2(uint16(parsed)), strconv.FormatUint(parsed, 10), nil
	case hasGeneratorPrefix(value, "U4:"):
		parsed, err := parseUint(value[len("U4:"):], 32)
		if err != nil {
			return hsms.Item{}, "", err
		}
		return hsms.U4(uint32(parsed)), strconv.FormatUint(parsed, 10), nil
	case hasGeneratorPrefix(value, "BOOL:"):
		parsed, err := strconv.ParseBool(strings.TrimSpace(value[len("BOOL:"):]))
		if err != nil {
			return hsms.Item{}, "", err
		}
		return hsms.Boolean(parsed), strconv.FormatBool(parsed), nil
	case hasGeneratorPrefix(value, "B:"):
		parsed, err := parseBinaryBytes(value[len("B:"):])
		if err != nil {
			return hsms.Item{}, "", err
		}
		return hsms.Binary(parsed...), strings.TrimSpace(value[len("B:"):]), nil
	default:
		if parsed, err := strconv.ParseUint(value, 10, 32); err == nil {
			return hsms.U4(uint32(parsed)), strconv.FormatUint(parsed, 10), nil
		}
		return hsms.ASCII(value), value, nil
	}
}

func hasGeneratorPrefix(value string, prefix string) bool {
	return len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix)
}

func parseUint(raw string, bits int) (uint64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("empty numeric value")
	}

	parsed, err := strconv.ParseUint(value, 10, bits)
	if err != nil {
		return 0, err
	}

	return parsed, nil
}

func parseBinaryBytes(raw string) ([]byte, error) {
	normalized := strings.NewReplacer(",", " ", "\n", " ", "\t", " ").Replace(raw)
	parts := strings.Fields(normalized)
	if len(parts) == 0 {
		return []byte{}, nil
	}

	bytes := make([]byte, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimPrefix(strings.TrimPrefix(part, "0x"), "0X")
		parsed, err := strconv.ParseUint(token, 16, 8)
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, byte(parsed))
	}

	return bytes, nil
}
