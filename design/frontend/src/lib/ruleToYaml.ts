import type { Rule } from "../types";

function quote(value: string): string {
  return JSON.stringify(value);
}

export function ruleToYaml(rule: Rule): string {
  const conditionLines =
    rule.conditions.length === 0
      ? ["  conditions: []"]
      : [
          "  conditions:",
          ...rule.conditions.flatMap((condition) => [
            "    - field: " + quote(condition.field),
            "      value: " + quote(condition.value),
          ]),
        ];

  const actionLines =
    rule.actions.length === 0
      ? ["  actions: []"]
      : [
          "  actions:",
          ...rule.actions.flatMap((action) => {
            const lines = [
              "    - delay_ms: " + action.delayMs,
              "      type: " + quote(action.type),
            ];
            if (action.type === "event") {
              lines.push("      ceid: " + quote(action.ceid ?? ""));
              if ((action.reports?.length ?? 0) > 0) {
                lines.push("      reports:");
                for (const report of action.reports ?? []) {
                  lines.push("        - rptid: " + quote(report.rptid ?? ""));
                  if (report.variables.length === 0) {
                    lines.push("          variables: []");
                    continue;
                  }

                  lines.push("          variables:");
                  for (const variable of report.variables) {
                    lines.push("            - vid: " + quote(variable.vid ?? ""));
                    lines.push("              value: " + quote(variable.value ?? ""));
                  }
                }
              }
            } else {
              lines.push("      target: " + quote(action.target ?? ""));
              lines.push("      value: " + quote(action.value ?? ""));
            }
            return lines;
          }),
        ];

  return [
    "- name: " + quote(rule.name),
    "  enabled: " + rule.enabled,
    "  match:",
    "    stream: " + rule.match.stream,
    "    function: " + rule.match.function,
    "    rcmd: " + quote(rule.match.rcmd),
    ...conditionLines,
    "  reply:",
    "    stream: " + rule.reply.stream,
    "    function: " + rule.reply.function,
    "    ack: " + rule.reply.ack,
    ...actionLines,
  ].join("\n");
}
