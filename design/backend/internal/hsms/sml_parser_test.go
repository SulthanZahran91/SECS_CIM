package hsms

import "testing"

func TestSignedItemsRoundTrip(t *testing.T) {
	body := List(I1(-1), I2(-2), I4(-3))

	encoded, err := EncodeItem(body)
	if err != nil {
		t.Fatalf("encode signed items: %v", err)
	}

	decoded, consumed, err := DecodeItem(encoded)
	if err != nil {
		t.Fatalf("decode signed items: %v", err)
	}
	if consumed != len(encoded) {
		t.Fatalf("expected to consume %d bytes, consumed %d", len(encoded), consumed)
	}
	if got := decoded.Compact(); got != body.Compact() {
		t.Fatalf("expected compact SML %q, got %q", body.Compact(), got)
	}
}

func TestParseSMLItemParsesNestedGenericPayload(t *testing.T) {
	item, err := ParseSMLItem("L:3 <A \"TRANSFER\"> L:2 <U1 1> <A \"LP01\"> <I -7>")
	if err != nil {
		t.Fatalf("parse generic SML payload: %v", err)
	}

	if item.Type != ItemList || len(item.Children) != 3 {
		t.Fatalf("expected outer list with three children, got %#v", item)
	}
	if got := item.Children[0].ScalarValue(); got != "TRANSFER" {
		t.Fatalf("expected first child TRANSFER, got %q", got)
	}
	if item.Children[1].Type != ItemList || len(item.Children[1].Children) != 2 {
		t.Fatalf("expected nested list child, got %#v", item.Children[1])
	}
	if got := item.Children[1].Children[0].ScalarValue(); got != "1" {
		t.Fatalf("expected nested first child 1, got %q", got)
	}
	if got := item.Children[1].Children[1].ScalarValue(); got != "LP01" {
		t.Fatalf("expected nested second child LP01, got %q", got)
	}
	if got := item.Children[2].ScalarValue(); got != "-7" {
		t.Fatalf("expected signed scalar -7, got %q", got)
	}
	if got := item.Compact(); got != "L:3 <A \"TRANSFER\"> L:2 <U1 1> <A \"LP01\"> <I4 -7>" {
		t.Fatalf("expected canonical compact output, got %q", got)
	}
}

func TestExtractS6F11CEIDRequiresRecognizedEventShapes(t *testing.T) {
	genericBody := List(ASCII("TRANSFER_INITIATED"), I4(7))
	genericMessage := Message{Stream: 6, Function: 11, Body: &genericBody}
	if ceid, ok := ExtractS6F11CEID(genericMessage); ok || ceid != "" {
		t.Fatalf("expected generic two-item payload not to masquerade as CEID, got %q %t", ceid, ok)
	}
	if got := genericMessage.Label(); got != "Event Report" {
		t.Fatalf("expected generic S6F11 label fallback, got %q", got)
	}

	structuredBody := List(U4(0), U4(1001), List())
	structuredMessage := Message{Stream: 6, Function: 11, Body: &structuredBody}
	if ceid, ok := ExtractS6F11CEID(structuredMessage); !ok || ceid != "1001" {
		t.Fatalf("expected structured S6F11 CEID 1001, got %q %t", ceid, ok)
	}
}
