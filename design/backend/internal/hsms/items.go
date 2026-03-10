package hsms

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

const (
	formatList    byte = 0x00
	formatBinary  byte = 0x20
	formatBoolean byte = 0x24
	formatASCII   byte = 0x40
	formatU1      byte = 0xA4
	formatU2      byte = 0xA8
	formatU4      byte = 0xB0
)

type ItemType byte

const (
	ItemList ItemType = iota
	ItemBinary
	ItemBoolean
	ItemASCII
	ItemU1
	ItemU2
	ItemU4
)

type Item struct {
	Type     ItemType
	Children []Item
	Bytes    []byte
	Bool     bool
	Text     string
	Uint8    uint8
	Uint16   uint16
	Uint32   uint32
}

func List(children ...Item) Item {
	return Item{Type: ItemList, Children: append([]Item(nil), children...)}
}

func Binary(values ...byte) Item {
	return Item{Type: ItemBinary, Bytes: append([]byte(nil), values...)}
}

func Boolean(value bool) Item {
	return Item{Type: ItemBoolean, Bool: value}
}

func ASCII(value string) Item {
	return Item{Type: ItemASCII, Text: value}
}

func U1(value uint8) Item {
	return Item{Type: ItemU1, Uint8: value}
}

func U2(value uint16) Item {
	return Item{Type: ItemU2, Uint16: value}
}

func U4(value uint32) Item {
	return Item{Type: ItemU4, Uint32: value}
}

func DecodeItem(data []byte) (Item, int, error) {
	if len(data) < 2 {
		return Item{}, 0, fmt.Errorf("item truncated")
	}

	format := data[0] & 0xFC
	lengthBytes := int(data[0] & 0x03)
	if lengthBytes < 1 || lengthBytes > 3 {
		return Item{}, 0, fmt.Errorf("invalid item length byte count %d", lengthBytes)
	}
	if len(data) < 1+lengthBytes {
		return Item{}, 0, fmt.Errorf("item length truncated")
	}

	length := 0
	for index := 0; index < lengthBytes; index++ {
		length = (length << 8) | int(data[1+index])
	}
	offset := 1 + lengthBytes

	switch format {
	case formatList:
		children := make([]Item, 0, length)
		consumed := offset
		for childIndex := 0; childIndex < length; childIndex++ {
			child, used, err := DecodeItem(data[consumed:])
			if err != nil {
				return Item{}, 0, err
			}
			consumed += used
			children = append(children, child)
		}
		return List(children...), consumed, nil
	case formatBinary:
		if len(data) < offset+length {
			return Item{}, 0, fmt.Errorf("binary item truncated")
		}
		return Binary(data[offset : offset+length]...), offset + length, nil
	case formatBoolean:
		if len(data) < offset+length || length != 1 {
			return Item{}, 0, fmt.Errorf("unsupported boolean item length %d", length)
		}
		return Boolean(data[offset] != 0), offset + length, nil
	case formatASCII:
		if len(data) < offset+length {
			return Item{}, 0, fmt.Errorf("ASCII item truncated")
		}
		return ASCII(string(data[offset : offset+length])), offset + length, nil
	case formatU1:
		if len(data) < offset+length || length != 1 {
			return Item{}, 0, fmt.Errorf("unsupported U1 item length %d", length)
		}
		return U1(data[offset]), offset + length, nil
	case formatU2:
		if len(data) < offset+length || length != 2 {
			return Item{}, 0, fmt.Errorf("unsupported U2 item length %d", length)
		}
		return U2(binary.BigEndian.Uint16(data[offset : offset+length])), offset + length, nil
	case formatU4:
		if len(data) < offset+length || length != 4 {
			return Item{}, 0, fmt.Errorf("unsupported U4 item length %d", length)
		}
		return U4(binary.BigEndian.Uint32(data[offset : offset+length])), offset + length, nil
	default:
		return Item{}, 0, fmt.Errorf("unsupported item format 0x%02X", format)
	}
}

