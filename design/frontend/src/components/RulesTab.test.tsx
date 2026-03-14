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
    expect(templates[0].reply).toEqual({ stream: 6, function: 12, ack: 0 });
    expect(templates[1].match).toEqual({ stream: 1, function: 13, rcmd: "" });
    expect(templates[1].reply).toEqual({ stream: 1, function: 14, ack: 0 });
  });

  it("shows a compact execution preview that updates with delayed mutations", async () => {
    const user = userEvent.setup();

    render(<RulesTabHarness />);

    expect(screen.getByText("Execution Preview")).toBeInTheDocument();
    expect(screen.getByText("Receive S2F41 RCMD TRANSFER")).toBeInTheDocument();
    expect(screen.getByText("Reply S2F42")).toBeInTheDocument();
    expect(screen.getByText("Send S6F11 W")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "+ Mutate" }));

    fireEvent.change(screen.getByLabelText("Target Path"), {
      target: { value: "carriers.CARR001.location" },
    });
    fireEvent.change(screen.getByLabelText("New Value"), {
      target: { value: "SHELF_A01" },
    });

    expect(screen.getByText("Mutate runtime state")).toBeInTheDocument();
    expect(screen.getByText("Set carriers.CARR001.location -> SHELF_A01")).toBeInTheDocument();
  });
});
