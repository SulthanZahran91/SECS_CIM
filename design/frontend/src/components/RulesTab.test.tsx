import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { RulesTab } from "./RulesTab";
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
});
