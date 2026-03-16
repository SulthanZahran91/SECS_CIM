package sim

import (
	"fmt"
	"time"

	"secsim/design/backend/internal/hsms"
	"secsim/design/backend/internal/model"
)

type hostBootstrapMatch struct {
	Stream      byte
	Function    byte
	CEID        string
	BodyMatches func(hsms.Message) bool
}

type hostBootstrapState struct {
	Profile   string
	StepIndex int
}

type hostBootstrapStep struct {
	Send   func(model.Snapshot) (hsms.Message, error)
	Expect hostBootstrapMatch
}

type reportDefinition struct {
	ID   uint16
	VIDs []uint16
}

var conveyorReportDefinitions = []reportDefinition{
	{ID: 1, VIDs: []uint16{58, 1, 72}},
	{ID: 2, VIDs: []uint16{58, 1, 72, 68}},
	{ID: 3, VIDs: []uint16{58, 54, 56}},
	{ID: 4, VIDs: []uint16{58, 54, 56}},
	{ID: 5, VIDs: []uint16{58, 54, 56}},
	{ID: 6, VIDs: []uint16{58, 54, 56}},
	{ID: 7, VIDs: []uint16{58, 54, 56}},
	{ID: 8, VIDs: []uint16{58, 54, 56}},
	{ID: 9, VIDs: []uint16{58, 54, 56, 69}},
	{ID: 10, VIDs: []uint16{58, 54, 56, 60}},
	{ID: 11, VIDs: []uint16{58, 54, 56}},
	{ID: 12, VIDs: []uint16{58, 54, 56}},
	{ID: 13, VIDs: []uint16{54, 56}},
	{ID: 14, VIDs: []uint16{54, 56}},
	{ID: 15, VIDs: []uint16{54, 56, 64}},
	{ID: 16, VIDs: []uint16{58, 54, 56}},
	{ID: 17, VIDs: []uint16{54, 56}},
	{ID: 18, VIDs: []uint16{54, 56}},
	{ID: 19, VIDs: []uint16{630}},
	{ID: 20, VIDs: []uint16{630}},
	{ID: 21, VIDs: []uint16{58, 80, 54, 70, 60, 67}},
	{ID: 22, VIDs: []uint16{54, 418}},
	{ID: 23, VIDs: []uint16{54, 418}},
	{ID: 24, VIDs: []uint16{306, 414}},
	{ID: 25, VIDs: []uint16{306, 414}},
	{ID: 26, VIDs: []uint16{306, 414}},
	{ID: 27, VIDs: []uint16{306, 414}},
	{ID: 28, VIDs: []uint16{307, 414}},
	{ID: 29, VIDs: []uint16{307, 414}},
	{ID: 30, VIDs: []uint16{308, 414}},
	{ID: 31, VIDs: []uint16{308, 414}},
	{ID: 32, VIDs: []uint16{306, 414}},
	{ID: 33, VIDs: []uint16{306, 54}},
	{ID: 34, VIDs: []uint16{417}},
	{ID: 35, VIDs: []uint16{310, 414, 1, 312}},
	{ID: 36, VIDs: []uint16{54, 56, 318, 319}},
	{ID: 37, VIDs: []uint16{54, 56, 318, 319, 613}},
	{ID: 38, VIDs: []uint16{320, 54, 613}},
	{ID: 39, VIDs: []uint16{320, 54, 69, 613}},
	{ID: 40, VIDs: []uint16{320, 54, 418, 613}},
	{ID: 41, VIDs: []uint16{320, 54, 69}},
	{ID: 42, VIDs: []uint16{320, 418, 613}},
	{ID: 43, VIDs: []uint16{320, 54, 69, 613}},
	{ID: 44, VIDs: []uint16{320, 418, 613}},
	{ID: 45, VIDs: []uint16{320, 54, 69, 613}},
	{ID: 46, VIDs: []uint16{320, 418, 613}},
	{ID: 47, VIDs: []uint16{305}},
	{ID: 48, VIDs: []uint16{305}},
	{ID: 49, VIDs: []uint16{305, 322, 323}},
	{ID: 50, VIDs: []uint16{305, 322, 323}},
	{ID: 51, VIDs: []uint16{305}},
	{ID: 52, VIDs: []uint16{305}},
	{ID: 53, VIDs: []uint16{320, 54, 69, 613}},
	{ID: 54, VIDs: []uint16{320, 54, 69, 613}},
	{ID: 55, VIDs: []uint16{305, 443, 327}},
	{ID: 56, VIDs: []uint16{630}},
	{ID: 57, VIDs: []uint16{305, 54, 444, 70, 60}},
	{ID: 58, VIDs: []uint16{306, 634}},
	{ID: 59, VIDs: []uint16{54, 56}},
	{ID: 60, VIDs: []uint16{54, 56, 69}},
	{ID: 61, VIDs: []uint16{54, 56}},
	{ID: 62, VIDs: []uint16{54, 56, 69}},
	{ID: 63, VIDs: []uint16{54, 305, 64, 613}},
	{ID: 64, VIDs: []uint16{306, 54}},
	{ID: 65, VIDs: []uint16{54, 305, 613, 643, 642, 644}},
	{ID: 66, VIDs: []uint16{306, 54}},
	{ID: 67, VIDs: []uint16{306, 54}},
	{ID: 68, VIDs: []uint16{306, 54}},
	{ID: 69, VIDs: []uint16{306, 54}},
	{ID: 70, VIDs: []uint16{320, 54, 613, 628}},
	{ID: 71, VIDs: []uint16{54, 69, 402, 403, 338}},
	{ID: 72, VIDs: []uint16{306, 339}},
	{ID: 73, VIDs: []uint16{306, 339, 340}},
}

