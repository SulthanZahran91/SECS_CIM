package model

import "sort"

type Snapshot struct {
	Runtime  RuntimeState    `json:"runtime"`
	HSMS     HsmsConfig      `json:"hsms"`
	Device   DeviceConfig    `json:"device"`
	State    StateSnapshot   `json:"state"`
	Rules    []Rule          `json:"rules"`
	Messages []MessageRecord `json:"messages"`
}

type RuntimeState struct {
	Listening       bool   `json:"listening"`
	HSMSState       string `json:"hsmsState"`
	ConfigFile      string `json:"configFile"`
	Dirty           bool   `json:"dirty"`
	RestartRequired bool   `json:"restartRequired"`
	LastError       string `json:"lastError,omitempty"`
}

type HsmsConfig struct {
	Mode      string          `json:"mode"`
	IP        string          `json:"ip"`
	Port      int             `json:"port"`
	SessionID int             `json:"sessionId"`
	DeviceID  int             `json:"deviceId"`
	Timers    HsmsTimers      `json:"timers"`
	Handshake HandshakeConfig `json:"handshake"`
}

type HsmsTimers struct {
	T3 int `json:"t3"`
	T5 int `json:"t5"`
	T6 int `json:"t6"`
	T7 int `json:"t7"`
	T8 int `json:"t8"`
}

type HandshakeConfig struct {
	AutoS1F13 bool `json:"autoS1f13"`
	AutoS1F1  bool `json:"autoS1f1"`
	AutoS2F25 bool `json:"autoS2f25"`
}

type DeviceConfig struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	MDLN     string `json:"mdln"`
	SoftRev  string `json:"softrev"`
}

type StateSnapshot struct {
	Mode     string                  `json:"mode"`
	Ports    map[string]string       `json:"ports"`
	Carriers map[string]CarrierState `json:"carriers"`
}

type CarrierState struct {
	Location string `json:"location"`
}

type Rule struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Enabled    bool            `json:"enabled"`
	Match      RuleMatch       `json:"match"`
	Conditions []RuleCondition `json:"conditions"`
	Reply      RuleReply       `json:"reply"`
	Actions    []RuleAction    `json:"actions"`
}

type RuleMatch struct {
	Stream   int    `json:"stream"`
	Function int    `json:"function"`
	RCMD     string `json:"rcmd"`
}

type RuleCondition struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type RuleReply struct {
	Stream   int `json:"stream"`
	Function int `json:"function"`
	Ack      int `json:"ack"`
}

type RuleAction struct {
	ID      string `json:"id"`
	DelayMS int    `json:"delayMs"`
	Type    string `json:"type"`
	CEID    string `json:"ceid,omitempty"`
	Target  string `json:"target,omitempty"`
	Value   string `json:"value,omitempty"`
}

type MessageRecord struct {
	ID            string                `json:"id"`
	Timestamp     string                `json:"timestamp"`
	Direction     string                `json:"direction"`
	SF            string                `json:"sf"`
	Label         string                `json:"label"`
	MatchedRule   string                `json:"matchedRule,omitempty"`
	MatchedRuleID string                `json:"matchedRuleId,omitempty"`
	Detail        MessageDetail         `json:"detail"`
	Evaluations   []ConditionEvaluation `json:"evaluations,omitempty"`
}

type MessageDetail struct {
	Stream   int    `json:"stream"`
	Function int    `json:"function"`
	WBit     bool   `json:"wbit"`
	Body     string `json:"body"`
	RawSML   string `json:"rawSml"`
}

type ConditionEvaluation struct {
	Field    string `json:"field"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Passed   bool   `json:"passed"`
}

func CloneSnapshot(src Snapshot) Snapshot {
	cloned := Snapshot{
		Runtime: src.Runtime,
		HSMS:    src.HSMS,
		Device:  src.Device,
		State: StateSnapshot{
			Mode:     src.State.Mode,
			Ports:    make(map[string]string, len(src.State.Ports)),
			Carriers: make(map[string]CarrierState, len(src.State.Carriers)),
		},
		Rules:    make([]Rule, 0, len(src.Rules)),
		Messages: make([]MessageRecord, 0, len(src.Messages)),
	}

	for key, value := range src.State.Ports {
		cloned.State.Ports[key] = value
	}

	for key, value := range src.State.Carriers {
		cloned.State.Carriers[key] = value
	}

	for _, rule := range src.Rules {
		ruleCopy := Rule{
			ID:         rule.ID,
			Name:       rule.Name,
			Enabled:    rule.Enabled,
			Match:      rule.Match,
			Conditions: cloneSlice(rule.Conditions),
			Reply:      rule.Reply,
			Actions:    cloneSlice(rule.Actions),
		}
		cloned.Rules = append(cloned.Rules, ruleCopy)
	}

	for _, message := range src.Messages {
		messageCopy := MessageRecord{
			ID:            message.ID,
			Timestamp:     message.Timestamp,
			Direction:     message.Direction,
			SF:            message.SF,
			Label:         message.Label,
			MatchedRule:   message.MatchedRule,
			MatchedRuleID: message.MatchedRuleID,
			Detail:        message.Detail,
			Evaluations:   cloneSlice(message.Evaluations),
		}
		cloned.Messages = append(cloned.Messages, messageCopy)
	}

	return cloned
}

func cloneSlice[T any](src []T) []T {
	return append(make([]T, 0, len(src)), src...)
}

func SortActions(actions []RuleAction) {
	sort.SliceStable(actions, func(i, j int) bool {
		if actions[i].DelayMS == actions[j].DelayMS {
			return actions[i].ID < actions[j].ID
		}
		return actions[i].DelayMS < actions[j].DelayMS
	})
}
