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
		{ID: "action-1", DelayMS: 20, Type: "event", CEID: "TRANSFER_INITIATED"},
		{ID: "action-2", DelayMS: 40, Type: "mutate", Target: "carriers.CARR001.location", Value: "SHELF_A01"},
		{ID: "action-3", DelayMS: 40, Type: "mutate", Target: "ports.LP01", Value: "empty"},
		{ID: "action-4", DelayMS: 40, Type: "event", CEID: "TRANSFER_COMPLETED"},
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
	if ceid, ok := hsms.ExtractSingleASCII(firstEvent); !ok || ceid != "TRANSFER_INITIATED" {
		t.Fatalf("expected TRANSFER_INITIATED event body, got %#v", firstEvent)
	}

	secondEvent := readMessage(t, conn)
	if ceid, ok := hsms.ExtractSingleASCII(secondEvent); !ok || ceid != "TRANSFER_COMPLETED" {
		t.Fatalf("expected TRANSFER_COMPLETED event body, got %#v", secondEvent)
	}

	waitFor(t, time.Second, func() bool {
		snapshot := state.Snapshot()
		return snapshot.State.Ports["LP01"] == "empty" && snapshot.State.Carriers["CARR001"].Location == "SHELF_A01"
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
