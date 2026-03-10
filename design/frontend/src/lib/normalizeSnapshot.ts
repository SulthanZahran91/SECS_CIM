import type {
  ConditionEvaluation,
  MessageRecord,
  Rule,
  RuleAction,
  RuleCondition,
  Snapshot,
} from "../types";

function ensureArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

function ensureObject<T extends object>(value: T | null | undefined, fallback: T): T {
  return value && typeof value === "object" ? value : fallback;
}

function normalizeRule(rule: Rule): Rule {
  return {
    ...rule,
    conditions: ensureArray<RuleCondition>(rule.conditions),
    actions: ensureArray<RuleAction>(rule.actions).map(normalizeRuleAction),
  };
}

function normalizeRuleAction(action: RuleAction): RuleAction {
  const rawType = (action as RuleAction & { type?: string }).type;
  const type: RuleAction["type"] = rawType === "mutate" ? "mutate" : "send";
  return {
    ...action,
    type,
  };
}

function normalizeMessage(message: MessageRecord): MessageRecord {
  return {
    ...message,
    evaluations: ensureArray<ConditionEvaluation>(message.evaluations),
  };
}

export function normalizeSnapshot(snapshot: Snapshot): Snapshot {
  return {
    ...snapshot,
    hsms: {
      ...snapshot.hsms,
      handshake: {
        ...snapshot.hsms.handshake,
        autoHostStartup: Boolean(snapshot.hsms?.handshake?.autoHostStartup),
      },
    },
    runtime: {
      ...snapshot.runtime,
      restartRequired: Boolean(snapshot.runtime?.restartRequired),
    },
    state: {
      ...snapshot.state,
      ports: ensureObject(snapshot.state?.ports, {}),
      carriers: ensureObject(snapshot.state?.carriers, {}),
    },
    rules: ensureArray<Rule>(snapshot.rules).map(normalizeRule),
    messages: ensureArray<MessageRecord>(snapshot.messages).map(normalizeMessage),
  };
}