var conveyorLinkedCEIDs = []uint16{
	51, 52, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 151, 152, 153, 157, 158, 159, 251, 253, 254, 255, 256,
	301, 302, 303, 304, 305, 306, 307, 308, 309, 310, 601, 605, 607, 609, 617, 618, 619, 620, 621, 622, 623, 624,
	625, 626, 627, 628, 629, 632, 634, 635, 636, 637, 638, 639, 643, 644, 645, 646, 647, 648, 702, 703, 704, 705,
	706, 707, 708, 709, 710, 711,
}

var conveyorEnabledCEIDs = []uint16{
	1, 2, 3, 51, 52, 53, 54, 55, 56, 57, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 151, 152, 153, 157, 158,
	159, 251, 253, 254, 255, 256, 301, 302, 303, 304, 305, 306, 307, 308, 309, 310, 601, 605, 607, 609, 617, 618,
	619, 620, 621, 622, 623, 624, 625, 626, 627, 628, 629, 632, 634, 635, 636, 637, 638, 639, 643, 644, 645, 646,
	647, 648, 702, 703, 704, 705, 706, 707, 708, 709, 710, 711,
}

var conveyorStatusSVIDs = []uint16{98, 81, 83, 4, 401, 507, 509, 511, 76, 628, 631, 632}

func (m hostBootstrapMatch) Matches(message hsms.Message) bool {
	if message.Stream != m.Stream || message.Function != m.Function {
		return false
	}
	if m.CEID == "" {
		if m.BodyMatches == nil {
			return true
		}
		return m.BodyMatches(message)
	}
	ceid, ok := hsms.ExtractS6F11CEID(message)
	if !ok || ceid != m.CEID {
		return false
	}
	if m.BodyMatches == nil {
		return true
	}
	return m.BodyMatches(message)
}

