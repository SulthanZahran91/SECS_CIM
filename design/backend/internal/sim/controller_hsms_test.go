package sim

import (
	"bytes"
	"log"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"secsim/design/backend/internal/hsms"
	"secsim/design/backend/internal/model"
	"secsim/design/backend/internal/store"
)

func TestControllerPassiveHSMSSessionDrivesAutoResponsesAndRules(t *testing.T) {
	state := store.New()
	state.ClearLog()

	hsmsConfig := state.ConfigSnapshot().HSMS
	hsmsConfig.Mode = "passive"
	hsmsConfig.IP = "127.0.0.1"
	hsmsConfig.Port = freePort(t)
	state.UpdateHSMS(hsmsConfig)

	rule := state.ConfigSnapshot().Rules[0]
	rule.Actions = []model.RuleAction{
		{
			ID:       "action-1",
			DelayMS:  20,
			Type:     "send",
			Stream:   6,
			Function: 11,
			WBit:     true,
			Body:     "L:2 <A \"TRANSFER_INITIATED\"> <I 7>",
		},
		{ID: "action-2", DelayMS: 40, Type: "send", Stream: 6, Function: 11, WBit: true, Body: "L:1 <A \"TRANSFER_COMPLETED\">"},
	}
	if _, err := state.UpdateRule(rule); err != nil {
		t.Fatalf("update rule timings: %v", err)
	}

	controller := New(state)
	started, err := controller.Start()
	if err != nil {
		t.Fatalf("start controller: %v", err)
	}
	defer controller.Stop()

	if !started.Runtime.Listening || started.Runtime.HSMSState != "LISTENING" {
		t.Fatalf("expected passive controller to enter LISTENING, got %#v", started.Runtime)
	}

	conn := dialEventually(t, hsmsConfig.Port)
	defer conn.Close()

	if err := hsms.WriteFrame(conn, hsms.NewControlFrame(uint16(hsmsConfig.SessionID), 0x01020304, hsms.STypeSelectReq, 0)); err != nil {
		t.Fatalf("write select.req: %v", err)
	}
	selectRsp := readFrame(t, conn)
	if selectRsp.SType != hsms.STypeSelectRsp || selectRsp.ControlCode != hsms.SelectStatusSuccess {
		t.Fatalf("unexpected select response: %#v", selectRsp)
	}
	waitFor(t, 500*time.Millisecond, func() bool {
		return state.Snapshot().Runtime.HSMSState == "SELECTED"
	})

	establish := hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      1,
		Function:    13,
		WBit:        true,
		SystemBytes: 0x01020305,
		Body:        itemPtr(hsms.List()),
	}
	writeMessage(t, conn, establish)
	establishAck := readMessage(t, conn)
	if establishAck.Stream != 1 || establishAck.Function != 14 {
		t.Fatalf("expected S1F14 auto-response, got %#v", establishAck)
	}

	command := hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      2,
		Function:    41,
		WBit:        true,
		SystemBytes: 0x01020306,
		Body: itemPtr(hsms.List(
			hsms.ASCII("TRANSFER"),
			hsms.List(
				hsms.List(hsms.ASCII("CarrierID"), hsms.ASCII("CARR001")),
				hsms.List(hsms.ASCII("SourcePort"), hsms.ASCII("LP01")),
			),
		)),
	}
	writeMessage(t, conn, command)

	reply := readMessage(t, conn)
	if reply.Stream != 2 || reply.Function != 42 {
		t.Fatalf("expected S2F42 rule reply, got %#v", reply)
	}

	firstEvent := readMessage(t, conn)
	if firstEvent.Stream != 6 || firstEvent.Function != 11 {
		t.Fatalf("expected S6F11 event, got %#v", firstEvent)
	}
	if firstEvent.Body == nil || firstEvent.Body.Type != hsms.ItemList || len(firstEvent.Body.Children) != 2 {
		t.Fatalf("expected generic outbound SML body, got %#v", firstEvent.Body)
	}
	if got := firstEvent.Body.Children[0].ScalarValue(); got != "TRANSFER_INITIATED" {
		t.Fatalf("expected first generic item TRANSFER_INITIATED, got %q", got)
	}
	if got := firstEvent.Body.Children[1].ScalarValue(); got != "7" {
		t.Fatalf("expected second generic item 7, got %q", got)
	}

	secondEvent := readMessage(t, conn)
	if ceid, ok := hsms.ExtractS6F11CEID(secondEvent); !ok || ceid != "TRANSFER_COMPLETED" {
		t.Fatalf("expected TRANSFER_COMPLETED event body, got %#v", secondEvent)
	}

	waitFor(t, time.Second, func() bool {
		return len(state.Snapshot().Messages) >= 6
	})

	snapshot := state.Snapshot()
	if len(snapshot.Messages) != 6 {
		t.Fatalf("expected 6 logged protocol messages, got %d", len(snapshot.Messages))
	}
	if snapshot.Messages[0].SF != "S1F13" || snapshot.Messages[1].SF != "S1F14" {
		t.Fatalf("expected auto-response log entries, got %#v", snapshot.Messages[:2])
	}
	if snapshot.Messages[2].MatchedRuleID != "rule-1" || snapshot.Messages[3].SF != "S2F42" {
		t.Fatalf("expected rule match and reply logs, got %#v", snapshot.Messages[2:4])
	}
}

