package sim

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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

	online := hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      1,
		Function:    17,
		WBit:        true,
		SystemBytes: 0x01020305 + 1,
	}
	writeMessage(t, conn, online)
	onlineAck := readMessage(t, conn)
	if onlineAck.Stream != 1 || onlineAck.Function != 18 || onlineAck.SystemBytes != online.SystemBytes {
		t.Fatalf("expected S1F18 auto-response, got %#v", onlineAck)
	}
	if onlineAck.Body == nil || onlineAck.Body.Type != hsms.ItemBinary || onlineAck.Body.Bytes[0] != 0x00 {
		t.Fatalf("expected S1F18 ack body <B 0x00>, got %#v", onlineAck.Body)
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
		return len(state.Snapshot().Messages) >= 8
	})

	snapshot := state.Snapshot()
	if len(snapshot.Messages) != 8 {
		t.Fatalf("expected 8 logged protocol messages, got %d", len(snapshot.Messages))
	}
	if snapshot.Messages[0].SF != "S1F13" || snapshot.Messages[1].SF != "S1F14" || snapshot.Messages[2].SF != "S1F17" || snapshot.Messages[3].SF != "S1F18" {
		t.Fatalf("expected handshake auto-response log entries, got %#v", snapshot.Messages[:4])
	}
	if snapshot.Messages[4].MatchedRuleID != "rule-1" || snapshot.Messages[5].SF != "S2F42" {
		t.Fatalf("expected rule match and reply logs, got %#v", snapshot.Messages[4:6])
	}
}

func TestInboundMessageFromHSMSSetsS6F11CEIDField(t *testing.T) {
	body := hsms.List(
		hsms.U2(0),
		hsms.U2(158),
		hsms.List(),
	)

	inbound := inboundMessageFromHSMS(hsms.Message{
		SessionID:   1,
		Stream:      6,
		Function:    11,
		WBit:        true,
		SystemBytes: 0x01020304,
		Body:        &body,
	}, time.Date(2026, time.March, 16, 23, 11, 56, 980000000, time.UTC))

	if inbound.Fields == nil {
		t.Fatal("expected inbound S6F11 fields to be populated")
	}
	if got := inbound.Fields["CEID"]; got != "158" {
		t.Fatalf("expected CEID field 158, got %q", got)
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

func TestControllerPassiveS1F17AutoResponseCanBeDisabled(t *testing.T) {
	state := store.New()
	state.ClearLog()

	hsmsConfig := state.ConfigSnapshot().HSMS
	hsmsConfig.Mode = "passive"
	hsmsConfig.IP = "127.0.0.1"
	hsmsConfig.Port = freePort(t)
	hsmsConfig.Handshake.AutoS1F17 = false
	state.UpdateHSMS(hsmsConfig)

	controller := New(state)
	if _, err := controller.Start(); err != nil {
		t.Fatalf("start controller: %v", err)
	}
	defer controller.Stop()

	conn := dialEventually(t, hsmsConfig.Port)
	defer conn.Close()

	if err := hsms.WriteFrame(conn, hsms.NewControlFrame(uint16(hsmsConfig.SessionID), 0x01020304, hsms.STypeSelectReq, 0)); err != nil {
		t.Fatalf("write select.req: %v", err)
	}
	selectRsp := readFrame(t, conn)
	if selectRsp.SType != hsms.STypeSelectRsp || selectRsp.ControlCode != hsms.SelectStatusSuccess {
		t.Fatalf("unexpected select response: %#v", selectRsp)
	}

	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      1,
		Function:    17,
		WBit:        true,
		SystemBytes: 0x01020305,
	})

	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, err := hsms.ReadFrame(conn)
	if err == nil {
		t.Fatal("expected no S1F18 response when auto S1F17 is disabled")
	}
	if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("expected read timeout waiting for disabled S1F17 response, got %v", err)
	}
}

