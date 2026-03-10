package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"secsim/design/backend/internal/model"
)

type yamlConfig struct {
	HSMS         yamlHSMSConfig      `yaml:"hsms"`
	Device       model.DeviceConfig  `yaml:"device"`
	Handshake    yamlHandshakeConfig `yaml:"handshake"`
	InitialState model.StateSnapshot `yaml:"initial_state"`
	Rules        []yamlRule          `yaml:"rules"`
}

type yamlHSMSConfig struct {
	Mode      string           `yaml:"mode"`
	IP        string           `yaml:"ip"`
	Port      int              `yaml:"port"`
	SessionID int              `yaml:"session_id"`
	DeviceID  int              `yaml:"device_id"`
	Timers    model.HsmsTimers `yaml:"timers"`
}

type yamlRule struct {
	Name       string                `yaml:"name"`
	Enabled    *bool                 `yaml:"enabled,omitempty"`
	Match      yamlRuleMatch         `yaml:"match"`
	Conditions []model.RuleCondition `yaml:"conditions,omitempty"`
	Reply      yamlRuleReply         `yaml:"reply"`
	Events     []yamlRuleAction      `yaml:"events,omitempty"`
	Actions    []yamlRuleAction      `yaml:"actions,omitempty"`
}

type yamlRuleMatch struct {
	Stream   int    `yaml:"stream"`
	Function int    `yaml:"function"`
	RCMD     string `yaml:"rcmd"`
}

type yamlRuleReply struct {
	Stream   int `yaml:"stream"`
	Function int `yaml:"function"`
	Ack      int `yaml:"ack"`
}

type yamlRuleAction struct {
	DelayMS int                    `yaml:"delay_ms"`
	Type    string                 `yaml:"type"`
	DataID  string                 `yaml:"data_id,omitempty"`
	CEID    string                 `yaml:"ceid,omitempty"`
	Reports []yamlRuleActionReport `yaml:"reports,omitempty"`
	Target  string                 `yaml:"target,omitempty"`
	Value   string                 `yaml:"value,omitempty"`
}

type yamlRuleActionReport struct {
	RPTID           string                   `yaml:"rptid,omitempty"`
	Values          []string                 `yaml:"values,omitempty"`
	LegacyVariables []yamlRuleActionVariable `yaml:"variables,omitempty"`
}

type yamlRuleActionVariable struct {
	VID   string `yaml:"vid,omitempty"`
	Value string `yaml:"value,omitempty"`
}

type yamlHandshakeConfig struct {
	AutoS1F13       bool `yaml:"auto_s1f13"`
	AutoS1F1        bool `yaml:"auto_s1f1"`
	AutoS2F25       bool `yaml:"auto_s2f25"`
	AutoHostStartup bool `yaml:"auto_host_startup"`
}

func loadSnapshotFromYAML(path string, base model.Snapshot) (model.Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Snapshot{}, err
	}

	config := snapshotConfig(base)
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return model.Snapshot{}, fmt.Errorf("%w: decode YAML config %s: %v", ErrInvalidConfig, path, err)
	}

	snapshot := model.CloneSnapshot(base)
	snapshot.Runtime.ConfigFile = path
	snapshot.Runtime.LastError = ""
	snapshot.HSMS = model.HsmsConfig{
		Mode:      config.HSMS.Mode,
		IP:        config.HSMS.IP,
		Port:      config.HSMS.Port,
		SessionID: config.HSMS.SessionID,
		DeviceID:  config.HSMS.DeviceID,
		Timers:    config.HSMS.Timers,
		Handshake: model.HandshakeConfig{
			AutoS1F13:       config.Handshake.AutoS1F13,
			AutoS1F1:        config.Handshake.AutoS1F1,
			AutoS2F25:       config.Handshake.AutoS2F25,
			AutoHostStartup: config.Handshake.AutoHostStartup,
		},
	}
	snapshot.Device = config.Device
	snapshot.State = normalizeState(config.InitialState)
	snapshot.Rules = make([]model.Rule, 0, len(config.Rules))

	actionID := 1
	for index, ruleConfig := range config.Rules {
		enabled := true
		if ruleConfig.Enabled != nil {
			enabled = *ruleConfig.Enabled
		}

		rule := model.Rule{
			ID:         fmt.Sprintf("rule-%d", index+1),
			Name:       ruleConfig.Name,
			Enabled:    enabled,
			Match:      model.RuleMatch(ruleConfig.Match),
			Conditions: append(make([]model.RuleCondition, 0, len(ruleConfig.Conditions)), ruleConfig.Conditions...),
			Reply:      model.RuleReply(ruleConfig.Reply),
			Actions:    make([]model.RuleAction, 0, len(ruleConfig.ruleActions())),
		}
		if rule.Name == "" {
			rule.Name = "unnamed rule"
		}

		for _, actionConfig := range ruleConfig.ruleActions() {
			rule.Actions = append(rule.Actions, model.RuleAction{
				ID:      fmt.Sprintf("action-%d", actionID),
				DelayMS: actionConfig.DelayMS,
				Type:    actionConfig.Type,
				DataID:  actionConfig.DataID,
				CEID:    actionConfig.CEID,
				Reports: ruleActionReportsFromYAML(actionConfig.Reports),
				Target:  actionConfig.Target,
				Value:   actionConfig.Value,
			})
			actionID++
		}
		model.SortActions(rule.Actions)
		snapshot.Rules = append(snapshot.Rules, rule)
	}

	return snapshot, nil
}

