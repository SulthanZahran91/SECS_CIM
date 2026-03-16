import type {
  ConditionEvaluation,
  HandshakeConfig,
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

function normalizeHostStartupProfile(handshake: Partial<HandshakeConfig> | undefined): string {
  const rawProfile = String(handshake?.hostStartupProfile ?? "").trim().toLowerCase();
  switch (rawProfile) {
    case "stocker":
    case "conveyor":
      return rawProfile;
    case "disabled":
    case "none":
    case "off":
      return "disabled";
    default:
      return handshake?.autoHostStartup ? "stocker" : "disabled";
  }
}

export function normalizeSnapshot(snapshot: Snapshot): Snapshot {
  const hostStartupProfile = normalizeHostStartupProfile(snapshot.hsms?.handshake);
  return {
    ...snapshot,
    hsms: {
      ...snapshot.hsms,
      handshake: {
        ...snapshot.hsms.handshake,
        autoHostStartup: hostStartupProfile !== "disabled",
        hostStartupProfile,
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
