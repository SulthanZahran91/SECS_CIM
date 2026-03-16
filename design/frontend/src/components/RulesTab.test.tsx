import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { RulesTab, parseLogRoutine } from "./RulesTab";
import type { Rule } from "../types";
import { makeSnapshot } from "../test/fixtures";

function RulesTabHarness() {
  const initialRule = makeSnapshot().rules[0];
  const [rule, setRule] = useState<Rule>(initialRule);

  return (
    <RulesTab
      rules={[rule]}
      expandedRuleId={rule.id}
      onToggleRule={vi.fn()}
      onCreateRule={vi.fn()}
      onChangeRule={setRule}
      onDuplicateRule={vi.fn()}
      onDeleteRule={vi.fn()}
      onMoveRule={vi.fn()}
      onExportRule={vi.fn()}
      onImportRules={vi.fn()}
    />
  );
}

describe("RulesTab", () => {
  it("edits send actions through the generic SML body editor", async () => {
    const user = userEvent.setup();

    render(<RulesTabHarness />);

    expect(document.querySelector(".panel-scroll-content")).not.toBeNull();
    expect(document.querySelectorAll(".action-editor-row")).toHaveLength(2);
    expect(screen.getByText("Generic outbound SECS message")).toBeInTheDocument();
    expect(screen.getByText("S6F11 W")).toBeInTheDocument();
    expect(screen.getByDisplayValue('L:1 <A "TRANSFER_INITIATED">')).toBeInTheDocument();

    const bodyInput = screen.getByLabelText("Body (SML)");
    fireEvent.change(bodyInput, {
      target: { value: 'L:2\n  <A "TRANSFER_INITIATED">\n  <I 7>' },
    });

    await user.selectOptions(screen.getByLabelText("W-Bit"), "false");

    expect(bodyInput).toHaveValue('L:2\n  <A "TRANSFER_INITIATED">\n  <I 7>');
    expect(screen.getByText("S6F11")).toBeInTheDocument();
    expect(screen.getByText(/Handwrite the outbound message directly/)).toBeInTheDocument();
  });

  it("surfaces readiness issues and applies starter presets", async () => {
    const user = userEvent.setup();

    render(<RulesTabHarness />);

    fireEvent.change(screen.getByLabelText("Body (SML)"), {
      target: { value: "" },
    });

    // Readiness section appears when there are issues - expand it
    const readinessHeader = screen.getByText("Readiness");
    await user.click(readinessHeader);
    expect(screen.getByText("Send action 1 needs a message body.")).toBeInTheDocument();

    // Expand presets section and apply one
    const presetsHeader = screen.getByText("Presets");
    await user.click(presetsHeader);

    await user.click(screen.getByRole("button", { name: /Reject blocked/i }));

    expect(screen.getByDisplayValue("reject when blocked")).toBeInTheDocument();
    expect(screen.getByDisplayValue("blocked")).toBeInTheDocument();
    expect(screen.getByText("ACK 3")).toBeInTheDocument();
  });

  it("parses a pasted routine log and shows detected rules ready to import", async () => {
    const user = userEvent.setup();

    render(<RulesTabHarness />);

    const importHeader = screen.getByText("Import from Log");
    await user.click(importHeader);

    const logSnippet = [
      "@2026/03/11^INFO^SECS_II^SEND",
      "2026/03/11 S6F11 W, S6F11 - Event Report Send [SystemByte = 15]",
      "<L,3>.",
      " ",
      "@2026/03/11^INFO^SECS_II^RECV",
      "2026/03/11 S6F12  S06F12 - Event Report Acknowledge [SystemByte = 15]",
      "<B,1 0>.",
      " ",
      "@2026/03/11^INFO^SECS_II^SEND",
      "2026/03/11 S1F13 W, Establish Communications Request [SystemByte = 16]",
      "<L,2>.",
      " ",
      "@2026/03/11^INFO^SECS_II^RECV",
      "2026/03/11 S1F14  Establish Communications Request Acknowledge [SystemByte = 16]",
      "<L,2>.",
    ].join("\n");

    fireEvent.change(screen.getByLabelText("Paste log snippet"), {
      target: { value: logSnippet },
    });

    // Should detect 2 rules
    expect(screen.getByRole("button", { name: "Import 2 Rules" })).toBeInTheDocument();
    expect(screen.getByText("S6F11", { selector: ".mono" })).toBeInTheDocument();
    expect(screen.getByText("S1F13", { selector: ".mono" })).toBeInTheDocument();
  });

  it("parseLogRoutine correctly extracts matched pairs from example log", () => {
    const log = [
      "@2026/03/11^INFO^SECS_II^SEND",
      "2026/03/11 S6F11 W, S6F11 - Event Report Send [SystemByte = 15]",
      ".",
      " ",
      "@2026/03/11^INFO^SECS_II^RECV",
      "2026/03/11 S6F12  Event Report Acknowledge [SystemByte = 15]",
      ".",
      " ",
      "@2026/03/11^INFO^SECS_II^SEND",
      "2026/03/11 S1F13 W, Establish Communications Request [SystemByte = 16]",
      ".",
      " ",
      "@2026/03/11^INFO^SECS_II^RECV",
      "2026/03/11 S1F14  Establish Acknowledge [SystemByte = 16]",
      ".",
      " ",
      // SEND without W-bit should be skipped
      "@2026/03/11^INFO^SECS_II^SEND",
      "2026/03/11 S1F18  Reply ON-LINE [SystemByte = 29110]",
      ".",
    ].join("\n");

    const templates = parseLogRoutine(log);
    expect(templates).toHaveLength(2);
    expect(templates[0].match).toEqual({ stream: 6, function: 11, rcmd: "" });
    expect(templates[0].conditions).toEqual([]);
    expect(templates[0].reply).toEqual({ stream: 6, function: 12, ack: 0 });
    expect(templates[0].actions).toEqual([]);
    expect(templates[1].match).toEqual({ stream: 1, function: 13, rcmd: "" });
    expect(templates[1].conditions).toEqual([]);
    expect(templates[1].reply).toEqual({ stream: 1, function: 14, ack: 0 });
    expect(templates[1].actions).toEqual([]);
  });

  it("parseLogRoutine captures follow-up host sends as imported actions", () => {
    const log = [
      "@2026/03/16 23:02:41.642^INFO^SECS_II^SEND",
      "TransactionTime : 2026/03/16 23:02:41.642 S6F11 W, S6F11 - Event Report - CEID 158. Carrier WaitIn_RptID14(Origin) [SystemByte = 406000]",
      " <L,3 [L0]",
      "   <U2,1 0 [DataID]>",
      "   <U2,1 158 [CEID]>",
      "   <L,1 [Ln]",
      "     <L,2 [Ln]",
      "       <U2,1 16 [ReportID]>",
      "       <L,1 [Ln]",
      "         <L,2 [Ln]",
      "           <A,10 BBENFB2816 [CarrierID]>",
      "           <A,15 B1ACNV13201-201 [Location]>",
      "         >",
      "       >",
      "     >",
      "   >",
      " >.",
      " ",
      "@2026/03/16 23:02:41.642^INFO^SECS_II^RECV",
      "TransactionTime : 2026/03/16 23:02:41.642 S6F12  S6F12 - Event Report Acknowledge (ERA) [SystemByte = 406000]",
      " <B,1 0 [ACKC6]>.",
      " ",
      "@2026/03/16 23:02:42.033^INFO^SECS_II^RECV",
      "TransactionTime : 2026/03/16 23:02:42.033 S2F49 W, S2F49 - Enhanced Remote Command (ERC) [SystemByte = 791133]",
      " <L,4 []",
      "   <U4,1 0 [DATAID]>",
      "   <A,0  [OBJSPEC]>",
      "   <A,8 TRANSFER [RCMD]>",
      "   <L,4 [CPList]",
      "     <L,2 []>",
      "       <A,11 COMMANDINFO [CPNAME]>",
      "       <L,2 [CEPVAL]>",
      "         <L,2 []>",
      "           <A,9 COMMANDID [CPNAME]>",
      "           <A,5 11991 [CPVAL]>",
      "         >",
      "         <L,2 []>",
      "           <A,8 PRIORITY [CPNAME]>",
      "           <U2,1 50 [CPVAL]>",
      "         >",
      "       >",
      "     >",
      "     <L,2 []>",
      "       <A,12 TRANSFERINFO [CPNAME]>",
      "       <L,3 [CEPVAL]>",
      "         <L,2 []>",
      "           <A,9 CARRIERID [CPNAME]>",
      "           <A,10 BBENFB2816 [CPVAL]>",
      "         >",
      "         <L,2 []>",
      "           <A,6 SOURCE [CPNAME]>",
      "           <A,15 B1ACNV13201-201 [CPVAL]>",
      "         >",
      "         <L,2 [L333]>",
      "           <A,4 DEST [CPNAME]>",
      "           <A,15 B1ACNV13201-999 [CPNAME]>",
      "         >",
      "       >",
      "     >",
      "     <L,2 []>",
      "       <A,16 CARRIERATTRIBUTE [CPNAME]>",
      "       <L,2 [CEPVAL]>",
      "         <L,2 []>",
      "           <A,11 EMPTYSTATUS [CPNAME]>",
      "           <U2,1 1 [CPVAL]>",
      "         >",
      "         <L,2 []>",
      "           <A,12 MATERIALCODE [CPNAME]>",
      "           <A,0  [CPVAL]>",
      "         >",
      "       >",
      "     >",
      "     <L,2 []>",
      "       <A,16 CHILDCARRIERINFO [CPNAME]>",
      "       <L,0 [CEPVAL]>",
      "     >",
      "   >",
      " >.",
      " ",
      "@2026/03/16 23:02:42.033^INFO^SECS_II^SEND",
      "TransactionTime : 2026/03/16 23:02:42.033 S2F50  S2F50 - Enhanced Remote Command Acknowledge (ERCA) [SystemByte = 791133]",
      " <L,2 [L0]",
      "   <B,1 4 [HCACK]>",
      "   <L,0 [L2]>",
      " >.",
    ].join("\n");

    const templates = parseLogRoutine(log);

    expect(templates).toHaveLength(1);
    expect(templates[0].match).toEqual({ stream: 6, function: 11, rcmd: "" });
    expect(templates[0].conditions).toEqual([{ field: "CEID", value: "158" }]);
    expect(templates[0].reply).toEqual({ stream: 6, function: 12, ack: 0 });
    expect(templates[0].actions).toEqual([
      {
        delayMs: 391,
        type: "send",
        stream: 2,
        function: 49,
        wbit: true,
        body:
          'L:4 <U4 0> <A ""> <A "TRANSFER"> L:4 L:2 <A "COMMANDINFO"> L:2 L:2 <A "COMMANDID"> <A "11991"> L:2 <A "PRIORITY"> <U2 50> L:2 <A "TRANSFERINFO"> L:3 L:2 <A "CARRIERID"> <A "BBENFB2816"> L:2 <A "SOURCE"> <A "B1ACNV13201-201"> L:2 <A "DEST"> <A "B1ACNV13201-999"> L:2 <A "CARRIERATTRIBUTE"> L:2 L:2 <A "EMPTYSTATUS"> <U2 1> L:2 <A "MATERIALCODE"> <A ""> L:2 <A "CHILDCARRIERINFO"> L:0',
      },
    ]);
  });

  it("parseLogRoutine keeps repeated S6F11 events separate by CEID", () => {
    const log = [
      "@2026/03/16 23:11:56.762^INFO^SECS_II^SEND",
      "TransactionTime : 2026/03/16 23:11:56.762 S6F11 W, S6F11 - Event Report - CEID 643. Carrier ID Read Report [SystemByte = 406137]",
      " <L,3 [L0]",
      "   <U2,1 0 [DataID]>",
      "   <U2,1 643 [CEID]>",
      "   <L,0 [L1]>",
      " >.",
      " ",
      "@2026/03/16 23:11:56.762^INFO^SECS_II^RECV",
      "TransactionTime : 2026/03/16 23:11:56.762 S6F12  S6F12 - Event Report Acknowledge (ERA) [SystemByte = 406137]",
      " <B,1 0 [ACKC6]>.",
      " ",
      "@2026/03/16 23:11:56.933^INFO^SECS_II^SEND",
      "TransactionTime : 2026/03/16 23:11:56.933 S6F11 W, S6F11 - Event Report - CEID 251. Carrier ID Read Done [SystemByte = 406138]",
      " <L,3 [L0]",
      "   <U2,1 0 [DataID]>",
      "   <U2,1 251 [CEID]>",
      "   <L,0 [L1]>",
      " >.",
      " ",
      "@2026/03/16 23:11:56.933^INFO^SECS_II^RECV",
      "TransactionTime : 2026/03/16 23:11:56.933 S6F12  S6F12 - Event Report Acknowledge (ERA) [SystemByte = 406138]",
      " <B,1 0 [ACKC6]>.",
      " ",
      "@2026/03/16 23:11:56.980^INFO^SECS_II^SEND",
      "TransactionTime : 2026/03/16 23:11:56.980 S6F11 W, S6F11 - Event Report - CEID 158. Carrier WaitIn_RptID14(Origin) [SystemByte = 406139]",
      " <L,3 [L0]",
      "   <U2,1 0 [DataID]>",
      "   <U2,1 158 [CEID]>",
      "   <L,0 [Ln]>",
      " >.",
      " ",
      "@2026/03/16 23:11:56.980^INFO^SECS_II^RECV",
      "TransactionTime : 2026/03/16 23:11:56.980 S6F12  S6F12 - Event Report Acknowledge (ERA) [SystemByte = 406139]",
      " <B,1 0 [ACKC6]>.",
      " ",
      "@2026/03/16 23:11:57.027^INFO^SECS_II^RECV",
      "TransactionTime : 2026/03/16 23:11:57.027 S2F49 W, S2F49 - Enhanced Remote Command (ERC) [SystemByte = 791234]",
      " <L,4 []",
      "   <U4,1 0 [DATAID]>",
      "   <A,0  [OBJSPEC]>",
      "   <A,8 TRANSFER [RCMD]>",
      "   <L,0 [CPList]>",
      " >.",
      " ",
      "@2026/03/16 23:11:57.027^INFO^SECS_II^SEND",
      "TransactionTime : 2026/03/16 23:11:57.027 S2F50  S2F50 - Enhanced Remote Command Acknowledge (ERCA) [SystemByte = 791234]",
      " <L,2 [L0]",
      "   <B,1 4 [HCACK]>",
      "   <L,0 [L2]>",
      " >.",
    ].join("\n");

    const templates = parseLogRoutine(log);

    expect(templates).toHaveLength(3);
    expect(templates.map((template) => template.conditions)).toEqual([
      [{ field: "CEID", value: "643" }],
      [{ field: "CEID", value: "251" }],
      [{ field: "CEID", value: "158" }],
    ]);
    expect(templates[2].actions).toEqual([
      {
        delayMs: 47,
        type: "send",
        stream: 2,
        function: 49,
        wbit: true,
        body: 'L:4 <U4 0> <A ""> <A "TRANSFER"> L:0',
      },
    ]);
  });

  it("shows imported follow-up sends in the log preview", async () => {
    const user = userEvent.setup();

    render(<RulesTabHarness />);

    await user.click(screen.getByText("Import from Log"));

    const log = [
      "@2026/03/16 23:02:41.642^INFO^SECS_II^SEND",
      "TransactionTime : 2026/03/16 23:02:41.642 S6F11 W, S6F11 - Event Report [SystemByte = 406000]",
      " <L,1 [L0]",
      '   <A,5 READY [CEID]>',
      " >.",
      " ",
      "@2026/03/16 23:02:41.642^INFO^SECS_II^RECV",
      "TransactionTime : 2026/03/16 23:02:41.642 S6F12  S6F12 - Event Report Acknowledge (ERA) [SystemByte = 406000]",
      " <B,1 0 [ACKC6]>.",
      " ",
      "@2026/03/16 23:02:42.033^INFO^SECS_II^RECV",
      "TransactionTime : 2026/03/16 23:02:42.033 S2F49 W, S2F49 - Enhanced Remote Command (ERC) [SystemByte = 791133]",
      " <L,4 []",
      "   <U4,1 0 [DATAID]>",
      "   <A,0  [OBJSPEC]>",
      "   <A,8 TRANSFER [RCMD]>",
      "   <L,0 [CPList]>",
      " >.",
      " ",
      "@2026/03/16 23:02:42.033^INFO^SECS_II^SEND",
      "TransactionTime : 2026/03/16 23:02:42.033 S2F50  S2F50 - Enhanced Remote Command Acknowledge (ERCA) [SystemByte = 791133]",
      " <L,2 [L0]",
      "   <B,1 4 [HCACK]>",
      "   <L,0 [L2]>",
      " >.",
    ].join("\n");

    fireEvent.change(screen.getByLabelText("Paste log snippet"), {
      target: { value: log },
    });

    expect(screen.getByRole("button", { name: "Import 1 Rule" })).toBeInTheDocument();
    expect(screen.getByText("CEID=READY")).toBeInTheDocument();
    expect(screen.getByText("+ S2F49 W")).toBeInTheDocument();
  });

  it("shows a compact execution preview", () => {
    render(<RulesTabHarness />);

    expect(screen.getByText("Execution Preview")).toBeInTheDocument();
    expect(screen.getByText("Receive S2F41 RCMD TRANSFER")).toBeInTheDocument();
    expect(screen.getByText("Reply S2F42")).toBeInTheDocument();
    expect(screen.getByText("Send S6F11 W")).toBeInTheDocument();
  });
});
