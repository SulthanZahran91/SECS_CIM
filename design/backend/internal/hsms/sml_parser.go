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
	parser.skipTrailingPeriods()
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
		return p.parseCompactList()
	}
	if p.src[p.pos] == '<' {
		if p.startsLoggedList() {
			return p.parseLoggedList()
		}
		return p.parseScalar()
	}
	return Item{}, fmt.Errorf("expected item at position %d", p.pos)
}

func (p *smlParser) parseCompactList() (Item, error) {
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

func (p *smlParser) startsLoggedList() bool {
	cursor := p.pos + 1
	for cursor < len(p.src) && unicode.IsSpace(rune(p.src[cursor])) {
		cursor++
	}
	return cursor+1 < len(p.src) && (p.src[cursor] == 'L' || p.src[cursor] == 'l') && p.src[cursor+1] == ','
}

func (p *smlParser) parseLoggedList() (Item, error) {
	content, err := p.readLoggedListHeader()
	if err != nil {
		return Item{}, err
	}

	token, ok, err := parseLoggedToken(content)
	if err != nil {
		return Item{}, err
	}
	if !ok || token.Type != "L" {
		return Item{}, fmt.Errorf("expected logged list token at position %d", p.pos)
	}
	if token.Length == 0 {
		return List(), nil
	}

	children := make([]Item, 0, token.Length)
	for index := 0; index < token.Length; index++ {
		child, err := p.parseItem()
		if err != nil {
			return Item{}, err
		}
		children = append(children, child)
	}

	p.skipSpace()
	if !p.eof() && p.src[p.pos] == '>' {
		p.pos++
		p.skipTrailingPeriods()
	}

	return List(children...), nil
}

func (p *smlParser) parseScalar() (Item, error) {
	content, err := p.readAngleContent()
	if err != nil {
		return Item{}, err
	}
	return parseScalarToken(content)
}

func (p *smlParser) readLoggedListHeader() (string, error) {
	p.pos++
	start := p.pos
	for !p.eof() {
		char := p.src[p.pos]
		if char == '>' || char == '\n' || char == '\r' {
			content := strings.TrimSpace(p.src[start:p.pos])
			if char == '>' {
				p.pos++
			}
			return content, nil
		}
		p.pos++
	}

	if start == p.pos {
		return "", fmt.Errorf("unterminated logged list header")
	}
	return strings.TrimSpace(p.src[start:p.pos]), nil
}

func (p *smlParser) readAngleContent() (string, error) {
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
			return content, nil
		}
		p.pos++
	}

	return "", fmt.Errorf("unterminated scalar item")
}

type loggedToken struct {
	Type   string
	Length int
	Value  string
}

func parseLoggedToken(content string) (loggedToken, bool, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return loggedToken{}, false, nil
	}

	firstSpace := len(trimmed)
	for index, char := range trimmed {
		if unicode.IsSpace(char) {
			firstSpace = index
			break
		}
	}

	commaIndex := strings.IndexByte(trimmed, ',')
	if commaIndex == -1 || commaIndex > firstSpace {
		return loggedToken{}, false, nil
	}

	typeName := strings.ToUpper(strings.TrimSpace(trimmed[:commaIndex]))
	if typeName == "" {
		return loggedToken{}, false, fmt.Errorf("missing logged token type")
	}

	cursor := commaIndex + 1
	for cursor < len(trimmed) && unicode.IsSpace(rune(trimmed[cursor])) {
		cursor++
	}
	start := cursor
	for cursor < len(trimmed) && unicode.IsDigit(rune(trimmed[cursor])) {
		cursor++
	}
	if start == cursor {
		return loggedToken{}, false, fmt.Errorf("missing logged token length")
	}

	length, err := strconv.Atoi(trimmed[start:cursor])
	if err != nil {
		return loggedToken{}, false, err
	}

	value := strings.TrimSpace(trimmed[cursor:])
	switch {
	case strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]"):
		value = ""
	case strings.HasSuffix(value, "]"):
		if labelIndex := strings.LastIndex(value, " ["); labelIndex != -1 {
			value = strings.TrimSpace(value[:labelIndex])
		}
	}

	return loggedToken{Type: typeName, Length: length, Value: value}, true, nil
}

func parseScalarToken(content string) (Item, error) {
	if content == "" {
		return Item{}, fmt.Errorf("empty scalar item")
	}

	if token, ok, err := parseLoggedToken(content); err != nil {
		return Item{}, err
	} else if ok {
		switch token.Type {
		case "L":
			return Item{}, fmt.Errorf("logged list token requires list parsing")
		case "A":
			return ASCII(token.Value), nil
		case "B":
			return parseLoggedBinaryItem(token.Value)
		case "BOOLEAN":
			switch strings.ToUpper(token.Value) {
			case "TRUE", "T", "1":
				return Boolean(true), nil
			case "FALSE", "F", "0":
				return Boolean(false), nil
			default:
				return Item{}, fmt.Errorf("invalid BOOLEAN value %q", token.Value)
			}
		case "U", "U4":
			return parseUnsignedItem(token.Value, 32)
		case "U1":
			return parseUnsignedItem(token.Value, 8)
		case "U2":
			return parseUnsignedItem(token.Value, 16)
		case "I", "I4":
			return parseSignedItem(token.Value, 32)
		case "I1":
			return parseSignedItem(token.Value, 8)
		case "I2":
			return parseSignedItem(token.Value, 16)
		default:
			return Item{}, fmt.Errorf("unsupported scalar type %q", token.Type)
		}
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

func parseLoggedBinaryItem(value string) (Item, error) {
	if strings.TrimSpace(value) == "" {
		return Binary(), nil
	}

	parts := strings.Fields(value)
	bytes := make([]byte, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		base := 10
		if strings.HasPrefix(token, "0x") || strings.HasPrefix(token, "0X") {
			token = token[2:]
			base = 16
		} else if strings.IndexFunc(token, func(char rune) bool {
			return (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')
		}) != -1 {
			base = 16
		}

		parsed, err := strconv.ParseUint(token, base, 8)
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

func (p *smlParser) skipTrailingPeriods() {
	for !p.eof() && p.src[p.pos] == '.' {
		p.pos++
		p.skipSpace()
	}
}

func (p *smlParser) eof() bool {
	return p.pos >= len(p.src)
}