func hostBootstrapSteps(profile string) []hostBootstrapStep {
	switch profile {
	case model.HostStartupProfileStocker:
		return []hostBootstrapStep{
			{Send: sendS1F13, Expect: hostBootstrapMatch{Stream: 1, Function: 14, BodyMatches: matchS1F14EstablishCommAck}},
			{Send: sendS1F17, Expect: hostBootstrapMatch{Stream: 1, Function: 18, BodyMatches: matchSingleBinaryAck(0x00)}},
			{Send: sendS2F31, Expect: hostBootstrapMatch{Stream: 2, Function: 32, BodyMatches: matchSingleBinaryAck(0x00)}},
		}
	case model.HostStartupProfileConveyor:
		steps := []hostBootstrapStep{
			{Send: sendS1F13, Expect: hostBootstrapMatch{Stream: 1, Function: 14, BodyMatches: matchS1F14EstablishCommAck}},
			{
				Send:   sendS1F17,
				Expect: hostBootstrapMatch{Stream: 6, Function: 11, CEID: "3", BodyMatches: matchConveyorEventReport(3, 1, matchEmptyListItem)},
			},
			{Send: nil, Expect: hostBootstrapMatch{Stream: 1, Function: 18, BodyMatches: matchSingleBinaryAck(0x00)}},
			{Send: sendS2F31, Expect: hostBootstrapMatch{Stream: 2, Function: 32, BodyMatches: matchSingleBinaryAck(0x00)}},
			{Send: sendConveyorS2F15, Expect: hostBootstrapMatch{Stream: 2, Function: 16, BodyMatches: matchSingleBinaryAck(0x00)}},
			{Send: sendConveyorS2F37Disable, Expect: hostBootstrapMatch{Stream: 2, Function: 38, BodyMatches: matchSingleBinaryAck(0x00)}},
			{Send: sendConveyorS2F33Reset, Expect: hostBootstrapMatch{Stream: 2, Function: 34, BodyMatches: matchSingleBinaryAck(0x00)}},
			{Send: sendConveyorS2F33DefineReports, Expect: hostBootstrapMatch{Stream: 2, Function: 34, BodyMatches: matchSingleBinaryAck(0x00)}},
			{Send: sendConveyorS2F35LinkReports, Expect: hostBootstrapMatch{Stream: 2, Function: 36, BodyMatches: matchSingleBinaryAck(0x00)}},
			{Send: sendConveyorS2F37Enable, Expect: hostBootstrapMatch{Stream: 2, Function: 38, BodyMatches: matchSingleBinaryAck(0x00)}},
			{Send: sendConveyorS5F3Disable, Expect: hostBootstrapMatch{Stream: 5, Function: 4, BodyMatches: matchSingleBinaryAck(0x00)}},
			{Send: sendConveyorS5F3Enable, Expect: hostBootstrapMatch{Stream: 5, Function: 4, BodyMatches: matchSingleBinaryAck(0x00)}},
			{Send: sendConveyorS1F3Status(6), Expect: hostBootstrapMatch{Stream: 1, Function: 4, BodyMatches: matchConveyorStatusReply(6)}},
			{Send: sendConveyorS2F41Command("PAUSE"), Expect: hostBootstrapMatch{Stream: 2, Function: 42, BodyMatches: matchCommandAck(0x00)}},
			{
				Send:   nil,
				Expect: hostBootstrapMatch{Stream: 6, Function: 11, CEID: "57", BodyMatches: matchConveyorEventReport(57, 1, matchEmptyListItem)},
			},
			{
				Send:   nil,
				Expect: hostBootstrapMatch{Stream: 6, Function: 11, CEID: "55", BodyMatches: matchConveyorEventReport(55, 1, matchEmptyListItem)},
			},
		}
		for _, svid := range conveyorStatusSVIDs {
			steps = append(steps, hostBootstrapStep{
				Send:   sendConveyorS1F3Status(svid),
				Expect: hostBootstrapMatch{Stream: 1, Function: 4, BodyMatches: matchConveyorStatusReply(svid)},
			})
		}
		steps = append(steps, hostBootstrapStep{
			Send:   sendConveyorS2F41Command("RESUME"),
			Expect: hostBootstrapMatch{Stream: 2, Function: 42, BodyMatches: matchCommandAck(0x00)},
		}, hostBootstrapStep{
			Send:   nil,
			Expect: hostBootstrapMatch{Stream: 6, Function: 11, CEID: "53", BodyMatches: matchConveyorEventReport(53, 1, matchEmptyListItem)},
		}, hostBootstrapStep{
			Send:   nil,
			Expect: hostBootstrapMatch{Stream: 6, Function: 11, CEID: "601", BodyMatches: matchConveyorEventReport(601, 23, matchSingleU2ListItem)},
		})
		return steps
	default:
		return nil
	}
}

