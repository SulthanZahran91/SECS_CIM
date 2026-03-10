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
            if (action.type === "send") {
              lines.push("      stream: " + (action.stream ?? 0));
              lines.push("      function: " + (action.function ?? 0));
              lines.push("      wbit: " + Boolean(action.wbit));
              lines.push("      body: " + quote(action.body ?? ""));
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