func TestControllerPassiveHSMSSessionHandlesControlMessages(t *testing.T) {
	traceOutput := captureLogs(t)

	state := store.New()
	state.ClearLog()

	hsmsConfig := state.ConfigSnapshot().HSMS
	hsmsConfig.Mode = "passive"
	hsmsConfig.IP = "127.0.0.1"
	hsmsConfig.Port = freePort(t)
	state.UpdateHSMS(hsmsConfig)

	controller := New(state)
	started, err := controller.Start()
	if err != nil {
		t.Fatalf("start controller: %v", err)
	}
	defer controller.Stop()

	if !started.Runtime.Listening || started.Runtime.HSMSState != "LISTENING" {
		t.Fatalf("expected passive controller to enter LISTENING, got %#v", started.Runtime)
	}

	conn := dialEventually(t, hsmsConfig.Port)
	defer conn.Close()

	if err := hsms.WriteFrame(conn, hsms.NewControlFrame(uint16(hsmsConfig.SessionID), 0x01010101, hsms.STypeSelectReq, 0)); err != nil {
		t.Fatalf("write initial select.req: %v", err)
	}
	selectRsp := readFrame(t, conn)
	if selectRsp.SType != hsms.STypeSelectRsp || selectRsp.ControlCode != hsms.SelectStatusSuccess {
		t.Fatalf("unexpected initial select response: %#v", selectRsp)
	}
	waitFor(t, 500*time.Millisecond, func() bool {
		return state.Snapshot().Runtime.HSMSState == "SELECTED"
	})

	if err := hsms.WriteFrame(conn, hsms.NewControlFrame(uint16(hsmsConfig.SessionID), 0x01010102, hsms.STypeLinktestReq, 0)); err != nil {
		t.Fatalf("write linktest.req: %v", err)
	}
	linktestRsp := readFrame(t, conn)
	if linktestRsp.SType != hsms.STypeLinktestRsp {
		t.Fatalf("expected linktest.rsp, got %#v", linktestRsp)
	}

	if err := hsms.WriteFrame(conn, hsms.NewControlFrame(uint16(hsmsConfig.SessionID), 0x01010103, hsms.STypeDeselectReq, 0)); err != nil {
		t.Fatalf("write deselect.req: %v", err)
	}
	deselectRsp := readFrame(t, conn)
	if deselectRsp.SType != hsms.STypeDeselectRsp {
		t.Fatalf("expected deselect.rsp, got %#v", deselectRsp)
	}
	waitFor(t, 500*time.Millisecond, func() bool {
		return state.Snapshot().Runtime.HSMSState == "CONNECTED"
	})

	if err := hsms.WriteFrame(conn, hsms.NewControlFrame(uint16(hsmsConfig.SessionID), 0x01010104, hsms.STypeSelectReq, 0)); err != nil {
		t.Fatalf("write second select.req: %v", err)
	}
	secondSelectRsp := readFrame(t, conn)
	if secondSelectRsp.SType != hsms.STypeSelectRsp || secondSelectRsp.ControlCode != hsms.SelectStatusSuccess {
		t.Fatalf("unexpected second select response: %#v", secondSelectRsp)
	}
	waitFor(t, 500*time.Millisecond, func() bool {
		return state.Snapshot().Runtime.HSMSState == "SELECTED"
	})

	if err := hsms.WriteFrame(conn, hsms.NewControlFrame(uint16(hsmsConfig.SessionID), 0x01010105, hsms.STypeSeparateReq, 0)); err != nil {
		t.Fatalf("write separate.req: %v", err)
	}
	waitFor(t, time.Second, func() bool {
		return state.Snapshot().Runtime.HSMSState == "LISTENING"
	})

	assertLogContains(t, traceOutput, "HSMS tcp accepted")
	assertLogContains(t, traceOutput, "HSMS control IN Select.req")
	assertLogContains(t, traceOutput, "HSMS control OUT Select.rsp")
	assertLogContains(t, traceOutput, "HSMS control IN Linktest.req")
	assertLogContains(t, traceOutput, "HSMS control OUT Linktest.rsp")
	assertLogContains(t, traceOutput, "HSMS control IN Separate.req")
	assertLogContains(t, traceOutput, "HSMS tcp closed")
}