func TestControllerActiveHSMSSessionNormalizesWildcardDialAddress(t *testing.T) {
	traceOutput := captureLogs(t)

	state := store.New()
	state.ClearLog()

	hostListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for wildcard dial normalization test: %v", err)
	}
	defer hostListener.Close()

	hsmsConfig := state.ConfigSnapshot().HSMS
	hsmsConfig.Mode = "active"
	hsmsConfig.IP = "0.0.0.0"
	hsmsConfig.Port = hostListener.Addr().(*net.TCPAddr).Port
	hsmsConfig.SessionID = 41
	hsmsConfig.DeviceID = 9
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

	conn := acceptEventually(t, hostListener)
	defer conn.Close()

	selectReq := readFrame(t, conn)
	if selectReq.SType != hsms.STypeSelectReq {
		t.Fatalf("expected active select.req, got %#v", selectReq)
	}
	if selectReq.SessionID != model.HSMSHeaderSessionID(hsmsConfig) {
		t.Fatalf("expected select.req header ID %d, got %d", model.HSMSHeaderSessionID(hsmsConfig), selectReq.SessionID)
	}
	if err := hsms.WriteFrame(conn, hsms.NewControlFrame(model.HSMSHeaderSessionID(hsmsConfig), selectReq.SystemBytes, hsms.STypeSelectRsp, hsms.SelectStatusSuccess)); err != nil {
		t.Fatalf("write select.rsp: %v", err)
	}

	waitFor(t, time.Second, func() bool {
		snapshot := state.Snapshot()
		return snapshot.Runtime.HSMSState == "SELECTED" && snapshot.Runtime.LastError == ""
	})

	assertLogContains(t, traceOutput, "HSMS control OUT Select.req sid=0x0029")
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
	hsmsConfig.SessionID = 41
	hsmsConfig.DeviceID = 9
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
	if selectReq.SessionID != model.HSMSHeaderSessionID(hsmsConfig) {
		t.Fatalf("expected select.req header ID %d, got %d", model.HSMSHeaderSessionID(hsmsConfig), selectReq.SessionID)
	}
	if err := hsms.WriteFrame(conn, hsms.NewControlFrame(model.HSMSHeaderSessionID(hsmsConfig), selectReq.SystemBytes, hsms.STypeSelectRsp, hsms.SelectStatusSuccess)); err != nil {
		t.Fatalf("write select.rsp: %v", err)
	}

	establishReq := readMessage(t, conn)
	if establishReq.Stream != 1 || establishReq.Function != 13 || !establishReq.WBit {
		t.Fatalf("expected S1F13 bootstrap request, got %#v", establishReq)
	}
	if establishReq.SessionID != model.HSMSHeaderSessionID(hsmsConfig) {
		t.Fatalf("expected startup header ID %d, got %d", model.HSMSHeaderSessionID(hsmsConfig), establishReq.SessionID)
	}

	writeMessage(t, conn, hsms.BuildS1F14(model.HSMSHeaderSessionID(hsmsConfig), establishReq.SystemBytes, "EQP-01", "1.2.3"))

	onlineReq := readMessage(t, conn)
	if onlineReq.Stream != 1 || onlineReq.Function != 17 || !onlineReq.WBit {
		t.Fatalf("expected S1F17 bootstrap request, got %#v", onlineReq)
	}

	s1f18Body := itemPtr(hsms.Binary(0x00))
	writeMessage(t, conn, hsms.Message{
		SessionID:   model.HSMSHeaderSessionID(hsmsConfig),
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
		SessionID:   model.HSMSHeaderSessionID(hsmsConfig),
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
	deviceConfig := state.ConfigSnapshot().Device
	deviceConfig.Name = "TEST_CNVC"
	state.UpdateDevice(deviceConfig)

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

	onlineReq := readMessage(t, conn)
	if onlineReq.Stream != 1 || onlineReq.Function != 17 || !onlineReq.WBit {
		t.Fatalf("expected S1F17 bootstrap request, got %#v", onlineReq)
	}

	onlineEvent := exampleConveyorEvent(t, uint16(hsmsConfig.SessionID), 0x0000BEEF, 3)
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
	if got := offlineReq.Body.Compact(); got != `L:2 <U2 62> <A "TEST_CNVC">` {
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

	enableCollectionReq := readMessage(t, conn)
	if enableCollectionReq.Stream != 2 || enableCollectionReq.Function != 37 || !enableCollectionReq.WBit || enableCollectionReq.Body == nil {
		t.Fatalf("expected first S2F37 request, got %#v", enableCollectionReq)
	}
	if got := enableCollectionReq.Body.Compact(); got != "L:2 <BOOLEAN TRUE> L:0" {
		t.Fatalf("unexpected first S2F37 body: %s", got)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      2,
		Function:    38,
		WBit:        false,
		SystemBytes: enableCollectionReq.SystemBytes,
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

	resetLinksReq := readMessage(t, conn)
	if resetLinksReq.Stream != 2 || resetLinksReq.Function != 35 || !resetLinksReq.WBit || resetLinksReq.Body == nil {
		t.Fatalf("expected S2F35 reset-link request, got %#v", resetLinksReq)
	}
	if got := resetLinksReq.Body.Compact(); got != "L:2 <U4 1> L:0" {
		t.Fatalf("unexpected S2F35 body: %s", got)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      2,
		Function:    36,
		WBit:        false,
		SystemBytes: resetLinksReq.SystemBytes,
		Body:        itemPtr(hsms.Binary(0x00)),
	})

	enableReportsReq := readMessage(t, conn)
	if enableReportsReq.Stream != 2 || enableReportsReq.Function != 37 || !enableReportsReq.WBit || enableReportsReq.Body == nil {
		t.Fatalf("expected second S2F37 request, got %#v", enableReportsReq)
	}
	if got := enableReportsReq.Body.Compact(); got != "L:2 <BOOLEAN TRUE> L:0" {
		t.Fatalf("unexpected second S2F37 body: %s", got)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      2,
		Function:    38,
		WBit:        false,
		SystemBytes: enableReportsReq.SystemBytes,
		Body:        itemPtr(hsms.Binary(0x00)),
	})

	enableAlarmsReq := readMessage(t, conn)
	if enableAlarmsReq.Stream != 5 || enableAlarmsReq.Function != 3 || !enableAlarmsReq.WBit || enableAlarmsReq.Body == nil {
		t.Fatalf("expected S5F3 request, got %#v", enableAlarmsReq)
	}
	if got := enableAlarmsReq.Body.Compact(); got != "L:2 <B 0x01> <U4 0>" {
		t.Fatalf("unexpected S5F3 body: %s", got)
	}
	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      5,
		Function:    4,
		WBit:        false,
		SystemBytes: enableAlarmsReq.SystemBytes,
		Body:        itemPtr(hsms.Binary(0x00)),
	})

	pauseReq := readMessage(t, conn)
	if pauseReq.Stream != 2 || pauseReq.Function != 41 || !pauseReq.WBit || pauseReq.Body == nil {
		t.Fatalf("expected PAUSE S2F41 request, got %#v", pauseReq)
	}
	if got := pauseReq.Body.Compact(); got != `L:2 <A "PAUSE"> L:0` {
		t.Fatalf("unexpected PAUSE command body: %s", got)
	}
	writeMessage(t, conn, hsms.BuildS2F42(uint16(hsmsConfig.SessionID), pauseReq.SystemBytes, 0))

	pauseInitiated := exampleConveyorEvent(t, uint16(hsmsConfig.SessionID), 0x0000CA02, 57)
	writeMessage(t, conn, pauseInitiated)
	pauseInitiatedAck := readMessage(t, conn)
	if pauseInitiatedAck.Stream != 6 || pauseInitiatedAck.Function != 12 || pauseInitiatedAck.SystemBytes != pauseInitiated.SystemBytes {
		t.Fatalf("expected S6F12 ack for pause initiated event, got %#v", pauseInitiatedAck)
	}
	firstStatusReq := readMessage(t, conn)
	if firstStatusReq.Stream != 1 || firstStatusReq.Function != 3 || !firstStatusReq.WBit || firstStatusReq.Body == nil || firstStatusReq.Body.Compact() != "L:1 <U2 4>" {
		t.Fatalf("expected first S1F3 status request, got %#v", firstStatusReq)
	}

	pauseCompleted := exampleConveyorEvent(t, uint16(hsmsConfig.SessionID), 0x0000CA03, 55)
	writeMessage(t, conn, pauseCompleted)
	pauseCompletedAck := readMessage(t, conn)
	if pauseCompletedAck.Stream != 6 || pauseCompletedAck.Function != 12 || pauseCompletedAck.SystemBytes != pauseCompleted.SystemBytes {
		t.Fatalf("expected S6F12 ack for pause completed event, got %#v", pauseCompletedAck)
	}

	writeMessage(t, conn, hsms.Message{
		SessionID:   uint16(hsmsConfig.SessionID),
		Stream:      1,
		Function:    4,
		WBit:        false,
		SystemBytes: firstStatusReq.SystemBytes,
		Body:        exampleConveyorStatusReply(t, 4),
	})

	for _, svid := range conveyorStatusSVIDs[1:] {
		statusReq := readMessage(t, conn)
		if statusReq.Stream != 1 || statusReq.Function != 3 || !statusReq.WBit || statusReq.Body == nil {
			t.Fatalf("expected S1F3 status request for SVID %d, got %#v", svid, statusReq)
		}
		if got := statusReq.Body.Compact(); got != fmt.Sprintf("L:1 <U2 %d>", svid) {
			t.Fatalf("unexpected S1F3 body for SVID %d: %s", svid, got)
		}
		writeMessage(t, conn, hsms.Message{
			SessionID:   uint16(hsmsConfig.SessionID),
			Stream:      1,
			Function:    4,
			WBit:        false,
			SystemBytes: statusReq.SystemBytes,
			Body:        exampleConveyorStatusReply(t, svid),
		})
	}

	waitForBootstrapCleared(t, controller)

	waitFor(t, time.Second, func() bool {
		messages := state.Snapshot().Messages
		return len(messages) >= 24 && messages[0].SF == "S1F17" && messages[len(messages)-1].SF == "S1F4"
	})
}

