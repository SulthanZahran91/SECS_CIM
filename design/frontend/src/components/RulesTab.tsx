import { useState } from "react";
import { ActionButton, Badge, CollapsibleSection, LabeledInput, LabeledSelect, SectionHeader, TogglePill } from "./ui";
import type { Rule, RuleAction, RuleCondition } from "../types";

type ImportedRuleAction = Omit<RuleAction, "id">;

export interface RuleTemplate {
  name: string;
  match: Rule["match"];
  conditions: RuleCondition[];
  reply: Rule["reply"];
  actions: ImportedRuleAction[];
}

interface RulesTabProps {
  rules: Rule[];
  expandedRuleId: string | null;
  onToggleRule: (id: string) => void;
  onCreateRule: () => void;
  onChangeRule: (rule: Rule) => void;
  onDuplicateRule: (id: string) => void;
  onDeleteRule: (id: string) => void;
  onMoveRule: (id: string, direction: "up" | "down") => void;
  onExportRule: (rule: Rule) => void;
  onImportRules: (templates: RuleTemplate[]) => void;
}

function toNumber(value: string): number {
  const parsed = Number.parseInt(value, 10);
  return Number.isNaN(parsed) ? 0 : parsed;
}

function createAction(): RuleAction {
  return {
    id: crypto.randomUUID(),
    delayMs: 0,
    type: "send",
    stream: 6,
    function: 11,
    wbit: true,
    body: 'L:1 <A "EVENT">',
  };
}

function sortActions(actions: RuleAction[]): RuleAction[] {
  return [...actions].sort((left, right) => left.delayMs - right.delayMs || left.id.localeCompare(right.id));
}

function sendSummary(action: Pick<RuleAction, "stream" | "function" | "wbit">): string {
  const stream = action.stream ?? 0;
  const fn = action.function ?? 0;
  return `S${stream}F${fn}${action.wbit ? " W" : ""}`;
}

function formatSf(stream: number, fn: number): string {
  return `S${stream}F${fn}`;
}

type PreviewStepKind = "trigger" | "reply" | "send";

interface PreviewStep {
  id: string;
  offset: string;
  kind: PreviewStepKind;
  title: string;
  summary: string;
  detail?: string;
}

interface RulePreset {
  label: string;
  description: string;
  name: string;
  match: Rule["match"];
  conditions: RuleCondition[];
  reply: Rule["reply"];
  actions: Omit<RuleAction, "id">[];
}

const RULE_PRESETS: RulePreset[] = [
  {
    label: "Accept transfer",
    description: "S2F41 TRANSFER with success ack and follow-up event.",
    name: "accept transfer",
    match: { stream: 2, function: 41, rcmd: "TRANSFER" },
    conditions: [{ field: "carrier_exists", value: "CARR001" }],
    reply: { stream: 2, function: 42, ack: 0 },
    actions: [{ delayMs: 300, type: "send", stream: 6, function: 11, wbit: true, body: 'L:1 <A "TRANSFER_INITIATED">' }],
  },
  {
    label: "Reject blocked",
    description: "Reject TRANSFER when the equipment reports blocked status.",
    name: "reject when blocked",
    match: { stream: 2, function: 41, rcmd: "TRANSFER" },
    conditions: [{ field: "status", value: "blocked" }],
    reply: { stream: 2, function: 42, ack: 3 },
    actions: [],
  },
  {
    label: "Loopback ack",
    description: "Respond to S2F25 loopback requests without extra side effects.",
    name: "ack loopback",
    match: { stream: 2, function: 25, rcmd: "" },
    conditions: [],
    reply: { stream: 2, function: 26, ack: 0 },
    actions: [],
  },
];

interface ParsedBlock {
  direction: "SEND" | "RECV";
  stream: number;
  fn: number;
  wbit: boolean;
  systemByte: number;
  timestampMs: number | null;
  label: string;
  rcmd: string;
  ceid: string;
  body: string;
}