func TestControllerActiveHSMSSessionReconnectsAfterDisconnect(t *testing.T) {
	traceOutput := captureLogs(t)

	state := store.New()
	state.ClearLog()

	hostListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for active host: %v", err)
	}
	defer hostListener.Close()

	hostPort := hostListener.Addr().(*net.TCPAddr).Port
	hsmsConfig := state.ConfigSnapshot().HSMS
	hsmsConfig.Mode = "active"
	hsmsConfig.IP = "127.0.0.1"
	hsmsConfig.Port = hostPort
	hsmsConfig.Timers.T5 = 1
	hsmsConfig.Timers.T6 = 1
	state.UpdateHSMS(hsmsConfig)

	controller := New(state)
	started, err := controller.Start()
	if err != nil {
		t.Fatalf("start controller: %v", err)
	}
	defer controller.Stop()

	if !started.Runtime.Listening || started.Runtime.HSMSState != "CONNECTING" {
		t.Fatalf("expected active controller to start CONNECTING, got %#v", started.Runtime)
	}

	firstConn := acceptEventually(t, hostListener)
	firstSelectReq := readFrame(t, firstConn)
	if firstSelectReq.SType != hsms.STypeSelectReq {
		t.Fatalf("expected active select.req, got %#v", firstSelectReq)
	}
	if err := hsms.WriteFrame(firstConn, hsms.NewControlFrame(uint16(hsmsConfig.SessionID), firstSelectReq.SystemBytes, hsms.STypeSelectRsp, hsms.SelectStatusSuccess)); err != nil {
		t.Fatalf("write first select.rsp: %v", err)
	}
	waitFor(t, time.Second, func() bool {
		return state.Snapshot().Runtime.HSMSState == "SELECTED"
	})

	if err := firstConn.Close(); err != nil {
		t.Fatalf("close first host connection: %v", err)
	}

	waitFor(t, time.Second, func() bool {
		snapshot := state.Snapshot()
		return snapshot.Runtime.HSMSState == "CONNECTING" && snapshot.Runtime.LastError != ""
	})

	secondConn := acceptEventually(t, hostListener)
	defer secondConn.Close()

	secondSelectReq := readFrame(t, secondConn)
	if secondSelectReq.SType != hsms.STypeSelectReq {
		t.Fatalf("expected reconnect select.req, got %#v", secondSelectReq)
	}
	if err := hsms.WriteFrame(secondConn, hsms.NewControlFrame(uint16(hsmsConfig.SessionID), secondSelectReq.SystemBytes, hsms.STypeSelectRsp, hsms.SelectStatusSuccess)); err != nil {
		t.Fatalf("write second select.rsp: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		snapshot := state.Snapshot()
		return snapshot.Runtime.HSMSState == "SELECTED" && snapshot.Runtime.LastError == ""
	})

	assertLogContains(t, traceOutput, "HSMS tcp connected")
	assertLogContains(t, traceOutput, "HSMS control OUT Select.req")
	assertLogContains(t, traceOutput, "HSMS control IN Select.rsp")
}

