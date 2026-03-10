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
	dataIDItem, _, err := parseGeneratorItem(firstNonEmpty(action.DataID, "U4:0"))
	if err != nil {
		return hsms.Item{}, "", fmt.Errorf("parse event DATAID %q: %w", action.DataID, err)
	}

	ceidItem, label, err := parseGeneratorItem(action.CEID)
	if err != nil {
		return hsms.Item{}, "", fmt.Errorf("parse event CEID %q: %w", action.CEID, err)
	}

	reports := make([]hsms.Item, 0, len(action.Reports))
	for _, report := range action.Reports {
		rptidItem, _, err := parseGeneratorItem(firstNonEmpty(report.RPTID, "U4:0"))
		if err != nil {
			return hsms.Item{}, "", fmt.Errorf("parse event RPTID %q: %w", report.RPTID, err)
		}

		values := make([]hsms.Item, 0, len(report.Values))
		for _, value := range report.Values {
			valueItem, _, err := parseGeneratorItem(value)
			if err != nil {
				return hsms.Item{}, "", fmt.Errorf("parse report value %q: %w", value, err)
			}
			values = append(values, valueItem)
		}

		reports = append(reports, hsms.List(
			rptidItem,
			hsms.List(values...),
		))
	}

	return hsms.List(
		dataIDItem,
		ceidItem,
		hsms.List(reports...),
	), label, nil
}

func parseGeneratorItem(raw string) (hsms.Item, string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return hsms.ASCII(""), "", nil
	}
	if hasGeneratorPrefix(value, "L:[") {
		return parseGeneratorList(value)
	}
	return parseGeneratorScalar(value)
}

func parseGeneratorScalar(value string) (hsms.Item, string, error) {
	switch {
	case hasGeneratorPrefix(value, "A:"):
		payload, err := parseASCIIValue(value[len("A:"):])
		if err != nil {
			return hsms.Item{}, "", err
		}
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

func parseGeneratorList(value string) (hsms.Item, string, error) {
	if !strings.HasSuffix(value, "]") {
		return hsms.Item{}, "", fmt.Errorf("list item must end with ]")
	}

	inner := strings.TrimSpace(value[len("L:[") : len(value)-1])
	if inner == "" {
		return hsms.List(), "", nil
	}

	parts, err := splitTopLevelListItems(inner)
	if err != nil {
		return hsms.Item{}, "", err
	}

	children := make([]hsms.Item, 0, len(parts))
	for _, part := range parts {
		child, _, err := parseGeneratorItem(part)
		if err != nil {
			return hsms.Item{}, "", err
		}
		children = append(children, child)
	}

	return hsms.List(children...), "", nil
}

func hasGeneratorPrefix(value string, prefix string) bool {
	return len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix)
}

func parseASCIIValue(raw string) (string, error) {
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