function parseBlocks(text: string): ParsedBlock[] {
  const rawBlocks = text.split(/(?=@)/);
  const result: ParsedBlock[] = [];

  for (const block of rawBlocks) {
    const headerMatch = block.match(/\^SECS_II\^(RECV|SEND)/);
    if (!headerMatch) continue;

    const direction = headerMatch[1] as "SEND" | "RECV";
    // Require S{n}F{m}; W-bit and label are optional
    const sfMatch = block.match(/S(\d+)F(\d+)(\s+W,)?/);
    if (!sfMatch) continue;

    const stream = Number.parseInt(sfMatch[1], 10);
    const fn = Number.parseInt(sfMatch[2], 10);
    const wbit = !!sfMatch[3];
    const labelMatch = block.match(/S\d+F\d+[^-\n]*-\s*([^\[<\n]+)/);
    const label = labelMatch ? labelMatch[1].trim() : `S${stream}F${fn}`;

    const sbyteMatch = block.match(/\[SystemByte = (\d+)\]/);
    const systemByte = sbyteMatch ? Number.parseInt(sbyteMatch[1], 10) : -1;
    const timestampMs = parseTimestampMs(block);
    const body = extractBlockBody(block);
    const ceid = stream === 6 && fn === 11 ? extractLoggedCEID(block, body) : "";

    let rcmd = "";
    if (stream === 2 && (fn === 41 || fn === 49)) {
      const rcmdMatch = block.match(/<A,\d+\s+(\S+)\s*\[/);
      if (rcmdMatch) rcmd = rcmdMatch[1];
    }

    result.push({ direction, stream, fn, wbit, systemByte, timestampMs, label, rcmd, ceid, body });
  }

  return result;
}

function parseTimestampMs(text: string): number | null {
  const match = text.match(/(?:TransactionTime\s*:|@)\s*(\d{4})\/(\d{2})\/(\d{2}) (\d{2}):(\d{2}):(\d{2})\.(\d{3})/);
  if (!match) {
    return null;
  }

  return Date.UTC(
    Number.parseInt(match[1], 10),
    Number.parseInt(match[2], 10) - 1,
    Number.parseInt(match[3], 10),
    Number.parseInt(match[4], 10),
    Number.parseInt(match[5], 10),
    Number.parseInt(match[6], 10),
    Number.parseInt(match[7], 10),
  );
}

function extractBlockBody(block: string): string {
  const lines = block.split(/\r?\n/);
  const transactionIndex = lines.findIndex((line) => line.includes("TransactionTime"));
  if (transactionIndex === -1) {
    return "";
  }
  return lines.slice(transactionIndex + 1).join("\n").trim();
}

function extractLoggedCEID(block: string, body: string): string {
  const bodyMatch = body.match(/<[^,>]+,\d+\s+(.*?)\s+\[CEID\]>/i);
  if (bodyMatch) {
    return bodyMatch[1].trim();
  }

  const headerMatch = block.match(/\bCEID\s+([^\s.\]]+)/i);
  return headerMatch ? headerMatch[1].trim() : "";
}

interface LoggedBodyToken {
  type: string;
  length: number;
  value: string;
}

interface LoggedListState {
  count: number;
  children: string[];
}

function parseLoggedBody(body: string): string {
  const lines = body
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  if (lines.length === 0) {
    return "";
  }

  const stack: LoggedListState[] = [];
  let root = "";

  function attach(item: string) {
    if (stack.length === 0) {
      root = item;
      return;
    }
    stack[stack.length - 1].children.push(item);
  }

  for (const rawLine of lines) {
    const line = rawLine.replace(/\.\s*$/, "").trim();
    if (!line) {
      continue;
    }
    if (line === ">") {
      const completed = stack.pop();
      if (!completed) {
        continue;
      }
      attach(compactLoggedList(completed));
      continue;
    }
    if (!line.startsWith("<")) {
      continue;
    }

    const token = parseLoggedToken(line);
    if (!token) {
      continue;
    }

    if (token.type === "L") {
      if (token.length === 0) {
        attach("L:0");
        continue;
      }
      stack.push({ count: token.length, children: [] });
      continue;
    }

    const scalar = parseLoggedScalar(token);
    if (scalar) {
      attach(scalar);
    }
  }

  while (stack.length > 0) {
    attach(compactLoggedList(stack.pop()!));
  }

  return root;
}

function compactLoggedList(list: LoggedListState): string {
  return list.children.length > 0 ? `L:${list.count} ${list.children.join(" ")}` : `L:${list.count}`;
}

function parseLoggedToken(line: string): LoggedBodyToken | null {
  const inner = line.endsWith(">") ? line.slice(1, -1).trim() : line.slice(1).trim();
  const commaIndex = inner.indexOf(",");
  if (commaIndex === -1) {
    return null;
  }

  const type = inner.slice(0, commaIndex).trim().toUpperCase();
  let cursor = commaIndex + 1;
  while (cursor < inner.length && /\s/.test(inner[cursor])) {
    cursor += 1;
  }

  let lengthText = "";
  while (cursor < inner.length && /\d/.test(inner[cursor])) {
    lengthText += inner[cursor];
    cursor += 1;
  }
  if (!lengthText) {
    return null;
  }

  const remainder = inner.slice(cursor).trim();
  let value = remainder;
  if (value.startsWith("[") && value.endsWith("]")) {
    value = "";
  } else if (value.endsWith("]")) {
    const labelIndex = value.lastIndexOf(" [");
    if (labelIndex !== -1) {
      value = value.slice(0, labelIndex).trim();
    }
  }
  return {
    type,
    length: Number.parseInt(lengthText, 10),
    value,
  };
}

function parseLoggedScalar(token: LoggedBodyToken): string | null {
  const type = token.type === "U" ? "U4" : token.type === "I" ? "I4" : token.type;

  switch (type) {
    case "A":
      return `<A ${JSON.stringify(token.value)}>`;
    case "B":
      if (!token.value) {
        return "<B>";
      }
      return `<B ${token.value.split(/\s+/).map(formatBinaryToken).join(" ")}>`;
    case "BOOLEAN": {
      const normalized = token.value.toUpperCase();
      if (normalized === "TRUE" || normalized === "T" || normalized === "1") {
        return "<BOOLEAN TRUE>";
      }
      if (normalized === "FALSE" || normalized === "F" || normalized === "0") {
        return "<BOOLEAN FALSE>";
      }
      return null;
    }
    case "U1":
    case "U2":
    case "U4":
    case "I1":
    case "I2":
    case "I4":
      return token.value ? `<${type} ${token.value}>` : null;
    default:
      return null;
  }
}

function formatBinaryToken(raw: string): string {
  const value = raw.trim();
  if (!value) {
    return "0x00";
  }
  if (/^0x/i.test(value)) {
    return `0x${value.slice(2).toUpperCase().padStart(2, "0")}`;
  }
  if (/[A-Fa-f]/.test(value)) {
    return `0x${value.toUpperCase().padStart(2, "0")}`;
  }

  const parsed = Number.parseInt(value, 10);
  if (Number.isNaN(parsed)) {
    return value;
  }
  return `0x${parsed.toString(16).toUpperCase().padStart(2, "0")}`;
}

function buildImportedAction(triggerBlock: ParsedBlock, outboundBlock: ParsedBlock): ImportedRuleAction {
  const body = parseLoggedBody(outboundBlock.body);

  return {
    delayMs:
      triggerBlock.timestampMs !== null && outboundBlock.timestampMs !== null
        ? Math.max(0, outboundBlock.timestampMs - triggerBlock.timestampMs)
        : 0,
    type: "send",
    stream: outboundBlock.stream,
    function: outboundBlock.fn,
    wbit: outboundBlock.wbit,
    body: body || undefined,
  };
}

function buildImportedConditions(block: ParsedBlock): RuleCondition[] {
  if (block.ceid) {
    return [{ field: "CEID", value: block.ceid }];
  }

  return [];
}

export function parseLogRoutine(text: string): RuleTemplate[] {
  const blocks = parseBlocks(text);
  const templates: RuleTemplate[] = [];
  const seen = new Set<string>();

  for (let index = 0; index < blocks.length; index += 1) {
    const block = blocks[index];
    // Log is equipment-perspective. Equipment SEND W = simulator (host) receives and must reply.
    if (block.direction !== "SEND" || !block.wbit) continue;

    // Find the equipment RECV with matching SystemByte (that's the reply the host sent back).
    const replyIndex = blocks.findIndex((candidate, candidateIndex) => {
      return candidateIndex > index && candidate.direction === "RECV" && candidate.systemByte === block.systemByte;
    });
    if (replyIndex === -1) continue;
    const replyBlock = blocks[replyIndex];

    const conditions = buildImportedConditions(block);
    const key = `${block.stream}/${block.fn}/${block.rcmd}/${block.ceid}`;
    if (seen.has(key)) continue;
    seen.add(key);

    const nextTriggerIndex = blocks.findIndex((candidate, candidateIndex) => {
      return candidateIndex > replyIndex && candidate.direction === "SEND" && candidate.wbit;
    });
    const followUpEnd = nextTriggerIndex === -1 ? blocks.length : nextTriggerIndex;
    const actions = blocks
      .slice(replyIndex + 1, followUpEnd)
      .filter((candidate) => candidate.direction === "RECV")
      .map((candidate) => buildImportedAction(block, candidate));

    templates.push({
      name: block.label.toLowerCase(),
      match: { stream: block.stream, function: block.fn, rcmd: block.rcmd },
      conditions,
      reply: { stream: replyBlock.stream, function: replyBlock.fn, ack: 0 },
      actions,
    });
  }

  return templates;
}

function applyPreset(rule: Rule, preset: RulePreset): Rule {
  return {
    ...rule,
    name: preset.name,
    match: { ...preset.match },
    conditions: preset.conditions.map((condition) => ({ ...condition })),
    reply: { ...preset.reply },
    actions: preset.actions.map((action) => ({
      ...action,
      id: crypto.randomUUID(),
    })),
  };
}

function collectRuleIssues(rule: Rule): string[] {
  const issues: string[] = [];

  if (!rule.name.trim()) {
    issues.push("Rule name is required.");
  }

  rule.conditions.forEach((condition, index) => {
    if (!condition.field.trim() || !condition.value.trim()) {
      issues.push(`Condition ${index + 1} needs both a field and value.`);
    }
  });

  rule.actions.forEach((action, index) => {
    if (!(action.body ?? "").trim()) {
      issues.push(`Send action ${index + 1} needs a message body.`);
    }
  });

  return issues;
}

function buildPreviewSteps(rule: Rule): PreviewStep[] {
  const conditionsSummary =
    rule.conditions.length === 0
      ? "No conditions required"
      : `${rule.conditions.length} condition${rule.conditions.length === 1 ? "" : "s"} must pass`;
  const conditionDetail =
    rule.conditions.length === 0
      ? undefined
      : rule.conditions.map((condition) => `${condition.field || "field"} = ${condition.value || "value"}`).join(" • ");

  const previewSteps: PreviewStep[] = [
    {
      id: "trigger",
      offset: "IN",
      kind: "trigger",
      title: `Receive ${formatSf(rule.match.stream, rule.match.function)}${rule.match.rcmd ? ` RCMD ${rule.match.rcmd}` : ""}`,
      summary: conditionsSummary,
      detail: conditionDetail,
    },
    {
      id: "reply",
      offset: "+0ms",
      kind: "reply",
      title: `Reply ${formatSf(rule.reply.stream, rule.reply.function)}`,
      summary: rule.reply.ack === 0 ? "ACK 0 success response" : `ACK ${rule.reply.ack} reject response`,
      detail: "Immediate response emitted as soon as the trigger matches.",
    },
  ];

  sortActions(rule.actions).forEach((action) => {
    previewSteps.push({
      id: action.id,
      offset: `+${action.delayMs}ms`,
      kind: "send",
      title: `Send ${sendSummary(action)}`,
      summary: (action.body ?? "").trim() ? "Outbound SECS message" : "Outbound message body missing",
      detail: compactBody(action.body),
    });
  });

  return previewSteps;
}

function compactBody(body?: string): string | undefined {
  const compact = (body ?? "").replace(/\s+/g, " ").trim();
  if (!compact) {
    return undefined;
  }

  return compact.length > 84 ? `${compact.slice(0, 81)}...` : compact;
}

function templateKey(template: RuleTemplate): string {
  const conditionKey = template.conditions.map((condition) => `${condition.field}=${condition.value}`).join("|");
  return `${template.match.stream}/${template.match.function}/${template.match.rcmd}/${conditionKey}`;
}

function previewTone(kind: PreviewStepKind): "green" | "yellow" | "accent" | "neutral" {
  switch (kind) {
    case "trigger":
      return "accent";
    case "reply":
      return "green";
    case "send":
      return "yellow";
  }
}

function previewLabel(kind: PreviewStepKind): string {
  switch (kind) {
    case "trigger":
      return "Trigger";
    case "reply":
      return "Reply";
    case "send":
      return "Send";
  }
}

export function RulesTab({
  rules,
  expandedRuleId,
  onToggleRule,
  onCreateRule,
  onChangeRule,
  onDuplicateRule,
  onDeleteRule,
  onMoveRule,
  onExportRule,
  onImportRules,
}: RulesTabProps) {
  const [logText, setLogText] = useState("");
  const [logTemplates, setLogTemplates] = useState<RuleTemplate[]>([]);
  const enabledRules = rules.filter((rule) => rule.enabled).length;

  function handleLogTextChange(value: string) {
    setLogText(value);
    setLogTemplates(parseLogRoutine(value));
  }

  function handleImport() {
    onImportRules(logTemplates);
    setLogText("");
    setLogTemplates([]);
  }

  return (
    <div className="panel-scroll">
      <div className="panel-scroll-content">
        <SectionHeader
          right={
            <div className="section-actions">
              <Badge tone={enabledRules > 0 ? "green" : "neutral"}>{enabledRules} enabled</Badge>
              <ActionButton variant="accent" onClick={onCreateRule}>
                + New Rule
              </ActionButton>
            </div>
          }
        >
          {rules.length} rules
        </SectionHeader>

        <CollapsibleSection title="Import from Log" defaultOpen={false}>
          <label className="field-group">
            <span className="field-label">Paste log snippet</span>
            <textarea
              className="field-input mono payload-editor"
              value={logText}
              onChange={(event) => handleLogTextChange(event.target.value)}
              placeholder={"@2026/03/11^INFO^SECS_II^SEND\n2026/03/11 S6F11 W, Event Report Send [SystemByte = 15]\n<L,3 ...>.\n@2026/03/11^INFO^SECS_II^RECV\n2026/03/11 S6F12  Event Report Acknowledge [SystemByte = 15]\n<B,1 0>."}
              rows={6}
              spellCheck={false}
            />
          </label>
          {logTemplates.length > 0 ? (
            <>
              <div className="log-import-rule-list">
                {logTemplates.map((t) => (
                  <div className="log-import-rule-row" key={templateKey(t)}>
                    <span className="mono">S{t.match.stream}F{t.match.function}</span>
                    {t.match.rcmd ? <Badge tone="yellow">{t.match.rcmd}</Badge> : null}
                    {t.conditions.map((condition) => (
                      <Badge tone="accent" key={`${condition.field}/${condition.value}`}>
                        {condition.field}={condition.value}
                      </Badge>
                    ))}
                    <span className="log-import-arrow">→</span>
                    <span className="mono">S{t.reply.stream}F{t.reply.function}</span>
                    {t.actions.map((action, index) => (
                      <span className="log-import-followup mono" key={`${action.stream}-${action.function}-${index}`}>
                        + {sendSummary(action)}
                      </span>
                    ))}
                    <span className="log-import-label">{t.name}</span>
                  </div>
                ))}
              </div>
              <div className="log-import-preview">
                <ActionButton variant="accent" onClick={handleImport}>
                  Import {logTemplates.length} Rule{logTemplates.length === 1 ? "" : "s"}
                </ActionButton>
              </div>
            </>
          ) : logText.trim() ? (
            <div className="meta-note">No equipment SEND W exchanges found. Check the log format.</div>
          ) : null}
          <div className="meta-note">
            Paste equipment log text to detect message exchanges. Each equipment <code>SEND W</code> paired with a matching <code>RECV</code> (by SystemByte) becomes a rule.
          </div>
        </CollapsibleSection>

        <div className="rule-list">
          {rules.length === 0 ? <div className="empty-copy padded">Create a rule to start responding to host traffic.</div> : null}
          {rules.map((rule, index) => (
            <RuleCard
              key={rule.id}
              rule={rule}
              expanded={expandedRuleId === rule.id}
              isFirst={index === 0}
              isLast={index === rules.length - 1}
              onToggle={() => onToggleRule(rule.id)}
              onChange={onChangeRule}
              onDuplicate={() => onDuplicateRule(rule.id)}
              onDelete={() => onDeleteRule(rule.id)}
              onMoveUp={() => onMoveRule(rule.id, "up")}
              onMoveDown={() => onMoveRule(rule.id, "down")}
              onExport={() => onExportRule(rule)}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

interface RuleCardProps {
  rule: Rule;
  expanded: boolean;
  isFirst: boolean;
  isLast: boolean;
  onToggle: () => void;
  onChange: (rule: Rule) => void;
  onDuplicate: () => void;
  onDelete: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onExport: () => void;
}

function RuleCard({
  rule,
  expanded,
  isFirst,
  isLast,
  onToggle,
  onChange,
  onDuplicate,
  onDelete,
  onMoveUp,
  onMoveDown,
  onExport,
}: RuleCardProps) {
  const issues = collectRuleIssues(rule);
  const previewSteps = buildPreviewSteps(rule);

  function updateRule(nextRule: Rule) {
    onChange({
      ...nextRule,
      actions: sortActions(nextRule.actions),
    });
  }

  function updateConditions(nextConditions: RuleCondition[]) {
    updateRule({
      ...rule,
      conditions: nextConditions,
    });
  }

  function updateActions(nextActions: RuleAction[]) {
    updateRule({
      ...rule,
      actions: sortActions(nextActions),
    });
  }

  return (
    <article className={`rule-card ${expanded ? "expanded" : ""} ${rule.enabled ? "" : "disabled"}`}>
      <div className="rule-header" onClick={onToggle} role="button" tabIndex={0}>
        <span className={`rule-chevron ${expanded ? "open" : ""}`}>▶</span>
        <TogglePill
          checked={rule.enabled}
          onToggle={() =>
            updateRule({
              ...rule,
              enabled: !rule.enabled,
            })
          }
        />
        <Badge tone="yellow">
          S{rule.match.stream}F{rule.match.function}
        </Badge>
        <div className="rule-title-block">
          <span className="rule-name">{rule.name}</span>
          <span className="rule-match-copy mono">{rule.match.rcmd ? `RCMD ${rule.match.rcmd}` : "Any payload"}</span>
        </div>
        <Badge tone={rule.reply.ack === 0 ? "green" : "red"}>ACK {rule.reply.ack}</Badge>
        <span className="rule-summary">
          {rule.conditions.length} cond · {rule.actions.length} action{rule.actions.length === 1 ? "" : "s"}
        </span>
      </div>

      {expanded ? (
        <div className="rule-body">
          <section className="rule-section">
            <div className="field-row">
              <LabeledInput
                label="Rule Name"
                value={rule.name}
                onChange={(value) =>
                  updateRule({
                    ...rule,
                    name: value,
                  })
                }
                width="100%"
              />
            </div>
          </section>

          {issues.length > 0 ? (
            <CollapsibleSection
              title="Readiness"
              defaultOpen={false}
              right={<Badge tone="yellow">{issues.length} issue{issues.length === 1 ? "" : "s"}</Badge>}
            >
              <div className="rule-readiness-list">
                {issues.map((issue) => (
                  <div className="meta-note" key={issue}>{issue}</div>
                ))}
              </div>
            </CollapsibleSection>
          ) : null}

          <CollapsibleSection title="Presets" defaultOpen={false}>
            <div className="preset-row">
              {RULE_PRESETS.map((preset) => (
                <button
                  className="preset-card"
                  key={preset.label}
                  onClick={() => updateRule(applyPreset(rule, preset))}
                  type="button"
                >
                  <span className="preset-title">{preset.label}</span>
                  <span className="preset-copy">{preset.description}</span>
                </button>
              ))}
            </div>
          </CollapsibleSection>

          <CollapsibleSection
            title="Execution Preview"
            defaultOpen
            right={<Badge tone="teal">{previewSteps.length} steps</Badge>}
          >
            <div className="rule-preview-overview">
              <div className="rule-preview-overview-copy">
                <div className="rule-preview-overview-title">Expected flow</div>
                <div className="rule-preview-overview-text">
                  The simulator will evaluate the inbound trigger, reply immediately, then execute delayed side effects in timestamp order.
                </div>
              </div>
              <Badge tone={rule.actions.length > 0 ? "accent" : "neutral"}>
                {rule.actions.length} delayed effect{rule.actions.length === 1 ? "" : "s"}
              </Badge>
            </div>
            <div className="rule-preview-timeline">
              {previewSteps.map((step) => (
                <div className={`rule-preview-step ${step.kind}`} key={step.id}>
                  <div className="rule-preview-step-rail">
                    <span className={`rule-preview-step-dot ${step.kind}`} />
                  </div>
                  <div className="rule-preview-step-body">
                    <div className="rule-preview-step-head">
                      <Badge tone={previewTone(step.kind)}>{previewLabel(step.kind)}</Badge>
                      <span className="rule-preview-step-title">{step.title}</span>
                      <span className="rule-preview-step-offset mono">{step.offset}</span>
                    </div>
                    <div className="rule-preview-step-summary">{step.summary}</div>
                    {step.detail ? <div className="rule-preview-step-detail mono">{step.detail}</div> : null}
                  </div>
                </div>
              ))}
            </div>
          </CollapsibleSection>

          <section className="rule-section">
            <div className="rule-section-title">Match When Received</div>
            <div className="field-row">
              <LabeledInput
                label="Stream"
                value={rule.match.stream}
                onChange={(value) =>
                  updateRule({
                    ...rule,
                    match: { ...rule.match, stream: toNumber(value) },
                  })
                }
                width={70}
                type="number"
                mono
                min={0}
                max={127}
              />
              <LabeledInput
                label="Function"
                value={rule.match.function}
                onChange={(value) =>
                  updateRule({
                    ...rule,
                    match: { ...rule.match, function: toNumber(value) },
                  })
                }
                width={80}
                type="number"
                mono
                min={0}
                max={255}
              />
              <LabeledInput
                label="Remote CMD (RCMD)"
                value={rule.match.rcmd}
                onChange={(value) =>
                  updateRule({
                    ...rule,
                    match: { ...rule.match, rcmd: value },
                  })
                }
                width="100%"
                mono
              />
            </div>
          </section>

          <section className="rule-section">
            <div className="rule-section-header">
              <div className="rule-section-title">If Conditions Met</div>
              <ActionButton
                variant="accent"
                onClick={() => updateConditions([...rule.conditions, { field: "", value: "" }])}
              >
                + Add
              </ActionButton>
            </div>
            {rule.conditions.length === 0 ? (
              <div className="empty-copy">Always matches (no conditions).</div>
            ) : (
              <div className="stack-list">
                {rule.conditions.map((condition, index) => (
                  <div className="condition-row" key={`${rule.id}-condition-${index}`}>
                    <div className="field-group" style={{ flex: 1 }}>
                      <span className="field-label">Field</span>
                      <input
                        className="field-input mono"
                        value={condition.field}
                        onChange={(event) => {
                          const nextConditions = [...rule.conditions];
                          nextConditions[index] = { ...condition, field: event.target.value };
                          updateConditions(nextConditions);
                        }}
                        placeholder="e.g. DATA.RCMD"
                      />
                    </div>
                    <span className="condition-equals">=</span>
                    <div className="field-group" style={{ flex: 1 }}>
                      <span className="field-label">Value</span>
                      <input
                        className="field-input"
                        value={condition.value}
                        onChange={(event) => {
                          const nextConditions = [...rule.conditions];
                          nextConditions[index] = { ...condition, value: event.target.value };
                          updateConditions(nextConditions);
                        }}
                        placeholder="Expected value"
                      />
                    </div>
                    <button
                      className="icon-button danger"
                      onClick={() => {
                        const nextConditions = rule.conditions.filter((_, conditionIndex) => conditionIndex !== index);
                        updateConditions(nextConditions);
                      }}
                      type="button"
                      style={{ marginTop: 20 }}
                    >
                      ×
                    </button>
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="rule-section">
            <div className="rule-section-title">Then Reply</div>
            <div className="field-row">
              <LabeledInput
                label="Stream"
                value={rule.reply.stream}
                onChange={(value) =>
                  updateRule({
                    ...rule,
                    reply: { ...rule.reply, stream: toNumber(value) },
                  })
                }
                width={70}
                type="number"
                mono
              />
              <LabeledInput
                label="Function"
                value={rule.reply.function}
                onChange={(value) =>
                  updateRule({
                    ...rule,
                    reply: { ...rule.reply, function: toNumber(value) },
                  })
                }
                width={80}
                type="number"
                mono
              />
              <LabeledInput
                label="ACK Code"
                value={rule.reply.ack}
                onChange={(value) =>
                  updateRule({
                    ...rule,
                    reply: { ...rule.reply, ack: toNumber(value) },
                  })
                }
                width={90}
                type="number"
                mono
                hint={rule.reply.ack === 0 ? "success" : "reject"}
              />
            </div>
          </section>

          <section className="rule-section">
            <div className="rule-section-header">
              <div className="rule-section-title">Then Side Effects</div>
              <div className="button-row">
                <ActionButton variant="accent" onClick={() => updateActions([...rule.actions, createAction()])}>
                  + Message
                </ActionButton>
              </div>
            </div>
            {rule.actions.length === 0 ? (
              <div className="empty-copy">No side effects configured.</div>
            ) : (
              <div className="timeline-list">
                {rule.actions.map((action) => (
                  <div className="timeline-item" key={action.id}>
                    <div className={`timeline-dot ${action.type}`} />
                    <div className="timeline-delay">+{action.delayMs}ms</div>
                    <div className="timeline-editor">
                      <div className="field-row compact action-editor-row">
                        <LabeledInput
                          label="Delay (ms)"
                          value={action.delayMs}
                          onChange={(value) => {
                            const nextActions = rule.actions.map((item) =>
                              item.id === action.id ? { ...item, delayMs: toNumber(value) } : item,
                            );
                            updateActions(nextActions);
                          }}
                          width={90}
                          type="number"
                          mono
                        />
                        <div className="event-generator">
                          <div className="event-generator-head">
                            <Badge tone="yellow">{sendSummary(action)}</Badge>
                            <span className="event-generator-copy">Generic outbound SECS message</span>
                          </div>
                          <div className="field-row compact action-editor-row">
                            <LabeledInput
                              label="Stream"
                              value={action.stream ?? 0}
                              onChange={(value) => {
                                const nextActions = rule.actions.map((item) =>
                                  item.id === action.id ? { ...item, stream: toNumber(value) } : item,
                                );
                                updateActions(nextActions);
                              }}
                              width={80}
                              type="number"
                              mono
                              min={0}
                              max={127}
                            />
                            <LabeledInput
                              label="Function"
                              value={action.function ?? 0}
                              onChange={(value) => {
                                const nextActions = rule.actions.map((item) =>
                                  item.id === action.id ? { ...item, function: toNumber(value) } : item,
                                );
                                updateActions(nextActions);
                              }}
                              width={90}
                              type="number"
                              mono
                              min={0}
                              max={255}
                            />
                            <LabeledSelect
                              label="W-Bit"
                              value={String(action.wbit ?? false)}
                              onChange={(value) => {
                                const nextActions = rule.actions.map((item) =>
                                  item.id === action.id ? { ...item, wbit: value === "true" } : item,
                                );
                                updateActions(nextActions);
                              }}
                              options={["true", "false"]}
                              width={100}
                            />
                          </div>
                          <label className="field-group">
                            <span className="field-label">Body (SML)</span>
                            <textarea
                              className="field-input mono payload-editor"
                              value={action.body ?? ""}
                              onChange={(event) => {
                                const nextActions = rule.actions.map((item) =>
                                  item.id === action.id ? { ...item, body: event.target.value } : item,
                                );
                                updateActions(nextActions);
                              }}
                              placeholder={'L:2\n  <A "TRANSFER">\n  <I 1>'}
                              rows={6}
                              spellCheck={false}
                            />
                          </label>
                          <div className="meta-note">
                            Handwrite the outbound message directly. Supported body syntax matches the monitor style:
                            {" "}
                            <code>L:n</code>, <code>&lt;A "text"&gt;</code>, <code>&lt;I 1&gt;</code>,
                            {" "}
                            <code>&lt;I1 -1&gt;</code>, <code>&lt;I2 -2&gt;</code>, <code>&lt;I4 -3&gt;</code>,
                            {" "}
                            <code>&lt;U1 1&gt;</code>, <code>&lt;U2 2&gt;</code>, <code>&lt;U4 4&gt;</code>,
                            {" "}
                            <code>&lt;B 0x00&gt;</code>, and <code>&lt;BOOLEAN TRUE&gt;</code>.
                          </div>
                        </div>
                      </div>
                    </div>
                    <button
                      className="icon-button danger"
                      onClick={() => updateActions(rule.actions.filter((item) => item.id !== action.id))}
                      type="button"
                      style={{ marginTop: 22 }}
                    >
                      ×
                    </button>
                  </div>
                ))}
              </div>
            )}
          </section>

          <div className="button-row spread" style={{ borderTop: "1px solid var(--border)", paddingTop: 16 }}>
            <div className="button-row">
              <ActionButton variant="neutral" onClick={onMoveUp}>
                ↑ Up
              </ActionButton>
              <ActionButton variant="neutral" onClick={onMoveDown}>
                ↓ Down
              </ActionButton>
            </div>
            <div className="button-row">
              <ActionButton variant="neutral" onClick={onDuplicate}>
                Duplicate
              </ActionButton>
              <ActionButton variant="danger" onClick={onDelete}>
                Delete
              </ActionButton>
              <ActionButton variant="accent" onClick={onExport}>
                Export YAML
              </ActionButton>
            </div>
          </div>

          <div className="meta-note" style={{ marginTop: 12 }}>{isFirst ? "Priority: Highest" : isLast ? "Priority: Lowest (Fallback)" : "Matched in order"}</div>
        </div>
      ) : null}
    </article>
  );
}
