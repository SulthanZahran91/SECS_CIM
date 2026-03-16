package sim

import (
	"fmt"
	"time"

	"secsim/design/backend/internal/hsms"
	"secsim/design/backend/internal/model"
)

type hostBootstrapMatch struct {
	Stream   byte
	Function byte
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

func hostBootstrapSteps(profile string) []hostBootstrapStep {
	switch profile {
	case model.HostStartupProfileStocker:
		return []hostBootstrapStep{
			{Send: sendS1F13, Expect: hostBootstrapMatch{Stream: 1, Function: 14}},
			{Send: sendS1F17, Expect: hostBootstrapMatch{Stream: 1, Function: 18}},
			{Send: sendS2F31, Expect: hostBootstrapMatch{Stream: 2, Function: 32}},
		}
	case model.HostStartupProfileConveyor:
		return []hostBootstrapStep{
			{Send: sendS1F13, Expect: hostBootstrapMatch{Stream: 1, Function: 14}},
			{Send: sendS1F17, Expect: hostBootstrapMatch{Stream: 1, Function: 18}},
			{Send: sendS2F31, Expect: hostBootstrapMatch{Stream: 2, Function: 32}},
			{Send: sendConveyorS2F15, Expect: hostBootstrapMatch{Stream: 2, Function: 16}},
			{Send: sendConveyorS2F37Disable, Expect: hostBootstrapMatch{Stream: 2, Function: 38}},
			{Send: sendConveyorS2F33Reset, Expect: hostBootstrapMatch{Stream: 2, Function: 34}},
			{Send: sendConveyorS2F33DefineReports, Expect: hostBootstrapMatch{Stream: 2, Function: 34}},
			{Send: sendConveyorS2F35LinkReports, Expect: hostBootstrapMatch{Stream: 2, Function: 36}},
			{Send: sendConveyorS2F37Enable, Expect: hostBootstrapMatch{Stream: 2, Function: 38}},
			{Send: sendConveyorS5F3Disable, Expect: hostBootstrapMatch{Stream: 5, Function: 4}},
			{Send: sendConveyorS5F3Enable, Expect: hostBootstrapMatch{Stream: 5, Function: 4}},
			{Send: sendConveyorS1F3Status, Expect: hostBootstrapMatch{Stream: 1, Function: 4}},
		}
	default:
		return nil
	}
}

func sendS1F13(config model.Snapshot) (hsms.Message, error) {
	return hsms.BuildS1F13(uint16(config.HSMS.SessionID), 0), nil
}

func sendS1F17(config model.Snapshot) (hsms.Message, error) {
	return hsms.BuildS1F17(uint16(config.HSMS.SessionID), 0), nil
}

func sendS2F31(config model.Snapshot) (hsms.Message, error) {
	return hsms.BuildS2F31(uint16(config.HSMS.SessionID), 0, time.Now()), nil
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

func sendConveyorS1F3Status(config model.Snapshot) (hsms.Message, error) {
	body := hsms.List(hsms.U2(6))
	return buildStartupMessage(config, 1, 3, true, &body), nil
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
		SessionID: uint16(config.HSMS.SessionID),
		Stream:    stream,
		Function:  function,
		WBit:      wbit,
		Body:      body,
	}
}
