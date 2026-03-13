export interface Snapshot {
  runtime: RuntimeState;
  hsms: HsmsConfig;
  device: DeviceConfig;
  state: StateSnapshot;
  rules: Rule[];
  messages: MessageRecord[];
}

export interface RuntimeState {
  listening: boolean;
  hsmsState: string;
  configFile: string;
  dirty: boolean;
  restartRequired: boolean;
  lastError?: string;
}

export interface HsmsConfig {
  mode: string;
  ip: string;
  port: number;
  sessionId: number;
  deviceId: number;
  timers: HsmsTimers;
  handshake: HandshakeConfig;
}

export interface HsmsTimers {
  t3: number;
  t5: number;
  t6: number;
  t7: number;
  t8: number;
}

export interface HandshakeConfig {
  autoS1f13: boolean;
  autoS1f1: boolean;
  autoS2f25: boolean;
  autoHostStartup: boolean;
}

export interface DeviceConfig {
  name: string;
  protocol: string;
  mdln: string;
  softrev: string;
}

export interface StateSnapshot {
  mode: string;
  ports: Record<string, string>;
  carriers: Record<string, CarrierState>;
}

export interface CarrierState {
  location: string;
}

export interface Rule {
  id: string;
  name: string;
  enabled: boolean;
  match: RuleMatch;
  conditions: RuleCondition[];
  reply: RuleReply;
  actions: RuleAction[];
}

export interface RuleMatch {
  stream: number;
  function: number;
  rcmd: string;
}

export interface RuleCondition {
  field: string;
  value: string;
}

export interface RuleReply {
  stream: number;
  function: number;
  ack: number;
}

export interface RuleAction {
  id: string;
  delayMs: number;
  type: "send" | "mutate";
  stream?: number;
  function?: number;
  wbit?: boolean;
  body?: string;
  target?: string;
  value?: string;
}

export interface MessageRecord {
  id: string;
  timestamp: string;
  direction: "IN" | "OUT";
  sf: string;
  label: string;
  matchedRule?: string;
  matchedRuleId?: string;
  detail: MessageDetail;
  evaluations?: ConditionEvaluation[];
}

export interface MessageDetail {
  stream: number;
  function: number;
  wbit: boolean;
  body: string;
  rawSml: string;
}

export interface ConditionEvaluation {
  field: string;
  expected: string;
  actual: string;
  passed: boolean;
}

export type LeftTab = "rules" | "hsms";
export type DetailTab = "decoded" | "raw" | "rule";