func TestControllerStopClosesActiveConnectionBeforeReturning(t *testing.T) {
	state := store.New()

	hostListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for active stop test: %v", err)
	}
	defer hostListener.Close()

	hsmsConfig := state.ConfigSnapshot().HSMS
	hsmsConfig.Mode = "active"
	hsmsConfig.IP = "127.0.0.1"
	hsmsConfig.Port = hostListener.Addr().(*net.TCPAddr).Port
	hsmsConfig.Timers.T5 = 1
	hsmsConfig.Timers.T6 = 1
	state.UpdateHSMS(hsmsConfig)

	controller := New(state)
	if _, err := controller.Start(); err != nil {
		t.Fatalf("start controller: %v", err)
	}

	conn := acceptEventually(t, hostListener)
	selectReq := readFrame(t, conn)
	if selectReq.SType != hsms.STypeSelectReq {
		t.Fatalf("expected active select.req, got %#v", selectReq)
	}
	if err := hsms.WriteFrame(conn, hsms.NewControlFrame(model.HSMSHeaderSessionID(hsmsConfig), selectReq.SystemBytes, hsms.STypeSelectRsp, hsms.SelectStatusSuccess)); err != nil {
		t.Fatalf("write select.rsp: %v", err)
	}
	waitFor(t, time.Second, func() bool {
		return state.Snapshot().Runtime.HSMSState == "SELECTED"
	})

	stopped := controller.Stop()
	if stopped.Runtime.Listening || stopped.Runtime.HSMSState != "NOT CONNECTED" {
		t.Fatalf("expected stopped runtime snapshot, got %#v", stopped.Runtime)
	}

	assertConnClosed(t, conn)
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

