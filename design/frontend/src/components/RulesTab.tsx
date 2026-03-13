import { ActionButton, Badge, CollapsibleSection, LabeledInput, LabeledSelect, SectionHeader, TogglePill } from "./ui";
import type { Rule, RuleAction, RuleCondition } from "../types";

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
}

function toNumber(value: string): number {
  const parsed = Number.parseInt(value, 10);
  return Number.isNaN(parsed) ? 0 : parsed;
}

function createAction(type: RuleAction["type"]): RuleAction {
  return type === "send"
    ? {
        id: crypto.randomUUID(),
        delayMs: 0,
        type,
        stream: 6,
        function: 11,
        wbit: true,
        body: 'L:1 <A "EVENT">',
      }
    : {
        id: crypto.randomUUID(),
        delayMs: 0,
        type,
        target: "",
        value: "",
      };
}

function convertAction(type: RuleAction["type"], action: RuleAction): RuleAction {
  return type === "send"
    ? {
        id: action.id,
        delayMs: action.delayMs,
        type,
        stream: action.stream ?? 6,
        function: action.function ?? 11,
        wbit: action.wbit ?? true,
        body: action.body ?? "",
      }
    : {
        id: action.id,
        delayMs: action.delayMs,
        type,
        target: action.target ?? "",
        value: action.value ?? "",
      };
}

function sortActions(actions: RuleAction[]): RuleAction[] {
  return [...actions].sort((left, right) => left.delayMs - right.delayMs || left.id.localeCompare(right.id));
}

function sendSummary(action: RuleAction): string {
  const stream = action.stream ?? 0;
  const fn = action.function ?? 0;
  return `S${stream}F${fn}${action.wbit ? " W" : ""}`;
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
    description: "Reject TRANSFER when the source path is blocked.",
    name: "reject when blocked",
    match: { stream: 2, function: 41, rcmd: "TRANSFER" },
    conditions: [{ field: "ports.LP01", value: "blocked" }],
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
    if (action.type === "send" && !(action.body ?? "").trim()) {
      issues.push(`Send action ${index + 1} needs a message body.`);
    }
    if (action.type === "mutate" && !(action.target ?? "").trim()) {
      issues.push(`Mutate action ${index + 1} needs a target path.`);
    }
    if (action.type === "mutate" && !(action.value ?? "").trim()) {
      issues.push(`Mutate action ${index + 1} needs a new value.`);
    }
  });

  return issues;
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
}: RulesTabProps) {
  const enabledRules = rules.filter((rule) => rule.enabled).length;

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
                <ActionButton variant="accent" onClick={() => updateActions([...rule.actions, createAction("send")])}>
                  + Message
                </ActionButton>
                <ActionButton
                  variant="accent"
                  onClick={() => updateActions([...rule.actions, createAction("mutate")])}
                >
                  + Mutate
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
                        <LabeledSelect
                          label="Type"
                          value={action.type}
                          onChange={(value) => {
                            const nextActions = rule.actions.map((item) =>
                              item.id === action.id ? convertAction(value as RuleAction["type"], item) : item,
                            );
                            updateActions(nextActions);
                          }}
                          options={["send", "mutate"]}
                          width={100}
                        />
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
                        {action.type === "send" ? (
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
                        ) : (
                          <>
                            <LabeledInput
                              label="Target Path"
                              value={action.target ?? ""}
                              onChange={(value) => {
                                const nextActions = rule.actions.map((item) =>
                                  item.id === action.id ? { ...item, target: value } : item,
                                );
                                updateActions(nextActions);
                              }}
                              width={180}
                              mono
                            />
                            <LabeledInput
                              label="New Value"
                              value={action.value ?? ""}
                              onChange={(value) => {
                                const nextActions = rule.actions.map((item) =>
                                  item.id === action.id ? { ...item, value } : item,
                                );
                                updateActions(nextActions);
                              }}
                              width={140}
                            />
                          </>
                        )}
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