func TestControllerActiveHostStartupStockerBootstrapsAfterSelect(t *testing.T) {
	state := store.New()
	state.ClearLog()

	hostListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for active host bootstrap test: %v", err)
	}
	defer hostListener.Close()

	hostPort := hostListener.Addr().(*net.TCPAddr).Port
	hsmsConfig := state.ConfigSnapshot().HSMS
	hsmsConfig.Mode = "active"
	hsmsConfig.IP = "127.0.0.1"
	hsmsConfig.Port = hostPort
	hsmsConfig.Timers.T5 = 1
	hsmsConfig.Timers.T6 = 1
	hsmsConfig.Handshake.AutoHostStartup = true
	hsmsConfig.Handshake.HostStartupProfile = model.HostStartupProfileStocker
	state.UpdateHSMS(hsmsConfig)

	controller := New(state)
	started, err := controller.Start()
	if err != nil {
		t.Fatalf("start controller: %v", err)
	}
	defer controller.Stop()

	if !started.Runtime.Listening || started.Runtime.HSMSState != "CONNECTING" {
		t.Fatalf("expected active controller to start CONNECTING, got %#v", started.Runtime)
	}

	conn := acceptEventually(t, hostListener)
	defer conn.Close()

	selectReq := readFrame(t, conn)
	if selectReq.SType != hsms.STypeSelectReq {
		t.Fatalf("expected active select.req, got %#v", selectReq)
	}
	if err := hsms.WriteFrame(conn, hsms.NewControlFrame(uint16(hsmsConfig.SessionID), selectReq.SystemBytes, hsms.STypeSelectRsp, hsms.SelectStatusSuccess)); err != nil {
		t.Fatalf("write select.rsp: %v", err)
	}

	establishReq := readMessage(t, conn)
	if establishReq.Stream != 1 || establishReq.Function != 13 || !establishReq.WBit {
		t.Fatalf("expected S1F13 bootstrap request, got %#v", establishReq)
	}

	writeMessage(t, conn, hsms.BuildS1F14(uint16(hsmsConfig.SessionID), establishReq.SystemBytes, "EQP-01", "1.2.3"))

	onlineReq := readMessage(t, conn)
	if onlineReq.Stream != 1 || onlineReq.Function != 17 || !onlineReq.WBit {
		t.Fatalf("expected S1F17 bootstrap request, got %#v", onlineReq)
	}

	s1f18Body := itemPtr(hsms.Binary(0x00))
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      1,
		Function:    18,
		WBit:        false,
		SystemBytes: onlineReq.SystemBytes,
		Body:        s1f18Body,
	})

	timeSetReq := readMessage(t, conn)
	if timeSetReq.Stream != 2 || timeSetReq.Function != 31 || !timeSetReq.WBit || timeSetReq.Body == nil || timeSetReq.Body.Type != hsms.ItemASCII {
		t.Fatalf("expected S2F31 bootstrap request, got %#v", timeSetReq)
	}
	if len(timeSetReq.Body.Text) != 16 {
		t.Fatalf("expected 16-char S2F31 timestamp, got %q", timeSetReq.Body.Text)
	}
	for _, ch := range timeSetReq.Body.Text {
		if ch < '0' || ch > '9' {
			t.Fatalf("expected numeric S2F31 timestamp, got %q", timeSetReq.Body.Text)
		}
	}

	s2f32Body := itemPtr(hsms.Binary(0x00))
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      2,
		Function:    32,
		WBit:        false,
		SystemBytes: timeSetReq.SystemBytes,
		Body:        s2f32Body,
	})

	eventBody := itemPtr(hsms.List(
		hsms.U4(0),
		hsms.U2(3),
		hsms.List(
			hsms.List(
				hsms.U2(1),
				hsms.List(),
			),
		),
	))
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      6,
		Function:    11,
		WBit:        true,
		SystemBytes: 0x0000BEEF,
		Body:        eventBody,
	})

	eventAck := readMessage(t, conn)
	if eventAck.Stream != 6 || eventAck.Function != 12 || eventAck.SystemBytes != 0x0000BEEF {
		t.Fatalf("expected S6F12 event ack, got %#v", eventAck)
	}

	waitFor(t, time.Second, func() bool {
		messages := state.Snapshot().Messages
		if len(messages) < 8 {
			return false
		}
		got := []string{
			messages[0].SF,
			messages[1].SF,
			messages[2].SF,
			messages[3].SF,
			messages[4].SF,
			messages[5].SF,
			messages[6].SF,
			messages[7].SF,
		}
		expected := []string{"S1F13", "S1F14", "S1F17", "S1F18", "S2F31", "S2F32", "S6F11", "S6F12"}
		for index := range expected {
			if got[index] != expected[index] {
				return false
			}
		}
		return true
	})
}