func assertConnClosed(t *testing.T, conn net.Conn) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	var buffer [1]byte
	_, err := conn.Read(buffer[:])
	if err == nil {
		t.Fatal("expected connection to be closed")
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		t.Fatalf("expected closed connection, got timeout: %v", err)
	}
	if errors.Is(err, io.EOF) {
		return
	}

	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "closed network connection") || strings.Contains(lower, "reset by peer") {
		return
	}

	t.Fatalf("expected closed connection, got %v", err)
}

func itemPtr(item hsms.Item) *hsms.Item {
	return &item
}

func exampleConveyorEvent(t *testing.T, sessionID uint16, systemBytes uint32, ceid uint16) hsms.Message {
	t.Helper()

	reportID := uint16(1)
	data := hsms.List()
	if ceid == 601 {
		reportID = 23
		data = hsms.List(hsms.U2(1))
	}

	return hsms.Message{
		SessionID:   sessionID,
		Stream:      6,
		Function:    11,
		WBit:        true,
		SystemBytes: systemBytes,
		Body: itemPtr(hsms.List(
			hsms.U2(0),
			hsms.U2(ceid),
			hsms.List(hsms.List(hsms.U2(reportID), data)),
		)),
	}
}

func exampleConveyorStatusReply(t *testing.T, svid uint16) *hsms.Item {
	t.Helper()

	var body hsms.Item
	switch svid {
	case 5:
		body = hsms.List(hsms.ASCII("2026031706442112"))
	case 6:
		body = hsms.List(hsms.U1(5))
	case 51:
		body = hsms.List(hsms.List(
			hsms.ASCII("TEST"),
			hsms.ASCII("B1ACNV13201-303"),
		))
	case 52:
		body = hsms.List(hsms.List())
	case 53:
		body = hsms.List(
			hsms.List(
				hsms.ASCII("B1ACNV13201-303"),
				hsms.ASCII("2026031702450439"),
				hsms.U2(15),
				hsms.U2(0),
				hsms.U2(0),
				hsms.U2(1),
				hsms.ASCII(""),
				hsms.ASCII(""),
			),
		)
	case 98:
		body = hsms.List(hsms.List(hsms.ASCII("SEMI.0309")))
	case 81, 83, 4, 628, 631, 632:
		body = hsms.List(hsms.List())
	case 401:
		body = hsms.List(hsms.U2(1))
	case 507:
		body = hsms.List(hsms.List(
			hsms.List(
				hsms.ASCII("B1ACNV15201-201"),
				hsms.ASCII(""),
				hsms.U2(0),
				hsms.U2(1),
				hsms.U2(0),
				hsms.U2(1),
				hsms.U2(0),
				hsms.U2(15),
				hsms.U2(0),
			),
		))
	case 509:
		body = hsms.List(hsms.List(
			hsms.List(
				hsms.ASCII("B1ACNV15201-305"),
				hsms.U2(0),
				hsms.U2(0),
				hsms.U2(1),
				hsms.U2(0),
				hsms.U2(1),
			),
		))
	case 511:
		body = hsms.List(hsms.List(
			hsms.List(
				hsms.ASCII("B1ACNV15201-595"),
				hsms.U2(0),
				hsms.U2(0),
				hsms.U2(0),
				hsms.ASCII(""),
				hsms.ASCII(""),
			),
		))
	case 76:
		body = hsms.List(hsms.U2(62))
	default:
		t.Fatalf("unsupported conveyor SVID %d", svid)
	}

	return &body
}

