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
  it("edits event actions through the generator-style report builder", async () => {
    const user = userEvent.setup();

    render(<RulesTabHarness />);

    expect(screen.getByText("Actual Event Report Send structure")).toBeInTheDocument();
    expect(screen.getByDisplayValue("TRANSFER_INITIATED")).toBeInTheDocument();
    expect(screen.getByDisplayValue("U4:0")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "+ Report" }));
    const rptidInput = await screen.findByLabelText("RPTID Item");
    await user.type(rptidInput, "5001");

    await user.click(screen.getByRole("button", { name: "+ Value" }));
    const valueInput = await screen.findByPlaceholderText('e.g. A:LP01, U4:100, or L:[U4:1, A:"LP01"]');

    fireEvent.change(valueInput, { target: { value: 'L:[U4:1, A:"LP01"]' } });

    expect(screen.getByDisplayValue("5001")).toBeInTheDocument();
    expect(screen.getByDisplayValue('L:[U4:1, A:"LP01"]')).toBeInTheDocument();
    expect(screen.getByText(/1 RPT/)).toBeInTheDocument();
    expect(screen.getByText(/1 V/)).toBeInTheDocument();
  });
});