func matchS1F14EstablishCommAck(message hsms.Message) bool {
	if message.Body == nil || message.Body.Type != hsms.ItemList || len(message.Body.Children) != 2 {
		return false
	}
	if !matchSingleByteBinary(message.Body.Children[0], 0x00) {
		return false
	}
	info := message.Body.Children[1]
	return info.Type == hsms.ItemList &&
		len(info.Children) == 2 &&
		info.Children[0].Type == hsms.ItemASCII &&
		info.Children[1].Type == hsms.ItemASCII
}

func matchSingleBinaryAck(ack byte) func(hsms.Message) bool {
	return func(message hsms.Message) bool {
		return message.Body != nil && matchSingleByteBinary(*message.Body, ack)
	}
}

func matchCommandAck(ack byte) func(hsms.Message) bool {
	return func(message hsms.Message) bool {
		if message.Body == nil || message.Body.Type != hsms.ItemList || len(message.Body.Children) != 2 {
			return false
		}
		return matchSingleByteBinary(message.Body.Children[0], ack) && matchEmptyListItem(message.Body.Children[1])
	}
}

func matchConveyorEventReport(ceid uint16, reportID uint16, dataMatch func(hsms.Item) bool) func(hsms.Message) bool {
	return func(message hsms.Message) bool {
		if message.Body == nil || message.Body.Type != hsms.ItemList || len(message.Body.Children) != 3 {
			return false
		}
		if message.Body.Children[0].ScalarValue() != "0" || message.Body.Children[1].ScalarValue() != fmt.Sprintf("%d", ceid) {
			return false
		}
		reports := message.Body.Children[2]
		if reports.Type != hsms.ItemList || len(reports.Children) != 1 {
			return false
		}
		report := reports.Children[0]
		if report.Type != hsms.ItemList || len(report.Children) != 2 || report.Children[0].ScalarValue() != fmt.Sprintf("%d", reportID) {
			return false
		}
		return dataMatch(report.Children[1])
	}
}

func matchConveyorStatusReply(svid uint16) func(hsms.Message) bool {
	return func(message hsms.Message) bool {
		if message.Body == nil || message.Body.Type != hsms.ItemList || len(message.Body.Children) != 1 {
			return false
		}

		value := message.Body.Children[0]
		switch svid {
		case 6:
			return value.Type == hsms.ItemU1
		case 98:
			return value.Type == hsms.ItemList &&
				len(value.Children) == 1 &&
				value.Children[0].Type == hsms.ItemASCII
		case 81, 83, 4, 628, 631, 632:
			return matchEmptyListItem(value)
		case 401, 76:
			return value.Type == hsms.ItemU2
		case 507:
			return matchListOfLists(value, 9)
		case 509, 511:
			return matchListOfLists(value, 6)
		default:
			return true
		}
	}
}

func matchSingleByteBinary(item hsms.Item, value byte) bool {
	return item.Type == hsms.ItemBinary && len(item.Bytes) == 1 && item.Bytes[0] == value
}

func matchEmptyListItem(item hsms.Item) bool {
	return item.Type == hsms.ItemList && len(item.Children) == 0
}

func matchSingleU2ListItem(item hsms.Item) bool {
	return item.Type == hsms.ItemList &&
		len(item.Children) == 1 &&
		item.Children[0].Type == hsms.ItemU2 &&
		item.Children[0].Uint16 == 1
}

func matchListOfLists(item hsms.Item, rowWidth int) bool {
	if item.Type != hsms.ItemList || len(item.Children) == 0 {
		return false
	}
	for _, child := range item.Children {
		if child.Type != hsms.ItemList || len(child.Children) != rowWidth || child.Children[0].Type != hsms.ItemASCII {
			return false
		}
	}
	return true
}

func sendS1F13(config model.Snapshot) (hsms.Message, error) {
	return hsms.BuildS1F13(model.HSMSHeaderSessionID(config.HSMS), 0), nil
}

func sendS1F17(config model.Snapshot) (hsms.Message, error) {
	return hsms.BuildS1F17(model.HSMSHeaderSessionID(config.HSMS), 0), nil
}

func sendS2F31(config model.Snapshot) (hsms.Message, error) {
	return hsms.BuildS2F31(model.HSMSHeaderSessionID(config.HSMS), 0, time.Now()), nil
}