func writeSnapshotToYAML(path string, snapshot model.Snapshot) error {
	config := snapshotConfig(snapshot)
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("%w: encode YAML config %s: %v", ErrInvalidConfig, path, err)
	}

	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create config directory %s: %w", dir, err)
		}
	}

	tempFile, err := os.CreateTemp(dir, ".secsim-config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp config file for %s: %w", path, err)
	}

	tempPath := tempFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write temp config file for %s: %w", path, err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp config file for %s: %w", path, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace config file %s: %w", path, err)
	}

	cleanup = false
	return nil
}

func configEquals(left model.Snapshot, right model.Snapshot) bool {
	return reflect.DeepEqual(snapshotConfig(left), snapshotConfig(right))
}

func snapshotConfig(snapshot model.Snapshot) yamlConfig {
	rules := make([]yamlRule, 0, len(snapshot.Rules))
	for _, rule := range snapshot.Rules {
		enabled := rule.Enabled
		ruleConfig := yamlRule{
			Name:       rule.Name,
			Enabled:    &enabled,
			Match:      yamlRuleMatch(rule.Match),
			Conditions: append(make([]model.RuleCondition, 0, len(rule.Conditions)), rule.Conditions...),
			Reply:      yamlRuleReply(rule.Reply),
			Events:     make([]yamlRuleAction, 0, len(rule.Actions)),
		}

		for _, action := range rule.Actions {
			ruleConfig.Events = append(ruleConfig.Events, yamlRuleAction{
				DelayMS: action.DelayMS,
				Type:    action.Type,
				DataID:  action.DataID,
				CEID:    action.CEID,
				Reports: ruleActionReportsToYAML(action.Reports),
				Target:  action.Target,
				Value:   action.Value,
			})
		}
		rules = append(rules, ruleConfig)
	}

	return yamlConfig{
		HSMS: yamlHSMSConfig{
			Mode:      snapshot.HSMS.Mode,
			IP:        snapshot.HSMS.IP,
			Port:      snapshot.HSMS.Port,
			SessionID: snapshot.HSMS.SessionID,
			DeviceID:  snapshot.HSMS.DeviceID,
			Timers:    snapshot.HSMS.Timers,
		},
		Device: snapshot.Device,
		Handshake: yamlHandshakeConfig{
			AutoS1F13:       snapshot.HSMS.Handshake.AutoS1F13,
			AutoS1F1:        snapshot.HSMS.Handshake.AutoS1F1,
			AutoS2F25:       snapshot.HSMS.Handshake.AutoS2F25,
			AutoHostStartup: snapshot.HSMS.Handshake.AutoHostStartup,
		},
		InitialState: normalizeState(snapshot.State),
		Rules:        rules,
	}
}

func normalizeState(state model.StateSnapshot) model.StateSnapshot {
	normalized := model.StateSnapshot{
		Mode:     state.Mode,
		Ports:    make(map[string]string, len(state.Ports)),
		Carriers: make(map[string]model.CarrierState, len(state.Carriers)),
	}

	for key, value := range state.Ports {
		normalized.Ports[key] = value
	}
	for key, value := range state.Carriers {
		normalized.Carriers[key] = value
	}

	return normalized
}

func (r yamlRule) ruleActions() []yamlRuleAction {
	if len(r.Events) > 0 || len(r.Actions) == 0 {
		return r.Events
	}

	return r.Actions
}

func ruleActionReportsFromYAML(src []yamlRuleActionReport) []model.RuleActionReport {
	reports := make([]model.RuleActionReport, 0, len(src))
	for _, report := range src {
		reports = append(reports, model.RuleActionReport{
			RPTID:  report.RPTID,
			Values: ruleActionValuesFromYAML(report),
		})
	}

	return reports
}

func ruleActionValuesFromYAML(report yamlRuleActionReport) []string {
	if len(report.Values) > 0 || len(report.LegacyVariables) == 0 {
		return append(make([]string, 0, len(report.Values)), report.Values...)
	}

	values := make([]string, 0, len(report.LegacyVariables))
	for _, variable := range report.LegacyVariables {
		values = append(values, variable.Value)
	}

	return values
}

func ruleActionReportsToYAML(src []model.RuleActionReport) []yamlRuleActionReport {
	reports := make([]yamlRuleActionReport, 0, len(src))
	for _, report := range src {
		reports = append(reports, yamlRuleActionReport{
			RPTID:  report.RPTID,
			Values: append(make([]string, 0, len(report.Values)), report.Values...),
		})
	}

	return reports
}

func nextIdentifierValue(prefix string, ids []string) int {
	next := 1
	for _, id := range ids {
		if !strings.HasPrefix(id, prefix) {
			continue
		}

		value, err := strconv.Atoi(strings.TrimPrefix(id, prefix))
		if err != nil {
			continue
		}
		if value >= next {
			next = value + 1
		}
	}

	return next
}

func ruleIDs(rules []model.Rule) []string {
	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}
	return ids
}

func actionIDs(rules []model.Rule) []string {
	var ids []string
	for _, rule := range rules {
		for _, action := range rule.Actions {
			ids = append(ids, action.ID)
		}
	}
	return ids
}
