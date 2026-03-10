import { ActionButton, Badge, LabeledInput, LabeledSelect, SectionHeader, TogglePill } from "./ui";
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
  return type === "event"
    ? {
        id: crypto.randomUUID(),
        delayMs: 0,
        type,
        ceid: "",
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
  return type === "event"
    ? {
        id: action.id,
        delayMs: action.delayMs,
        type,
        ceid: action.ceid ?? "",
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
  return (
    <div className="panel-scroll">
      <SectionHeader right={<ActionButton variant="accent" onClick={onCreateRule}>+ New Rule</ActionButton>}>
        {rules.length} rules
      </SectionHeader>
      <div className="rule-list">
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
        <span className="rule-name">{rule.name}</span>
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
                <ActionButton variant="accent" onClick={() => updateActions([...rule.actions, createAction("event")])}>
                  + Event
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
                      <div className="field-row compact" style={{ flexWrap: "wrap" }}>
                        <LabeledSelect
                          label="Type"
                          value={action.type}
                          onChange={(value) => {
                            const nextActions = rule.actions.map((item) =>
                              item.id === action.id ? convertAction(value as RuleAction["type"], item) : item,
                            );
                            updateActions(nextActions);
                          }}
                          options={["event", "mutate"]}
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
                        {action.type === "event" ? (
                          <LabeledInput
                            label="CEID"
                            value={action.ceid ?? ""}
                            onChange={(value) => {
                              const nextActions = rule.actions.map((item) =>
                                item.id === action.id ? { ...item, ceid: value } : item,
                              );
                              updateActions(nextActions);
                            }}
                            width={160}
                            mono
                          />
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