func sendConveyorS2F15(config model.Snapshot) (hsms.Message, error) {
	body := hsms.List(
		hsms.List(
			hsms.U2(62),
			hsms.ASCII("B1ACNV15201"),
		),
	)
	return buildStartupMessage(config, 2, 15, true, &body), nil
}

func sendConveyorS2F37Disable(config model.Snapshot) (hsms.Message, error) {
	body := hsms.List(hsms.Boolean(false), hsms.List())
	return buildStartupMessage(config, 2, 37, true, &body), nil
}

func sendConveyorS2F33Reset(config model.Snapshot) (hsms.Message, error) {
	body := hsms.List(hsms.U4(1), hsms.List())
	return buildStartupMessage(config, 2, 33, true, &body), nil
}

func sendConveyorS2F33DefineReports(config model.Snapshot) (hsms.Message, error) {
	reports := make([]hsms.Item, 0, len(conveyorReportDefinitions))
	for _, definition := range conveyorReportDefinitions {
		reports = append(reports, buildReportDefinitionItem(definition))
	}
	body := hsms.List(
		hsms.U4(1),
		hsms.List(reports...),
	)
	return buildStartupMessage(config, 2, 33, true, &body), nil
}

func sendConveyorS2F35LinkReports(config model.Snapshot) (hsms.Message, error) {
	if len(conveyorLinkedCEIDs) != len(conveyorReportDefinitions) {
		return hsms.Message{}, fmt.Errorf("conveyor CEID/report mapping mismatch: %d CEIDs for %d reports", len(conveyorLinkedCEIDs), len(conveyorReportDefinitions))
	}

	links := make([]hsms.Item, 0, len(conveyorLinkedCEIDs))
	for index, ceid := range conveyorLinkedCEIDs {
		links = append(links, hsms.List(
			hsms.U2(ceid),
			hsms.List(hsms.U2(conveyorReportDefinitions[index].ID)),
		))
	}
	body := hsms.List(
		hsms.U4(1),
		hsms.List(links...),
	)
	return buildStartupMessage(config, 2, 35, true, &body), nil
}

func sendConveyorS2F37Enable(config model.Snapshot) (hsms.Message, error) {
	enabled := make([]hsms.Item, 0, len(conveyorEnabledCEIDs))
	for _, ceid := range conveyorEnabledCEIDs {
		enabled = append(enabled, hsms.U2(ceid))
	}
	body := hsms.List(
		hsms.Boolean(true),
		hsms.List(enabled...),
	)
	return buildStartupMessage(config, 2, 37, true, &body), nil
}

func sendConveyorS5F3Disable(config model.Snapshot) (hsms.Message, error) {
	body := hsms.List(hsms.Binary(0x00), hsms.U4(0))
	return buildStartupMessage(config, 5, 3, true, &body), nil
}

func sendConveyorS5F3Enable(config model.Snapshot) (hsms.Message, error) {
	body := hsms.List(hsms.Binary(0x80), hsms.U4(0))
	return buildStartupMessage(config, 5, 3, true, &body), nil
}

func sendConveyorS1F3Status(svid uint16) func(model.Snapshot) (hsms.Message, error) {
	return func(config model.Snapshot) (hsms.Message, error) {
		body := hsms.List(hsms.U2(svid))
		return buildStartupMessage(config, 1, 3, true, &body), nil
	}
}

func sendConveyorS2F41Command(command string) func(model.Snapshot) (hsms.Message, error) {
	return func(config model.Snapshot) (hsms.Message, error) {
		body := hsms.List(hsms.ASCII(command), hsms.List())
		return buildStartupMessage(config, 2, 41, true, &body), nil
	}
}

func buildReportDefinitionItem(definition reportDefinition) hsms.Item {
	vids := make([]hsms.Item, 0, len(definition.VIDs))
	for _, vid := range definition.VIDs {
		vids = append(vids, hsms.U2(vid))
	}
	return hsms.List(
		hsms.U2(definition.ID),
		hsms.List(vids...),
	)
}

func buildStartupMessage(config model.Snapshot, stream byte, function byte, wbit bool, body *hsms.Item) hsms.Message {
	return hsms.Message{
		SessionID: model.HSMSHeaderSessionID(config.HSMS),
		Stream:    stream,
		Function:  function,
		WBit:      wbit,
		Body:      body,
	}
}
