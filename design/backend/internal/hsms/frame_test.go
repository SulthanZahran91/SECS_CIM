package hsms

import (
	"bytes"
	"net"
	"strings"
	"testing"
	"time"
)

func TestFrameRoundTrip(t *testing.T) {
	original := NewDataFrame(7, 0x01020304, 2, 41, true, []byte{0x41, 0x01, 'X'})

	var buffer bytes.Buffer
	if err := WriteFrame(&buffer, original); err != nil {
		t.Fatalf("write frame: %v", err)
	}

	decoded, err := ReadFrame(&buffer)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}

	if decoded.SessionID != original.SessionID {
		t.Fatalf("expected session %d, got %d", original.SessionID, decoded.SessionID)
	}
	if decoded.Stream != original.Stream || decoded.Function != original.Function || decoded.WBit != original.WBit {
		t.Fatalf("expected data header %#v, got %#v", original, decoded)
	}
	if decoded.SystemBytes != original.SystemBytes {
		t.Fatalf("expected system bytes %08x, got %08x", original.SystemBytes, decoded.SystemBytes)
	}
	if !bytes.Equal(decoded.Body, original.Body) {
		t.Fatalf("expected body %x, got %x", original.Body, decoded.Body)
	}
}

func TestItemEncodeDecodeAndRemoteCommandExtraction(t *testing.T) {
	body := List(
		ASCII("TRANSFER"),
		List(
			List(ASCII("CarrierID"), ASCII("CARR001")),
			List(ASCII("SourcePort"), ASCII("LP01")),
			List(ASCII("Priority"), U1(2)),
		),
	)

	encoded, err := EncodeItem(body)
	if err != nil {
		t.Fatalf("encode item: %v", err)
	}

	decoded, consumed, err := DecodeItem(encoded)
	if err != nil {
		t.Fatalf("decode item: %v", err)
	}
	if consumed != len(encoded) {
		t.Fatalf("expected to consume %d bytes, consumed %d", len(encoded), consumed)
	}
	if got := decoded.Compact(); got != body.Compact() {
		t.Fatalf("expected compact SML %q, got %q", body.Compact(), got)
	}

	message := Message{
		SessionID:   1,
		Stream:      2,
		Function:    41,
		WBit:        true,
		SystemBytes: 0x01020304,
		Body:        &decoded,
	}

	rcmd, fields, ok := ExtractRemoteCommand(message)
	if !ok {
		t.Fatalf("expected S2F41 remote command to parse")
	}
	if rcmd != "TRANSFER" {
		t.Fatalf("expected RCMD TRANSFER, got %q", rcmd)
	}
	if fields["CarrierID"] != "CARR001" || fields["SourcePort"] != "LP01" || fields["Priority"] != "2" {
		t.Fatalf("unexpected extracted fields: %#v", fields)
	}
	if got := message.Label(); got != "Remote Command: TRANSFER" {
		t.Fatalf("expected label to include RCMD, got %q", got)
	}
}

func TestReadFrameWithInterByteTimeoutAllowsIdleBeforeFirstByte(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	type result struct {
		frame *Frame
		err   error
	}

	done := make(chan result, 1)
	go func() {
		frame, err := ReadFrameWithInterByteTimeout(server, 20*time.Millisecond)
		done <- result{frame: frame, err: err}
	}()

	time.Sleep(60 * time.Millisecond)
	if err := WriteFrame(client, NewControlFrame(7, 0x01020304, STypeLinktestReq, 0)); err != nil {
		t.Fatalf("write delayed frame: %v", err)
	}

	select {
	case outcome := <-done:
		if outcome.err != nil {
			t.Fatalf("read delayed frame: %v", outcome.err)
		}
		if outcome.frame.SType != STypeLinktestReq {
			t.Fatalf("expected delayed linktest.req, got %#v", outcome.frame)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for delayed frame")
	}
}

func TestReadFrameWithInterByteTimeoutRejectsMidFrameStalls(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	frame := NewControlFrame(7, 0x01020304, STypeLinktestReq, 0)
	var buffer bytes.Buffer
	if err := WriteFrame(&buffer, frame); err != nil {
		t.Fatalf("encode stalled frame: %v", err)
	}
	raw := buffer.Bytes()

	done := make(chan error, 1)
	go func() {
		_, err := ReadFrameWithInterByteTimeout(server, 20*time.Millisecond)
		done <- err
	}()

	if _, err := client.Write(raw[:1]); err != nil {
		t.Fatalf("write first frame byte: %v", err)
	}
	time.Sleep(60 * time.Millisecond)

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected mid-frame stall to time out")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "timeout") {
			t.Fatalf("expected timeout error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stalled frame error")
	}
}