func TestControllerActiveHostStartupConveyorBootstrapsAfterSelect(t *testing.T) {
	state := store.New()
	state.ClearLog()

	hostListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for active conveyor bootstrap test: %v", err)
	}
	defer hostListener.Close()

	hsmsConfig := state.ConfigSnapshot().HSMS
	hsmsConfig.Mode = "active"
	hsmsConfig.IP = "127.0.0.1"
	hsmsConfig.Port = hostListener.Addr().(*net.TCPAddr).Port
	hsmsConfig.Timers.T5 = 1
	hsmsConfig.Timers.T6 = 1
	hsmsConfig.Handshake.AutoHostStartup = true
	hsmsConfig.Handshake.HostStartupProfile = model.HostStartupProfileConveyor
	state.UpdateHSMS(hsmsConfig)

	controller := New(state)
	started, err := controller.Start()
	if err != nil {
		t.Fatalf("start controller: %v", err)
	}
	defer controller.Stop()

	if !started.Runtime.Listening || started.Runtime.HSMSState != "CONNECTING" {
		t.Fatalf("expected active controller to start CONNECTING, got %#v", started.Runtime)
	}

	conn := acceptEventually(t, hostListener)
	defer conn.Close()

	selectReq := readFrame(t, conn)
	if selectReq.SType != hsms.STypeSelectReq {
		t.Fatalf("expected active select.req, got %#v", selectReq)
	}
	if err := hsms.WriteFrame(conn, hsms.NewControlFrame(uint16(hsmsConfig.SessionID), selectReq.SystemBytes, hsms.STypeSelectRsp, hsms.SelectStatusSuccess)); err != nil {
		t.Fatalf("write select.rsp: %v", err)
	}

	establishReq := readMessage(t, conn)
	if establishReq.Stream != 1 || establishReq.Function != 13 || !establishReq.WBit {
		t.Fatalf("expected S1F13 bootstrap request, got %#v", establishReq)
	}
	writeMessage(t, conn, hsms.BuildS1F14(uint16(hsmsConfig.SessionID), establishReq.SystemBytes, "EQP-01", "1.2.3"))

	onlineReq := readMessage(t, conn)
	if onlineReq.Stream != 1 || onlineReq.Function != 17 || !onlineReq.WBit {
		t.Fatalf("expected S1F17 bootstrap request, got %#v", onlineReq)
	}

	onlineEvent := hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      6,
		Function:    11,
		WBit:        true,
		SystemBytes: 0x0000BEEF,
		Body: itemPtr(hsms.List(
			hsms.U4(0),
			hsms.U2(3),
			hsms.List(
				hsms.List(
					hsms.U2(1),
					hsms.List(),
				),
			),
		)),
	}
	writeMessage(t, conn, onlineEvent)

	eventAck := readMessage(t, conn)
	if eventAck.Stream != 6 || eventAck.Function != 12 || eventAck.SystemBytes != onlineEvent.SystemBytes {
		t.Fatalf("expected S6F12 ack for interleaved online event, got %#v", eventAck)
	}

	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      1,
		Function:    18,
		WBit:        false,
		SystemBytes: onlineReq.SystemBytes,
		Body:        itemPtr(hsms.Binary(0x00)),
	})

	timeSetReq := readMessage(t, conn)
	if timeSetReq.Stream != 2 || timeSetReq.Function != 31 || !timeSetReq.WBit || timeSetReq.Body == nil || timeSetReq.Body.Type != hsms.ItemASCII {
		t.Fatalf("expected S2F31 bootstrap request, got %#v", timeSetReq)
	}
	if len(timeSetReq.Body.Text) != 16 {
		t.Fatalf("expected 16-char S2F31 timestamp, got %q", timeSetReq.Body.Text)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      2,
		Function:    32,
		WBit:        false,
		SystemBytes: timeSetReq.SystemBytes,
		Body:        itemPtr(hsms.Binary(0x00)),
	})

	offlineReq := readMessage(t, conn)
	if offlineReq.Stream != 2 || offlineReq.Function != 15 || !offlineReq.WBit || offlineReq.Body == nil {
		t.Fatalf("expected S2F15 conveyor startup request, got %#v", offlineReq)
	}
	if got := offlineReq.Body.Compact(); got != `L:1 L:2 <U2 62> <A "B1ACNV15201">` {
		t.Fatalf("unexpected S2F15 body: %s", got)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      2,
		Function:    16,
		WBit:        false,
		SystemBytes: offlineReq.SystemBytes,
		Body:        itemPtr(hsms.Binary(0x00)),
	})

	disableReportsReq := readMessage(t, conn)
	if disableReportsReq.Stream != 2 || disableReportsReq.Function != 37 || !disableReportsReq.WBit || disableReportsReq.Body == nil {
		t.Fatalf("expected S2F37 disable request, got %#v", disableReportsReq)
	}
	if got := disableReportsReq.Body.Compact(); got != "L:2 <BOOLEAN FALSE> L:0" {
		t.Fatalf("unexpected first S2F37 body: %s", got)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      2,
		Function:    38,
		WBit:        false,
		SystemBytes: disableReportsReq.SystemBytes,
		Body:        itemPtr(hsms.Binary(0x00)),
	})

	resetReportsReq := readMessage(t, conn)
	if resetReportsReq.Stream != 2 || resetReportsReq.Function != 33 || !resetReportsReq.WBit || resetReportsReq.Body == nil {
		t.Fatalf("expected S2F33 reset request, got %#v", resetReportsReq)
	}
	if got := resetReportsReq.Body.Compact(); got != "L:2 <U4 1> L:0" {
		t.Fatalf("unexpected first S2F33 body: %s", got)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      2,
		Function:    34,
		WBit:        false,
		SystemBytes: resetReportsReq.SystemBytes,
		Body:        itemPtr(hsms.Binary(0x00)),
	})

	defineReportsReq := readMessage(t, conn)
	if defineReportsReq.Stream != 2 || defineReportsReq.Function != 33 || !defineReportsReq.WBit || defineReportsReq.Body == nil {
		t.Fatalf("expected S2F33 define-report request, got %#v", defineReportsReq)
	}
	if defineReportsReq.Body.Type != hsms.ItemList || len(defineReportsReq.Body.Children) != 2 {
		t.Fatalf("unexpected S2F33 define-report shape: %#v", defineReportsReq.Body)
	}
	reportList := defineReportsReq.Body.Children[1]
	if reportList.Type != hsms.ItemList || len(reportList.Children) != len(conveyorReportDefinitions) {
		t.Fatalf("expected %d report definitions, got %#v", len(conveyorReportDefinitions), reportList)
	}
	firstReport := reportList.Children[0]
	lastReport := reportList.Children[len(reportList.Children)-1]
	if firstReport.Children[0].Uint16 != 1 || lastReport.Children[0].Uint16 != 73 {
		t.Fatalf("unexpected report definition bounds: first=%#v last=%#v", firstReport, lastReport)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      2,
		Function:    34,
		WBit:        false,
		SystemBytes: defineReportsReq.SystemBytes,
		Body:        itemPtr(hsms.Binary(0x00)),
	})

	linkReportsReq := readMessage(t, conn)
	if linkReportsReq.Stream != 2 || linkReportsReq.Function != 35 || !linkReportsReq.WBit || linkReportsReq.Body == nil {
		t.Fatalf("expected S2F35 link-report request, got %#v", linkReportsReq)
	}
	linkList := linkReportsReq.Body.Children[1]
	if linkList.Type != hsms.ItemList || len(linkList.Children) != len(conveyorLinkedCEIDs) {
		t.Fatalf("expected %d linked CEIDs, got %#v", len(conveyorLinkedCEIDs), linkList)
	}
	firstLink := linkList.Children[0]
	lastLink := linkList.Children[len(linkList.Children)-1]
	if firstLink.Children[0].Uint16 != 51 || firstLink.Children[1].Children[0].Uint16 != 1 {
		t.Fatalf("unexpected first S2F35 link: %#v", firstLink)
	}
	if lastLink.Children[0].Uint16 != 711 || lastLink.Children[1].Children[0].Uint16 != 73 {
		t.Fatalf("unexpected last S2F35 link: %#v", lastLink)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      2,
		Function:    36,
		WBit:        false,
		SystemBytes: linkReportsReq.SystemBytes,
		Body:        itemPtr(hsms.Binary(0x00)),
	})

	enableReportsReq := readMessage(t, conn)
	if enableReportsReq.Stream != 2 || enableReportsReq.Function != 37 || !enableReportsReq.WBit || enableReportsReq.Body == nil {
		t.Fatalf("expected second S2F37 request, got %#v", enableReportsReq)
	}
	if enableReportsReq.Body.Type != hsms.ItemList || len(enableReportsReq.Body.Children) != 2 {
		t.Fatalf("unexpected second S2F37 shape: %#v", enableReportsReq.Body)
	}
	enabledCEIDs := enableReportsReq.Body.Children[1]
	if enabledCEIDs.Type != hsms.ItemList || len(enabledCEIDs.Children) != len(conveyorEnabledCEIDs) {
		t.Fatalf("expected %d enabled CEIDs, got %#v", len(conveyorEnabledCEIDs), enabledCEIDs)
	}
	if enabledCEIDs.Children[0].Uint16 != 1 || enabledCEIDs.Children[len(enabledCEIDs.Children)-1].Uint16 != 711 {
		t.Fatalf("unexpected enabled CEID bounds: %#v", enabledCEIDs)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      2,
		Function:    38,
		WBit:        false,
		SystemBytes: enableReportsReq.SystemBytes,
		Body:        itemPtr(hsms.Binary(0x00)),
	})

	disableAlarmsReq := readMessage(t, conn)
	if disableAlarmsReq.Stream != 5 || disableAlarmsReq.Function != 3 || !disableAlarmsReq.WBit || disableAlarmsReq.Body == nil {
		t.Fatalf("expected first S5F3 request, got %#v", disableAlarmsReq)
	}
	if got := disableAlarmsReq.Body.Compact(); got != "L:2 <B 0x00> <U4 0>" {
		t.Fatalf("unexpected first S5F3 body: %s", got)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      5,
		Function:    4,
		WBit:        false,
		SystemBytes: disableAlarmsReq.SystemBytes,
		Body:        itemPtr(hsms.Binary(0x00)),
	})

	enableAlarmsReq := readMessage(t, conn)
	if enableAlarmsReq.Stream != 5 || enableAlarmsReq.Function != 3 || !enableAlarmsReq.WBit || enableAlarmsReq.Body == nil {
		t.Fatalf("expected second S5F3 request, got %#v", enableAlarmsReq)
	}
	if got := enableAlarmsReq.Body.Compact(); got != "L:2 <B 0x80> <U4 0>" {
		t.Fatalf("unexpected second S5F3 body: %s", got)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      5,
		Function:    4,
		WBit:        false,
		SystemBytes: enableAlarmsReq.SystemBytes,
		Body:        itemPtr(hsms.Binary(0x00)),
	})

	statusReq := readMessage(t, conn)
	if statusReq.Stream != 1 || statusReq.Function != 3 || !statusReq.WBit || statusReq.Body == nil {
		t.Fatalf("expected S1F3 status request, got %#v", statusReq)
	}
	if got := statusReq.Body.Compact(); got != "L:1 <U2 6>" {
		t.Fatalf("unexpected S1F3 body: %s", got)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      1,
		Function:    4,
		WBit:        false,
		SystemBytes: statusReq.SystemBytes,
		Body:        itemPtr(hsms.List(hsms.U1(5))),
	})

	pauseReq := readMessage(t, conn)
	if pauseReq.Stream != 2 || pauseReq.Function != 41 || !pauseReq.WBit || pauseReq.Body == nil {
		t.Fatalf("expected PAUSE S2F41 request, got %#v", pauseReq)
	}
	if got := pauseReq.Body.Compact(); got != `L:2 <A "PAUSE"> L:0` {
		t.Fatalf("unexpected PAUSE command body: %s", got)
	}
	writeMessage(t, conn, hsms.BuildS2F42(uint16(hsmsConfig.SessionID), pauseReq.SystemBytes, 0))

	pauseInitiated := hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      6,
		Function:    11,
		WBit:        true,
		SystemBytes: 0x0000CA02,
		Body: itemPtr(hsms.List(
			hsms.U2(0),
			hsms.U2(57),
			hsms.List(hsms.List(hsms.U2(1), hsms.List())),
		)),
	}
	writeMessage(t, conn, pauseInitiated)
	pauseInitiatedAck := readMessage(t, conn)
	if pauseInitiatedAck.Stream != 6 || pauseInitiatedAck.Function != 12 || pauseInitiatedAck.SystemBytes != pauseInitiated.SystemBytes {
		t.Fatalf("expected S6F12 ack for pause initiated event, got %#v", pauseInitiatedAck)
	}

	pauseCompleted := hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      6,
		Function:    11,
		WBit:        true,
		SystemBytes: 0x0000CA03,
		Body: itemPtr(hsms.List(
			hsms.U2(0),
			hsms.U2(55),
			hsms.List(hsms.List(hsms.U2(1), hsms.List())),
		)),
	}
	writeMessage(t, conn, pauseCompleted)
	pauseCompletedAck := readMessage(t, conn)
	if pauseCompletedAck.Stream != 6 || pauseCompletedAck.Function != 12 || pauseCompletedAck.SystemBytes != pauseCompleted.SystemBytes {
		t.Fatalf("expected S6F12 ack for pause completed event, got %#v", pauseCompletedAck)
	}

	for _, svid := range []uint16{98, 81, 83, 4, 401, 507, 509, 511, 76, 628, 631, 632} {
		statusReq = readMessage(t, conn)
		if statusReq.Stream != 1 || statusReq.Function != 3 || !statusReq.WBit || statusReq.Body == nil {
			t.Fatalf("expected S1F3 status request for SVID %d, got %#v", svid, statusReq)
		}
		if got := statusReq.Body.Compact(); got != "L:1 <U2 "+strconv.Itoa(int(svid))+">" {
			t.Fatalf("unexpected S1F3 body for SVID %d: %s", svid, got)
		}
		writeMessage(t, conn, hsms.Message{
			SessionID:   uint16(hsmsConfig.SessionID),
			Stream:      1,
			Function:    4,
			WBit:        false,
			SystemBytes: statusReq.SystemBytes,
			Body:        itemPtr(hsms.List()),
		})
	}

	resumeReq := readMessage(t, conn)
	if resumeReq.Stream != 2 || resumeReq.Function != 41 || !resumeReq.WBit || resumeReq.Body == nil {
		t.Fatalf("expected RESUME S2F41 request, got %#v", resumeReq)
	}
	if got := resumeReq.Body.Compact(); got != `L:2 <A "RESUME"> L:0` {
		t.Fatalf("unexpected RESUME command body: %s", got)
	}
	writeMessage(t, conn, hsms.BuildS2F42(uint16(hsmsConfig.SessionID), resumeReq.SystemBytes, 0))

	autoComplete := hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      6,
		Function:    11,
		WBit:        true,
		SystemBytes: 0x0000CA04,
		Body: itemPtr(hsms.List(
			hsms.U2(0),
			hsms.U2(53),
			hsms.List(hsms.List(hsms.U2(1), hsms.List())),
		)),
	}
	writeMessage(t, conn, autoComplete)
	autoCompleteAck := readMessage(t, conn)
	if autoCompleteAck.Stream != 6 || autoCompleteAck.Function != 12 || autoCompleteAck.SystemBytes != autoComplete.SystemBytes {
		t.Fatalf("expected S6F12 ack for auto complete event, got %#v", autoCompleteAck)
	}

	conveyorChange := hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      6,
		Function:    11,
		WBit:        true,
		SystemBytes: 0x0000CA05,
		Body: itemPtr(hsms.List(
			hsms.U2(0),
			hsms.U2(601),
			hsms.List(hsms.List(hsms.U2(23), hsms.List(hsms.U2(1)))),
		)),
	}
	writeMessage(t, conn, conveyorChange)
	conveyorChangeAck := readMessage(t, conn)
	if conveyorChangeAck.Stream != 6 || conveyorChangeAck.Function != 12 || conveyorChangeAck.SystemBytes != conveyorChange.SystemBytes {
		t.Fatalf("expected S6F12 ack for conveyor state change event, got %#v", conveyorChangeAck)
	}

	waitFor(t, time.Second, func() bool {
		messages := state.Snapshot().Messages
		return len(messages) >= 48 && messages[0].SF == "S1F13" && messages[len(messages)-1].SF == "S6F12"
	})
}

