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
        type: "event",
        ceid: "TRANSFER_INITIATED",
        reports: [
          {
            rptid: "5001",
            variables: [{ vid: "100", value: "A:LP01" }],
          },
        ],
      },
      { id: "action-2", delayMs: 1200, type: "mutate", target: "ports.LP01", value: "empty" },
    ];

    const yaml = ruleToYaml(rule);

    expect(yaml).toContain('- name: "accept transfer"');
    expect(yaml).toContain('    rcmd: "TRANSFER"');
    expect(yaml).toContain('    - field: "carrier_exists"');
    expect(yaml).toContain('      ceid: "TRANSFER_INITIATED"');
    expect(yaml).toContain('      reports:');
    expect(yaml).toContain('        - rptid: "5001"');
    expect(yaml).toContain('            - vid: "100"');
    expect(yaml).toContain('              value: "A:LP01"');
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
