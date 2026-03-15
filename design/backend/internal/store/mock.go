package store

import "secsim/design/backend/internal/model"

func seedSnapshot() model.Snapshot {
	return model.Snapshot{
		Runtime: model.RuntimeState{
			Listening:       false,
			HSMSState:       "NOT CONNECTED",
			ConfigFile:      "stocker-sim.yaml",
			Dirty:           false,
			RestartRequired: false,
			LastError:       "",
		},
		HSMS: model.HsmsConfig{
			Mode:      "passive",
			IP:        "0.0.0.0",
			Port:      5000,
			SessionID: 1,
			DeviceID:  0,
			Timers: model.HsmsTimers{
				T3: 45,
				T5: 10,
				T6: 5,
				T7: 10,
				T8: 5,
			},
			Handshake: model.HandshakeConfig{
				AutoS1F13:       true,
				AutoS1F1:        true,
				AutoS2F25:       false,
				AutoHostStartup: false,
			},
		},
		Device: model.DeviceConfig{
			Name:     "stocker-A",
			Protocol: "e88",
			MDLN:     "STOCKER-SIM",
			SoftRev:  "1.0.0",
		},
		Rules: []model.Rule{
			{
				ID:      "rule-1",
				Name:    "accept transfer",
				Enabled: true,
				Match: model.RuleMatch{
					Stream:   2,
					Function: 41,
					RCMD:     "TRANSFER",
				},
				Conditions: []model.RuleCondition{
					{Field: "source_equals", Value: "LP01"},
				},
				Reply: model.RuleReply{
					Stream:   2,
					Function: 42,
					Ack:      0,
				},
				Actions: []model.RuleAction{
					{ID: "action-1", DelayMS: 300, Type: "send", Stream: 6, Function: 11, WBit: true, Body: "L:1 <A \"TRANSFER_INITIATED\">"},
					{ID: "action-2", DelayMS: 1200, Type: "send", Stream: 6, Function: 11, WBit: true, Body: "L:1 <A \"TRANSFER_COMPLETED\">"},
				},
			},
			{
				ID:      "rule-2",
				Name:    "reject when blocked",
				Enabled: true,
				Match: model.RuleMatch{
					Stream:   2,
					Function: 41,
					RCMD:     "TRANSFER",
				},
				Conditions: []model.RuleCondition{
					{Field: "source_equals", Value: "LP02"},
				},
				Reply: model.RuleReply{
					Stream:   2,
					Function: 42,
					Ack:      3,
				},
				Actions: []model.RuleAction{},
			},
			{
				ID:      "rule-3",
				Name:    "carrier locate",
				Enabled: false,
				Match: model.RuleMatch{
					Stream:   2,
					Function: 41,
					RCMD:     "LOCATE",
				},
				Conditions: []model.RuleCondition{},
				Reply: model.RuleReply{
					Stream:   2,
					Function: 42,
					Ack:      0,
				},
				Actions: []model.RuleAction{
					{ID: "action-3", DelayMS: 200, Type: "send", Stream: 6, Function: 11, WBit: true, Body: "L:1 <A \"LOCATE_COMPLETE\">"},
				},
			},
		},
		Messages: []model.MessageRecord{
			{
				ID:        "msg-1",
				Timestamp: "14:32:01.003",
				Direction: "IN",
				SF:        "S1F13",
				Label:     "Establish Comm",
				Detail: model.MessageDetail{
					Stream:   1,
					Function: 13,
					WBit:     true,
					Body:     "L:0",
					RawSML:   "S1F13 W L:0",
				},
			},
			{
				ID:        "msg-2",
				Timestamp: "14:32:01.015",
				Direction: "OUT",
				SF:        "S1F14",
				Label:     "Establish Comm Ack",
				Detail: model.MessageDetail{
					Stream:   1,
					Function: 14,
					WBit:     false,
					Body:     "L:2\n  <B 0x00>\n  L:2\n    <A \"MDLN\">\n    <A \"1.0\">",
					RawSML:   "S1F14 L:2 <B 0x00> L:2 <A \"MDLN\"> <A \"1.0\">",
				},
			},
			{
				ID:            "msg-3",
				Timestamp:     "14:32:05.210",
				Direction:     "IN",
				SF:            "S2F41",
				Label:         "Remote Command: TRANSFER",
				MatchedRule:   "accept transfer",
				MatchedRuleID: "rule-1",
				Detail: model.MessageDetail{
					Stream:   2,
					Function: 41,
					WBit:     true,
					Body:     "L:2\n  <A \"TRANSFER\">\n  L:2\n    L:2 <A \"SourcePort\"> <A \"LP01\">",
					RawSML:   "S2F41 W L:2 <A \"TRANSFER\"> L:2 L:2 <A \"SourcePort\"> <A \"LP01\">",
				},
				Evaluations: []model.ConditionEvaluation{
					{Field: "source_equals", Expected: "LP01", Actual: "LP01", Passed: true},
				},
			},
			{
				ID:            "msg-4",
				Timestamp:     "14:32:05.215",
				Direction:     "OUT",
				SF:            "S2F42",
				Label:         "Remote Cmd Ack",
				MatchedRule:   "accept transfer",
				MatchedRuleID: "rule-1",
				Detail: model.MessageDetail{
					Stream:   2,
					Function: 42,
					WBit:     false,
					Body:     "L:2\n  <B 0x00>\n  L:0",
					RawSML:   "S2F42 L:2 <B 0x00> L:0",
				},
				Evaluations: []model.ConditionEvaluation{},
			},
			{
				ID:            "msg-5",
				Timestamp:     "14:32:05.515",
				Direction:     "OUT",
				SF:            "S6F11",
				Label:         "TRANSFER_INITIATED",
				MatchedRule:   "accept transfer",
				MatchedRuleID: "rule-1",
				Detail: model.MessageDetail{
					Stream:   6,
					Function: 11,
					WBit:     true,
					Body:     "L:1\n  <A \"TRANSFER_INITIATED\">",
					RawSML:   "S6F11 W L:1 <A \"TRANSFER_INITIATED\">",
				},
				Evaluations: []model.ConditionEvaluation{},
			},
			{
				ID:            "msg-6",
				Timestamp:     "14:32:06.415",
				Direction:     "OUT",
				SF:            "S6F11",
				Label:         "TRANSFER_COMPLETED",
				MatchedRule:   "accept transfer",
				MatchedRuleID: "rule-1",
				Detail: model.MessageDetail{
					Stream:   6,
					Function: 11,
					WBit:     true,
					Body:     "L:1\n  <A \"TRANSFER_COMPLETED\">",
					RawSML:   "S6F11 W L:1 <A \"TRANSFER_COMPLETED\">",
				},
				Evaluations: []model.ConditionEvaluation{},
			},
			{
				ID:        "msg-7",
				Timestamp: "14:32:06.420",
				Direction: "IN",
				SF:        "S6F12",
				Label:     "Event Ack",
				Detail: model.MessageDetail{
					Stream:   6,
					Function: 12,
					WBit:     false,
					Body:     "<B 0x00>",
					RawSML:   "S6F12 <B 0x00>",
				},
				Evaluations: []model.ConditionEvaluation{},
			},
		},
	}
}
