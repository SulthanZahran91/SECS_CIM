package hsms

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

func ParseSMLItem(raw string) (Item, error) {
	parser := smlParser{src: raw}
	item, err := parser.parseItem()
	if err != nil {
		return Item{}, err
	}
	parser.skipSpace()
	if !parser.eof() {
		return Item{}, fmt.Errorf("unexpected trailing SML content at position %d", parser.pos)
	}
	return item, nil
}

type smlParser struct {
	src string
	pos int
}

func (p *smlParser) parseItem() (Item, error) {
	p.skipSpace()
	if p.eof() {
		return Item{}, fmt.Errorf("unexpected end of SML item")
	}

	if strings.HasPrefix(p.src[p.pos:], "L:") {
		return p.parseList()
	}
	if p.src[p.pos] == '<' {
		return p.parseScalar()
	}
	return Item{}, fmt.Errorf("expected item at position %d", p.pos)
}

func (p *smlParser) parseList() (Item, error) {
	p.pos += 2
	start := p.pos
	for !p.eof() && unicode.IsDigit(rune(p.src[p.pos])) {
		p.pos++
	}
	if start == p.pos {
		return Item{}, fmt.Errorf("missing list length at position %d", start)
	}

	count, err := strconv.Atoi(p.src[start:p.pos])
	if err != nil {
		return Item{}, err
	}

	children := make([]Item, 0, count)
	for index := 0; index < count; index++ {
		child, err := p.parseItem()
		if err != nil {
			return Item{}, err
		}
		children = append(children, child)
	}

	return List(children...), nil
}

func (p *smlParser) parseScalar() (Item, error) {
	p.pos++
	start := p.pos
	inQuote := false
	escaped := false
	for !p.eof() {
		char := p.src[p.pos]
		switch {
		case inQuote && escaped:
			escaped = false
		case inQuote && char == '\\':
			escaped = true
		case inQuote && char == '"':
			inQuote = false
		case !inQuote && char == '"':
			inQuote = true
		case !inQuote && char == '>':
			content := strings.TrimSpace(p.src[start:p.pos])
			p.pos++
			return parseScalarToken(content)
		}
		p.pos++
	}

	return Item{}, fmt.Errorf("unterminated scalar item")
}

func parseScalarToken(content string) (Item, error) {
	if content == "" {
		return Item{}, fmt.Errorf("empty scalar item")
	}

	typeName := content
	value := ""
	for index, char := range content {
		if unicode.IsSpace(char) {
			typeName = content[:index]
			value = strings.TrimSpace(content[index+1:])
			break
		}
	}

	switch strings.ToUpper(typeName) {
	case "A":
		if value == "" {
			return ASCII(""), nil
		}
		return parseASCIIItem(value)
	case "B":
		return parseBinaryItem(value)
	case "BOOLEAN":
		switch strings.ToUpper(value) {
		case "TRUE":
			return Boolean(true), nil
		case "FALSE":
			return Boolean(false), nil
		default:
			return Item{}, fmt.Errorf("invalid BOOLEAN value %q", value)
		}
	case "U", "U4":
		return parseUnsignedItem(value, 32)
	case "U1":
		return parseUnsignedItem(value, 8)
	case "U2":
		return parseUnsignedItem(value, 16)
	case "I", "I4":
		return parseSignedItem(value, 32)
	case "I1":
		return parseSignedItem(value, 8)
	case "I2":
		return parseSignedItem(value, 16)
	default:
		return Item{}, fmt.Errorf("unsupported scalar type %q", typeName)
	}
}

func parseASCIIItem(value string) (Item, error) {
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		decoded, err := strconv.Unquote(value)
		if err != nil {
			return Item{}, err
		}
		return ASCII(decoded), nil
	}

	return ASCII(value), nil
}

func parseBinaryItem(value string) (Item, error) {
	if strings.TrimSpace(value) == "" {
		return Binary(), nil
	}

	parts := strings.Fields(value)
	bytes := make([]byte, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimPrefix(strings.TrimPrefix(part, "0x"), "0X")
		parsed, err := strconv.ParseUint(token, 16, 8)
		if err != nil {
			return Item{}, err
		}
		bytes = append(bytes, byte(parsed))
	}

	return Binary(bytes...), nil
}

func parseUnsignedItem(value string, bits int) (Item, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, bits)
	if err != nil {
		return Item{}, err
	}

	switch bits {
	case 8:
		return U1(uint8(parsed)), nil
	case 16:
		return U2(uint16(parsed)), nil
	default:
		return U4(uint32(parsed)), nil
	}
}

func parseSignedItem(value string, bits int) (Item, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, bits)
	if err != nil {
		return Item{}, err
	}

	switch bits {
	case 8:
		return I1(int8(parsed)), nil
	case 16:
		return I2(int16(parsed)), nil
	default:
		return I4(int32(parsed)), nil
	}
}

func (p *smlParser) skipSpace() {
	for !p.eof() && unicode.IsSpace(rune(p.src[p.pos])) {
		p.pos++
	}
}

func (p *smlParser) eof() bool {
	return p.pos >= len(p.src)
}
