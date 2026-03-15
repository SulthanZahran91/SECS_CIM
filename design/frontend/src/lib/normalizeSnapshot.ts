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

function normalizeAction(action: RuleAction): RuleAction {
  return { ...action, type: "send" };
}

function normalizeRule(rule: Rule): Rule {
  return {
    ...rule,
    conditions: ensureArray<RuleCondition>(rule.conditions),
    actions: ensureArray<RuleAction>(rule.actions).map(normalizeAction),
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
    rules: ensureArray<Rule>(snapshot.rules).map(normalizeRule),
    messages: ensureArray<MessageRecord>(snapshot.messages).map(normalizeMessage),
  };
}