func TestControllerRestartClearsPendingConnectionChanges(t *testing.T) {
	state := store.New()

	hsmsConfig := state.ConfigSnapshot().HSMS
	hsmsConfig.Mode = "passive"
	hsmsConfig.IP = "127.0.0.1"
	hsmsConfig.Port = freePort(t)
	state.UpdateHSMS(hsmsConfig)

	controller := New(state)
	started, err := controller.Start()
	if err != nil {
		t.Fatalf("start controller: %v", err)
	}

	if started.Runtime.RestartRequired {
		t.Fatalf("expected fresh start to have no pending restart")
	}

	changed := hsmsConfig
	changed.Port = freePort(t)
	updated := state.UpdateHSMS(changed)
	if !updated.Runtime.RestartRequired {
		t.Fatalf("expected connection change to require restart while running")
	}

	stopped := controller.Stop()
	if stopped.Runtime.RestartRequired {
		t.Fatalf("expected stop to clear restartRequired")
	}

	restarted, err := controller.Start()
	if err != nil {
		t.Fatalf("restart controller: %v", err)
	}
	defer controller.Stop()

	if restarted.Runtime.RestartRequired {
		t.Fatalf("expected restart to apply pending connection change")
	}
}

func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve test port: %v", err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

func dialEventually(t *testing.T, port int) net.Conn {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 200*time.Millisecond)
		if err == nil {
			return conn
		}
		if time.Now().After(deadline) {
			t.Fatalf("dial simulator: %v", err)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func acceptEventually(t *testing.T, listener net.Listener) net.Conn {
	t.Helper()

	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		t.Fatal("listener is not a TCP listener")
	}

	if err := tcpListener.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set accept deadline: %v", err)
	}
	conn, err := tcpListener.Accept()
	if err != nil {
		t.Fatalf("accept active connection: %v", err)
	}
	return conn
}

func writeMessage(t *testing.T, conn net.Conn, message hsms.Message) {
	t.Helper()

	frame, err := hsms.EncodeMessage(message)
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}
	if err := hsms.WriteFrame(conn, frame); err != nil {
		t.Fatalf("write message frame: %v", err)
	}
}

func readFrame(t *testing.T, conn net.Conn) *hsms.Frame {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	frame, err := hsms.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	return frame
}

func readMessage(t *testing.T, conn net.Conn) hsms.Message {
	t.Helper()

	frame := readFrame(t, conn)
	if frame.SType != hsms.STypeData {
		t.Fatalf("expected data frame, got %#v", frame)
	}
	message, err := hsms.DecodeMessage(frame)
	if err != nil {
		t.Fatalf("decode message: %v", err)
	}
	return message
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("condition not met before timeout")
}

func itemPtr(item hsms.Item) *hsms.Item {
	return &item
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buffer bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	log.SetOutput(&buffer)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	})

	return &buffer
}

func assertLogContains(t *testing.T, buffer *bytes.Buffer, pattern string) {
	t.Helper()

	if !strings.Contains(buffer.String(), pattern) {
		t.Fatalf("expected logs to contain %q, got:\n%s", pattern, buffer.String())
	}
}