func waitForBootstrapStep(t *testing.T, controller *Controller, match func(hostBootstrapStep) bool) {
	t.Helper()

	waitFor(t, time.Second, func() bool {
		step, ok := currentBootstrapStep(controller)
		return ok && match(step)
	})
}

func waitForBootstrapCleared(t *testing.T, controller *Controller) {
	t.Helper()

	waitFor(t, time.Second, func() bool {
		controller.mu.Lock()
		defer controller.mu.Unlock()
		return controller.hostBootstrap.Profile == ""
	})
}

func currentBootstrapStep(controller *Controller) (hostBootstrapStep, bool) {
	controller.mu.Lock()
	state := controller.hostBootstrap
	controller.mu.Unlock()
	if state.Profile == "" {
		return hostBootstrapStep{}, false
	}

	steps := hostBootstrapSteps(state.Profile)
	if state.StepIndex < 0 || state.StepIndex >= len(steps) {
		return hostBootstrapStep{}, false
	}

	return steps[state.StepIndex], true
}

func waitForLoggedMessage(t *testing.T, state *store.Store, cursor int, match func(model.MessageRecord) bool) (model.MessageRecord, int) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		messages := state.Snapshot().Messages
		for index := cursor; index < len(messages); index++ {
			if match(messages[index]) {
				return messages[index], index + 1
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	messages := state.Snapshot().Messages
	tailStart := len(messages) - 3
	if tailStart < 0 {
		tailStart = 0
	}
	t.Fatalf("logged message not observed after cursor %d; tail=%#v", cursor, messages[tailStart:])
	return model.MessageRecord{}, cursor
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
