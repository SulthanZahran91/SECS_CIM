package hsms

import (
	"bytes"
	"testing"
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
