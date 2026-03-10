package store

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"secsim/design/backend/internal/hsms"
	"secsim/design/backend/internal/model"
)

func buildScheduledSendMessage(sessionID uint16, messageID string, action scheduledAction, occurredAt time.Time) (hsms.Message, model.MessageRecord, error) {
	var body *hsms.Item
	if strings.TrimSpace(action.Action.Body) != "" {
		parsed, err := hsms.ParseSMLItem(action.Action.Body)
		if err != nil {
			return hsms.Message{}, model.MessageRecord{}, fmt.Errorf("parse action body: %w", err)
		}
		body = &parsed
	}

	message := hsms.Message{
		SessionID:   sessionID,
		Stream:      byte(action.Action.Stream),
		Function:    byte(action.Action.Function),
		WBit:        action.Action.WBit,
		SystemBytes: 0,
		Body:        body,
	}

	record := model.MessageRecord{
		ID:            messageID,
		Timestamp:     formatTimestamp(occurredAt),
		Direction:     "OUT",
		SF:            formatSF(action.Action.Stream, action.Action.Function),
		Label:         message.Label(),
		MatchedRule:   action.RuleName,
		MatchedRuleID: action.RuleID,
		Detail: model.MessageDetail{
			Stream:   action.Action.Stream,
			Function: action.Action.Function,
			WBit:     action.Action.WBit,
			Body:     message.BodySML(),
			RawSML:   message.RawSML(),
		},
		Evaluations: []model.ConditionEvaluation{},
	}

	return message, record, nil
}

func parseLegacyEventExpression(raw string) (hsms.Item, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return hsms.ASCII(""), nil
	}
	if hasLegacyPrefix(value, "L:[") {
		return parseLegacyListExpression(value)
	}
	return parseLegacyScalar(value)
}

func parseLegacyScalar(value string) (hsms.Item, error) {
	switch {
	case hasLegacyPrefix(value, "A:"):
		payload, err := parseLegacyASCIIValue(value[len("A:"):])
		if err != nil {
			return hsms.Item{}, err
		}
		return hsms.ASCII(payload), nil
	case hasLegacyPrefix(value, "I:"):
		parsed, err := strconv.ParseInt(strings.TrimSpace(value[len("I:"):]), 10, 32)
		if err != nil {
			return hsms.Item{}, err
		}
		return hsms.I4(int32(parsed)), nil
	case hasLegacyPrefix(value, "I1:"):
		parsed, err := strconv.ParseInt(strings.TrimSpace(value[len("I1:"):]), 10, 8)
		if err != nil {
			return hsms.Item{}, err
		}
		return hsms.I1(int8(parsed)), nil
	case hasLegacyPrefix(value, "I2:"):
		parsed, err := strconv.ParseInt(strings.TrimSpace(value[len("I2:"):]), 10, 16)
		if err != nil {
			return hsms.Item{}, err
		}
		return hsms.I2(int16(parsed)), nil
	case hasLegacyPrefix(value, "I4:"):
		parsed, err := strconv.ParseInt(strings.TrimSpace(value[len("I4:"):]), 10, 32)
		if err != nil {
			return hsms.Item{}, err
		}
		return hsms.I4(int32(parsed)), nil
	case hasLegacyPrefix(value, "U:"):
		parsed, err := parseLegacyUint(value[len("U:"):], 32)
		if err != nil {
			return hsms.Item{}, err
		}
		return hsms.U4(uint32(parsed)), nil
	case hasLegacyPrefix(value, "U1:"):
		parsed, err := parseLegacyUint(value[len("U1:"):], 8)
		if err != nil {
			return hsms.Item{}, err
		}
		return hsms.U1(uint8(parsed)), nil
	case hasLegacyPrefix(value, "U2:"):
		parsed, err := parseLegacyUint(value[len("U2:"):], 16)
		if err != nil {
			return hsms.Item{}, err
		}
		return hsms.U2(uint16(parsed)), nil
	case hasLegacyPrefix(value, "U4:"):
		parsed, err := parseLegacyUint(value[len("U4:"):], 32)
		if err != nil {
			return hsms.Item{}, err
		}
		return hsms.U4(uint32(parsed)), nil
	case hasLegacyPrefix(value, "BOOL:"):
		parsed, err := strconv.ParseBool(strings.TrimSpace(value[len("BOOL:"):]))
		if err != nil {
			return hsms.Item{}, err
		}
		return hsms.Boolean(parsed), nil
	case hasLegacyPrefix(value, "B:"):
		parsed, err := parseBinaryBytes(value[len("B:"):])
		if err != nil {
			return hsms.Item{}, err
		}
		return hsms.Binary(parsed...), nil
	default:
		if parsed, err := strconv.ParseInt(value, 10, 32); err == nil {
			return hsms.I4(int32(parsed)), nil
		}
		return hsms.ASCII(value), nil
	}
}

func parseLegacyListExpression(value string) (hsms.Item, error) {
	if !strings.HasSuffix(value, "]") {
		return hsms.Item{}, fmt.Errorf("list item must end with ]")
	}

	inner := strings.TrimSpace(value[len("L:[") : len(value)-1])
	if inner == "" {
		return hsms.List(), nil
	}

	parts, err := splitTopLevelListItems(inner)
	if err != nil {
		return hsms.Item{}, err
	}

	children := make([]hsms.Item, 0, len(parts))
	for _, part := range parts {
		child, err := parseLegacyEventExpression(part)
		if err != nil {
			return hsms.Item{}, err
		}
		children = append(children, child)
	}

	return hsms.List(children...), nil
}

func hasLegacyPrefix(value string, prefix string) bool {
	return len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix)
}

func parseLegacyASCIIValue(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return "", err
		}
		return unquoted, nil
	}

	return value, nil
}

func parseLegacyUint(raw string, bits int) (uint64, error) {
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

func splitTopLevelListItems(raw string) ([]string, error) {
	items := []string{}
	start := 0
	depth := 0
	inQuote := false
	escaped := false

	for index, char := range raw {
		switch {
		case inQuote && escaped:
			escaped = false
		case inQuote && char == '\\':
			escaped = true
		case inQuote && char == '"':
			inQuote = false
		case !inQuote && char == '"':
			inQuote = true
		case !inQuote && char == '[':
			depth++
		case !inQuote && char == ']':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unexpected closing ] in list expression")
			}
		case !inQuote && char == ',' && depth == 0:
			item := strings.TrimSpace(raw[start:index])
			if item != "" {
				items = append(items, item)
			}
			start = index + 1
		}
	}

	if inQuote {
		return nil, fmt.Errorf("unterminated string in list expression")
	}
	if depth != 0 {
		return nil, fmt.Errorf("unbalanced list brackets in list expression")
	}

	tail := strings.TrimSpace(raw[start:])
	if tail != "" {
		items = append(items, tail)
	}

	return items, nil
}
