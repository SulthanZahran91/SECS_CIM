import { ActionButton, Badge, LabeledInput, LabeledSelect, SectionHeader, TogglePill } from "./ui";
import type { Rule, RuleAction, RuleActionReport, RuleActionVariable, RuleCondition } from "../types";

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
        reports: [],
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
        reports: action.reports ?? [],
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

function eventSummary(action: RuleAction): string {
  const reportCount = action.reports?.length ?? 0;
  const variableCount = (action.reports ?? []).reduce((total, report) => total + report.variables.length, 0);
  const ceid = action.ceid?.trim() || "unset";
  if (reportCount === 0) {
    return `S6F11 CEID=${ceid}`;
  }

  return `S6F11 CEID=${ceid} · ${reportCount} RPT · ${variableCount} VID`;
}

function createReport(): RuleActionReport {
  return {
    rptid: "",
    variables: [],
  };
}

function createVariable(): RuleActionVariable {
  return {
    vid: "",
    value: "",
  };
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
                          <div className="event-generator">
                            <div className="event-generator-head">
                              <Badge tone="yellow">S6F11</Badge>
                              <span className="event-generator-copy">Generator-style event declaration</span>
                              <span className="event-generator-preview">{eventSummary(action)}</span>
                            </div>
                            <div className="field-row compact" style={{ flexWrap: "wrap" }}>
                              <LabeledInput
                                label="CEID"
                                value={action.ceid ?? ""}
                                onChange={(value) => {
                                  const nextActions = rule.actions.map((item) =>
                                    item.id === action.id ? { ...item, ceid: value } : item,
                                  );
                                  updateActions(nextActions);
                                }}
                                width={180}
                                mono
                              />
                            </div>
                            <div className="event-generator-toolbar">
                              <div className="rule-section-title" style={{ marginBottom: 0 }}>Reports</div>
                              <ActionButton
                                variant="accent"
                                onClick={() => {
                                  const nextActions = rule.actions.map((item) =>
                                    item.id === action.id
                                      ? {
                                          ...item,
                                          reports: [...(item.reports ?? []), createReport()],
                                        }
                                      : item,
                                  );
                                  updateActions(nextActions);
                                }}
                              >
                                + Report
                              </ActionButton>
                            </div>
                            {(action.reports?.length ?? 0) === 0 ? (
                              <div className="empty-copy">CEID-only event. Add a report to declare RPTID and VID values.</div>
                            ) : (
                              <div className="stack-list">
                                {(action.reports ?? []).map((report, reportIndex) => (
                                  <div className="event-report-card" key={`${action.id}-report-${reportIndex}`}>
                                    <div className="field-row compact" style={{ flexWrap: "wrap" }}>
                                      <LabeledInput
                                        label="RPTID"
                                        value={report.rptid}
                                        onChange={(value) => {
                                          const nextActions = rule.actions.map((item) => {
                                            if (item.id !== action.id) {
                                              return item;
                                            }

                                            const nextReports = [...(item.reports ?? [])];
                                            nextReports[reportIndex] = { ...report, rptid: value };
                                            return { ...item, reports: nextReports };
                                          });
                                          updateActions(nextActions);
                                        }}
                                        width={180}
                                        mono
                                      />
                                      <ActionButton
                                        variant="danger"
                                        onClick={() => {
                                          const nextActions = rule.actions.map((item) =>
                                            item.id === action.id
                                              ? {
                                                  ...item,
                                                  reports: (item.reports ?? []).filter((_, index) => index !== reportIndex),
                                                }
                                              : item,
                                          );
                                          updateActions(nextActions);
                                        }}
                                      >
                                        Remove Report
                                      </ActionButton>
                                    </div>
                                    <div className="event-generator-toolbar">
                                      <div className="rule-section-title" style={{ marginBottom: 0 }}>Variables</div>
                                      <ActionButton
                                        variant="accent"
                                        onClick={() => {
                                          const nextActions = rule.actions.map((item) => {
                                            if (item.id !== action.id) {
                                              return item;
                                            }

                                            const nextReports = [...(item.reports ?? [])];
                                            nextReports[reportIndex] = {
                                              ...report,
                                              variables: [...report.variables, createVariable()],
                                            };
                                            return { ...item, reports: nextReports };
                                          });
                                          updateActions(nextActions);
                                        }}
                                      >
                                        + Variable
                                      </ActionButton>
                                    </div>
                                    {report.variables.length === 0 ? (
                                      <div className="empty-copy">No VID declarations in this report yet.</div>
                                    ) : (
                                      <div className="stack-list">
                                        {report.variables.map((variable, variableIndex) => (
                                          <div
                                            className="condition-row event-variable-row"
                                            key={`${action.id}-report-${reportIndex}-variable-${variableIndex}`}
                                          >
                                            <div className="field-group" style={{ width: 180 }}>
                                              <span className="field-label">VID</span>
                                              <input
                                                className="field-input mono"
                                                value={variable.vid}
                                                onChange={(event) => {
                                                  const nextActions = rule.actions.map((item) => {
                                                    if (item.id !== action.id) {
                                                      return item;
                                                    }

                                                    const nextReports = [...(item.reports ?? [])];
                                                    const nextVariables = [...report.variables];
                                                    nextVariables[variableIndex] = { ...variable, vid: event.target.value };
                                                    nextReports[reportIndex] = { ...report, variables: nextVariables };
                                                    return { ...item, reports: nextReports };
                                                  });
                                                  updateActions(nextActions);
                                                }}
                                                placeholder="e.g. 100"
                                              />
                                            </div>
                                            <div className="field-group" style={{ flex: 1 }}>
                                              <span className="field-label">Value</span>
                                              <input
                                                className="field-input mono"
                                                value={variable.value}
                                                onChange={(event) => {
                                                  const nextActions = rule.actions.map((item) => {
                                                    if (item.id !== action.id) {
                                                      return item;
                                                    }

                                                    const nextReports = [...(item.reports ?? [])];
                                                    const nextVariables = [...report.variables];
                                                    nextVariables[variableIndex] = { ...variable, value: event.target.value };
                                                    nextReports[reportIndex] = { ...report, variables: nextVariables };
                                                    return { ...item, reports: nextReports };
                                                  });
                                                  updateActions(nextActions);
                                                }}
                                                placeholder='e.g. A:LP01 or U4:100'
                                              />
                                            </div>
                                            <button
                                              className="icon-button danger"
                                              onClick={() => {
                                                const nextActions = rule.actions.map((item) => {
                                                  if (item.id !== action.id) {
                                                    return item;
                                                  }

                                                  const nextReports = [...(item.reports ?? [])];
                                                  nextReports[reportIndex] = {
                                                    ...report,
                                                    variables: report.variables.filter((_, index) => index !== variableIndex),
                                                  };
                                                  return { ...item, reports: nextReports };
                                                });
                                                updateActions(nextActions);
                                              }}
                                              type="button"
                                            >
                                              ×
                                            </button>
                                          </div>
                                        ))}
                                      </div>
                                    )}
                                  </div>
                                ))}
                              </div>
                            )}
                            <div className="meta-note">
                              Values default to <code>U4</code> when numeric and <code>A</code> otherwise. Prefix with
                              {" "}
                              <code>A:</code>, <code>U1:</code>, <code>U2:</code>, <code>U4:</code>, <code>BOOL:</code>,
                              {" "}
                              or <code>B:</code> to force a SECS type.
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
