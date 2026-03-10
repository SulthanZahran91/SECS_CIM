package hsms

import "fmt"

type Message struct {
	SessionID   uint16
	Stream      byte
	Function    byte
	WBit        bool
	SystemBytes uint32
	Body        *Item
}

func DecodeMessage(frame *Frame) (Message, error) {
	if frame == nil {
		return Message{}, fmt.Errorf("nil HSMS frame")
	}
	if frame.SType != STypeData {
		return Message{}, fmt.Errorf("frame stype %d is not a data message", frame.SType)
	}

	message := Message{
		SessionID:   frame.SessionID,
		Stream:      frame.Stream,
		Function:    frame.Function,
		WBit:        frame.WBit,
		SystemBytes: frame.SystemBytes,
	}

	if len(frame.Body) > 0 {
		body, consumed, err := DecodeItem(frame.Body)
		if err != nil {
			return Message{}, err
		}
		if consumed != len(frame.Body) {
			return Message{}, fmt.Errorf("unexpected trailing body bytes: consumed=%d total=%d", consumed, len(frame.Body))
		}
		message.Body = &body
	}

	return message, nil
}

func EncodeMessage(message Message) (*Frame, error) {
	body := []byte(nil)
	if message.Body != nil {
		encoded, err := EncodeItem(*message.Body)
		if err != nil {
			return nil, err
		}
		body = encoded
	}

	return NewDataFrame(message.SessionID, message.SystemBytes, message.Stream, message.Function, message.WBit, body), nil
}

func (message Message) BodySML() string {
	if message.Body == nil {
		return ""
	}

	return message.Body.Pretty()
}

func (message Message) RawSML() string {
	sml := fmt.Sprintf("S%dF%d", message.Stream, message.Function)
	if message.WBit {
		sml += " W"
	}
	if message.Body != nil {
		sml += " " + message.Body.Compact()
	}

	return sml
}

func (message Message) Label() string {
	switch {
	case message.Stream == 1 && message.Function == 1:
		return "Are You There?"
	case message.Stream == 1 && message.Function == 2:
		return "On Line Data"
	case message.Stream == 1 && message.Function == 13:
		return "Establish Comm"
	case message.Stream == 1 && message.Function == 14:
		return "Establish Comm Ack"
	case message.Stream == 2 && message.Function == 25:
		return "Loopback Diagnostic"
	case message.Stream == 2 && message.Function == 26:
		return "Loopback Diagnostic Ack"
	case message.Stream == 2 && message.Function == 41:
		if rcmd, _, ok := ExtractRemoteCommand(message); ok {
			return "Remote Command: " + rcmd
		}
		return "Remote Command"
	case message.Stream == 2 && message.Function == 42:
		return "Remote Cmd Ack"
	case message.Stream == 6 && message.Function == 11:
		if ceid, ok := ExtractSingleASCII(message); ok {
			return ceid
		}
		return "Event Report"
	case message.Stream == 6 && message.Function == 12:
		return "Event Ack"
	default:
		return fmt.Sprintf("S%dF%d", message.Stream, message.Function)
	}
}

func ExtractRemoteCommand(message Message) (string, map[string]string, bool) {
	if message.Body == nil || message.Body.Type != ItemList || len(message.Body.Children) < 2 {
		return "", nil, false
	}

	commandItem := message.Body.Children[0]
	parametersItem := message.Body.Children[1]
	if commandItem.Type != ItemASCII || parametersItem.Type != ItemList {
		return "", nil, false
	}

	fields := map[string]string{
		"RCMD": commandItem.Text,
	}
	for _, pair := range parametersItem.Children {
		if pair.Type != ItemList || len(pair.Children) != 2 {
			continue
		}
		key := pair.Children[0]
		if key.Type != ItemASCII {
			continue
		}
		fields[key.Text] = pair.Children[1].ScalarValue()
	}

	return commandItem.Text, fields, true
}

func ExtractSingleASCII(message Message) (string, bool) {
	if message.Body == nil || message.Body.Type != ItemList || len(message.Body.Children) != 1 {
		return "", false
	}

	value := message.Body.Children[0]
	if value.Type != ItemASCII {
		return "", false
	}

	return value.Text, true
}

func BuildS1F2(sessionID uint16, systemBytes uint32, mdln string, softrev string) Message {
	body := List(ASCII(mdln), ASCII(softrev))
	return Message{
		SessionID:   sessionID,
		Stream:      1,
		Function:    2,
		WBit:        false,
		SystemBytes: systemBytes,
		Body:        &body,
	}
}

func BuildS1F14(sessionID uint16, systemBytes uint32, mdln string, softrev string) Message {
	body := List(
		Binary(0x00),
		List(
			ASCII(mdln),
			ASCII(softrev),
		),
	)
	return Message{
		SessionID:   sessionID,
		Stream:      1,
		Function:    14,
		WBit:        false,
		SystemBytes: systemBytes,
		Body:        &body,
	}
}

func BuildS2F26(sessionID uint16, systemBytes uint32, body *Item) Message {
	return Message{
		SessionID:   sessionID,
		Stream:      2,
		Function:    26,
		WBit:        false,
		SystemBytes: systemBytes,
		Body:        body,
	}
}

func BuildS2F42(sessionID uint16, systemBytes uint32, ack byte) Message {
	body := List(Binary(ack), List())
	return Message{
		SessionID:   sessionID,
		Stream:      2,
		Function:    42,
		WBit:        false,
		SystemBytes: systemBytes,
		Body:        &body,
	}
}

func BuildS6F11(sessionID uint16, systemBytes uint32, ceid string) Message {
	body := List(ASCII(ceid))
	return Message{
		SessionID:   sessionID,
		Stream:      6,
		Function:    11,
		WBit:        true,
		SystemBytes: systemBytes,
		Body:        &body,
	}
}
