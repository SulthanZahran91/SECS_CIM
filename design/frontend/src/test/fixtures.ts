import type { Snapshot } from "../types";

const baseSnapshot: Snapshot = {
  runtime: {
    listening: true,
    hsmsState: "SELECTED",
    configFile: "stocker-sim.yaml",
    dirty: true,
    restartRequired: false,
    lastError: "",
  },
  hsms: {
    mode: "passive",
    ip: "0.0.0.0",
    port: 5000,
    sessionId: 1,
    deviceId: 0,
    timers: {
      t3: 45,
      t5: 10,
      t6: 5,
      t7: 10,
      t8: 5,
    },
    handshake: {
      autoS1f13: true,
      autoS1f1: true,
      autoS2f25: false,
      autoHostStartup: false,
    },
  },
  device: {
    name: "stocker-A",
    protocol: "e88",
    mdln: "STOCKER-SIM",
    softrev: "1.0.0",
  },
  state: {
    mode: "online-remote",
    ports: {
      LP01: "occupied",
      LP02: "empty",
    },
    carriers: {
      CARR001: { location: "LP01" },
    },
  },
  rules: [
    {
      id: "rule-1",
      name: "accept transfer",
      enabled: true,
      match: { stream: 2, function: 41, rcmd: "TRANSFER" },
      conditions: [{ field: "carrier_exists", value: "CARR001" }],
      reply: { stream: 2, function: 42, ack: 0 },
      actions: [{ id: "action-1", delayMs: 300, type: "event", ceid: "TRANSFER_INITIATED" }],
    },
    {
      id: "rule-2",
      name: "reject when blocked",
      enabled: true,
      match: { stream: 2, function: 41, rcmd: "TRANSFER" },
      conditions: [{ field: "ports.LP01", value: "blocked" }],
      reply: { stream: 2, function: 42, ack: 3 },
      actions: [],
    },
  ],
  messages: [
    {
      id: "msg-1",
      timestamp: "14:32:05.210",
      direction: "IN",
      sf: "S2F41",
      label: "Remote Command: TRANSFER",
      matchedRule: "accept transfer",
      matchedRuleId: "rule-1",
      detail: {
        stream: 2,
        function: 41,
        wbit: true,
        body: "L:2\n  <A \"TRANSFER\">",
        rawSml: "S2F41 W L:2 <A \"TRANSFER\">",
      },
      evaluations: [{ field: "carrier_exists", expected: "CARR001", actual: "true", passed: true }],
    },
    {
      id: "msg-2",
      timestamp: "14:32:05.215",
      direction: "OUT",
      sf: "S2F42",
      label: "Remote Cmd Ack",
      matchedRule: "accept transfer",
      matchedRuleId: "rule-1",
      detail: {
        stream: 2,
        function: 42,
        wbit: false,
        body: "<B 0x00>",
        rawSml: "S2F42 <B 0x00>",
      },
      evaluations: [],
    },
  ],
};

export function makeSnapshot(): Snapshot {
  return structuredClone(baseSnapshot);
}
