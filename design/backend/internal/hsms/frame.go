package hsms

import (
	"encoding/binary"
	"fmt"
	"io"
)

const headerLength = 10

const (
	STypeData        byte = 0
	STypeSelectReq   byte = 1
	STypeSelectRsp   byte = 2
	STypeDeselectReq byte = 3
	STypeDeselectRsp byte = 4
	STypeLinktestReq byte = 5
	STypeLinktestRsp byte = 6
	STypeRejectReq   byte = 7
	STypeSeparateReq byte = 9
)

const (
	PTypeSecsII         byte = 0
	SelectStatusSuccess byte = 0
)

type Frame struct {
	SessionID   uint16
	Stream      byte
	Function    byte
	WBit        bool
	PType       byte
	SType       byte
	SystemBytes uint32
	ControlCode byte
	Body        []byte
}

func NewDataFrame(sessionID uint16, systemBytes uint32, stream byte, function byte, wbit bool, body []byte) *Frame {
	return &Frame{
		SessionID:   sessionID,
		Stream:      stream,
		Function:    function,
		WBit:        wbit,
		PType:       PTypeSecsII,
		SType:       STypeData,
		SystemBytes: systemBytes,
		Body:        append([]byte(nil), body...),
	}
}

func NewControlFrame(sessionID uint16, systemBytes uint32, stype byte, controlCode byte) *Frame {
	return &Frame{
		SessionID:   sessionID,
		PType:       PTypeSecsII,
		SType:       stype,
		SystemBytes: systemBytes,
		ControlCode: controlCode,
		Body:        []byte{},
	}
}

func ReadFrame(r io.Reader) (*Frame, error) {
	lengthBytes := make([]byte, 4)
	if _, err := io.ReadFull(r, lengthBytes); err != nil {
		return nil, err
	}

	payloadLength := binary.BigEndian.Uint32(lengthBytes)
	if payloadLength < headerLength {
		return nil, fmt.Errorf("invalid HSMS payload length %d", payloadLength)
	}

	payload := make([]byte, payloadLength)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	frame := &Frame{
		SessionID:   binary.BigEndian.Uint16(payload[0:2]),
		PType:       payload[4],
		SType:       payload[5],
		SystemBytes: binary.BigEndian.Uint32(payload[6:10]),
		Body:        append([]byte(nil), payload[10:]...),
	}

	if frame.SType == STypeData {
		frame.WBit = payload[2]&0x80 != 0
		frame.Stream = payload[2] & 0x7F
		frame.Function = payload[3]
	} else {
		frame.ControlCode = payload[3]
	}

	return frame, nil
}

func WriteFrame(w io.Writer, frame *Frame) error {
	if frame == nil {
		return fmt.Errorf("nil HSMS frame")
	}

	payloadLength := headerLength + len(frame.Body)
	buffer := make([]byte, 4+payloadLength)
	binary.BigEndian.PutUint32(buffer[0:4], uint32(payloadLength))
	binary.BigEndian.PutUint16(buffer[4:6], frame.SessionID)
	if frame.SType == STypeData {
		headerByte2 := frame.Stream & 0x7F
		if frame.WBit {
			headerByte2 |= 0x80
		}
		buffer[6] = headerByte2
		buffer[7] = frame.Function
	} else {
		buffer[6] = 0
		buffer[7] = frame.ControlCode
	}
	buffer[8] = frame.PType
	buffer[9] = frame.SType
	binary.BigEndian.PutUint32(buffer[10:14], frame.SystemBytes)
	copy(buffer[14:], frame.Body)

	_, err := w.Write(buffer)
	return err
}
