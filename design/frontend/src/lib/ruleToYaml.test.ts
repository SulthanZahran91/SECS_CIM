import { describe, expect, it } from "vitest";
import { ruleToYaml } from "./ruleToYaml";
import { makeSnapshot } from "../test/fixtures";

describe("ruleToYaml", () => {
  it("serializes conditions, reply, and mixed actions", () => {
    const rule = makeSnapshot().rules[0];
    rule.actions = [
      {
        id: "action-1",
        delayMs: 300,
        type: "send",
        stream: 6,
        function: 11,
        wbit: true,
        body: 'L:2 <A "TRANSFER_INITIATED"> <I 7>',
      },
      { id: "action-2", delayMs: 1200, type: "mutate", target: "ports.LP01", value: "empty" },
    ];

    const yaml = ruleToYaml(rule);

    expect(yaml).toContain('- name: "accept transfer"');
    expect(yaml).toContain('    rcmd: "TRANSFER"');
    expect(yaml).toContain('    - field: "carrier_exists"');
    expect(yaml).toContain("      stream: 6");
    expect(yaml).toContain("      function: 11");
    expect(yaml).toContain("      wbit: true");
    expect(yaml).toContain('      body: "L:2 <A \\"TRANSFER_INITIATED\\"> <I 7>"');
    expect(yaml).toContain('      target: "ports.LP01"');
    expect(yaml).toContain('      value: "empty"');
  });

  it("renders empty conditions and actions as explicit arrays", () => {
    const rule = makeSnapshot().rules[1];

    const yaml = ruleToYaml(rule);

    expect(yaml).toContain("  actions: []");
    expect(yaml).not.toContain("  conditions: []");
  });
});
