import { render, screen } from "@testing-library/react";
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
  it("edits event actions through the generator-style report builder", async () => {
    const user = userEvent.setup();

    render(<RulesTabHarness />);

    expect(screen.getByText("Generator-style event declaration")).toBeInTheDocument();
    expect(screen.getByDisplayValue("TRANSFER_INITIATED")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "+ Report" }));
    const rptidInput = await screen.findByLabelText("RPTID");
    await user.type(rptidInput, "5001");

    await user.click(screen.getByRole("button", { name: "+ Variable" }));
    const vidInput = await screen.findByPlaceholderText("e.g. 100");
    const valueInput = await screen.findByPlaceholderText('e.g. A:LP01 or U4:100');

    await user.type(vidInput, "100");
    await user.type(valueInput, "A:LP01");

    expect(screen.getByDisplayValue("5001")).toBeInTheDocument();
    expect(screen.getByDisplayValue("100")).toBeInTheDocument();
    expect(screen.getByDisplayValue("A:LP01")).toBeInTheDocument();
    expect(screen.getByText(/1 RPT/)).toBeInTheDocument();
    expect(screen.getByText(/1 VID/)).toBeInTheDocument();
  });
});