func EncodeItem(item Item) ([]byte, error) {
	switch item.Type {
	case ItemList:
		payload := make([]byte, 0)
		for _, child := range item.Children {
			encoded, err := EncodeItem(child)
			if err != nil {
				return nil, err
			}
			payload = append(payload, encoded...)
		}
		return append(encodeHeader(formatList, len(item.Children)), payload...), nil
	case ItemBinary:
		return append(encodeHeader(formatBinary, len(item.Bytes)), item.Bytes...), nil
	case ItemBoolean:
		value := byte(0)
		if item.Bool {
			value = 1
		}
		return append(encodeHeader(formatBoolean, 1), value), nil
	case ItemASCII:
		payload := []byte(item.Text)
		return append(encodeHeader(formatASCII, len(payload)), payload...), nil
	case ItemU1:
		return append(encodeHeader(formatU1, 1), item.Uint8), nil
	case ItemU2:
		payload := make([]byte, 2)
		binary.BigEndian.PutUint16(payload, item.Uint16)
		return append(encodeHeader(formatU2, len(payload)), payload...), nil
	case ItemU4:
		payload := make([]byte, 4)
		binary.BigEndian.PutUint32(payload, item.Uint32)
		return append(encodeHeader(formatU4, len(payload)), payload...), nil
	default:
		return nil, fmt.Errorf("unsupported item type %d", item.Type)
	}
}

func (item Item) Pretty() string {
	var builder strings.Builder
	writePrettyItem(&builder, item, 0)
	return builder.String()
}

func (item Item) Compact() string {
	switch item.Type {
	case ItemList:
		parts := make([]string, 0, len(item.Children)+1)
		parts = append(parts, "L:"+strconv.Itoa(len(item.Children)))
		for _, child := range item.Children {
			parts = append(parts, child.Compact())
		}
		return strings.Join(parts, " ")
	case ItemBinary:
		if len(item.Bytes) == 0 {
			return "<B>"
		}
		parts := make([]string, 0, len(item.Bytes)+1)
		parts = append(parts, "<B")
		for _, value := range item.Bytes {
			parts = append(parts, fmt.Sprintf("0x%02X", value))
		}
		return strings.Join(parts, " ") + ">"
	case ItemBoolean:
		if item.Bool {
			return "<BOOLEAN TRUE>"
		}
		return "<BOOLEAN FALSE>"
	case ItemASCII:
		return fmt.Sprintf("<A %q>", item.Text)
	case ItemU1:
		return fmt.Sprintf("<U1 %d>", item.Uint8)
	case ItemU2:
		return fmt.Sprintf("<U2 %d>", item.Uint16)
	case ItemU4:
		return fmt.Sprintf("<U4 %d>", item.Uint32)
	default:
		return ""
	}
}

func (item Item) ScalarValue() string {
	switch item.Type {
	case ItemASCII:
		return item.Text
	case ItemU1:
		return strconv.Itoa(int(item.Uint8))
	case ItemU2:
		return strconv.Itoa(int(item.Uint16))
	case ItemU4:
		return strconv.FormatUint(uint64(item.Uint32), 10)
	case ItemBoolean:
		if item.Bool {
			return "true"
		}
		return "false"
	case ItemBinary:
		if len(item.Bytes) == 1 {
			return fmt.Sprintf("0x%02X", item.Bytes[0])
		}
		parts := make([]string, 0, len(item.Bytes))
		for _, value := range item.Bytes {
			parts = append(parts, fmt.Sprintf("0x%02X", value))
		}
		return strings.Join(parts, " ")
	default:
		return item.Compact()
	}
}

func writePrettyItem(builder *strings.Builder, item Item, depth int) {
	switch item.Type {
	case ItemList:
		builder.WriteString("L:")
		builder.WriteString(strconv.Itoa(len(item.Children)))
		for _, child := range item.Children {
			builder.WriteByte('\n')
			builder.WriteString(strings.Repeat("  ", depth+1))
			writePrettyItem(builder, child, depth+1)
		}
	case ItemBinary:
		builder.WriteString(item.Compact())
	case ItemBoolean:
		builder.WriteString(item.Compact())
	case ItemASCII:
		builder.WriteString(item.Compact())
	case ItemU1:
		builder.WriteString(item.Compact())
	case ItemU2:
		builder.WriteString(item.Compact())
	case ItemU4:
		builder.WriteString(item.Compact())
	}
}

func encodeHeader(format byte, length int) []byte {
	lengthBytes := encodeLength(length)
	header := make([]byte, 1+len(lengthBytes))
	header[0] = format | byte(len(lengthBytes))
	copy(header[1:], lengthBytes)
	return header
}

func encodeLength(length int) []byte {
	switch {
	case length <= 0xFF:
		return []byte{byte(length)}
	case length <= 0xFFFF:
		return []byte{byte(length >> 8), byte(length)}
	default:
		return []byte{byte(length >> 16), byte(length >> 8), byte(length)}
	}
}
