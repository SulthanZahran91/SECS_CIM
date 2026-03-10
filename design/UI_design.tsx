import { useState } from "react";

const colors = {
  bg: "#1e1e2e", surface: "#252536", surfaceAlt: "#2a2a3d", surfaceHover: "#30304a",
  border: "#353548", borderActive: "#6c6cff", text: "#cdd6f4", textDim: "#6c7086",
  textBright: "#f0f0ff", accent: "#6c6cff", accentDim: "#4a4a9e", green: "#a6e3a1",
  greenDim: "#2a3a2a", red: "#f38ba8", redDim: "#3a2a2e", yellow: "#f9e2af",
  yellowDim: "#3a362a", blue: "#89b4fa", teal: "#94e2d5", orange: "#fab387",
};

const MOCK_RULES = [
  {
    id: 1, name: "accept transfer", enabled: true,
    match: { stream: 2, function: 41, rcmd: "TRANSFER" },
    conditions: [{ field: "carrier_exists", value: "CARR001" }, { field: "source_equals", value: "LP01" }],
    reply: { stream: 2, function: 42, ack: 0 },
    events: [
      { delay: 300, type: "event", ceid: "TRANSFER_INITIATED" },
      { delay: 1200, type: "mutate", target: "carriers.CARR001.location", value: "SHELF_A01" },
      { delay: 1200, type: "mutate", target: "ports.LP01", value: "empty" },
      { delay: 1200, type: "event", ceid: "TRANSFER_COMPLETED" },
    ],
  },
  {
    id: 2, name: "reject when blocked", enabled: true,
    match: { stream: 2, function: 41, rcmd: "TRANSFER" },
    conditions: [{ field: "ports.LP01", value: "blocked" }],
    reply: { stream: 2, function: 42, ack: 3 },
    events: [],
  },
  {
    id: 3, name: "carrier locate", enabled: false,
    match: { stream: 2, function: 41, rcmd: "LOCATE" },
    conditions: [],
    reply: { stream: 2, function: 42, ack: 0 },
    events: [{ delay: 200, type: "event", ceid: "LOCATE_COMPLETE" }],
  },
];

const HSMS_CONFIG = {
  mode: "passive", ip: "0.0.0.0", port: 5000, sessionId: 1, deviceId: 0,
  t3: 45, t5: 10, t6: 5, t7: 10, t8: 5,
};

const MOCK_STATE = {
  device: "stocker-A", mode: "online-remote",
  ports: { LP01: "occupied", LP02: "empty", LP03: "blocked" },
  carriers: { CARR001: { location: "LP01" }, CARR002: { location: "SHELF_A01" } },
};

const MOCK_MESSAGES = [
  { id: 1, ts: "14:32:01.003", dir: "IN", sf: "S1F13", label: "Establish Comm", matched: null, detail: { stream: 1, function: 13, wbit: true, body: "L:0" } },
  { id: 2, ts: "14:32:01.015", dir: "OUT", sf: "S1F14", label: "Establish Comm Ack", matched: null, detail: { stream: 1, function: 14, wbit: false, body: 'L:2 <B 0x00> L:2 <A "MDLN"> <A "1.0">' } },
  { id: 3, ts: "14:32:05.210", dir: "IN", sf: "S2F41", label: "Remote Command", matched: "accept transfer", detail: { stream: 2, function: 41, wbit: true, body: 'L:2 <A "TRANSFER"> L:2 L:2 <A "CarrierID"> <A "CARR001"> L:2 <A "SourcePort"> <A "LP01">' } },
  { id: 4, ts: "14:32:05.215", dir: "OUT", sf: "S2F42", label: "Remote Cmd Ack", matched: "accept transfer", detail: { stream: 2, function: 42, wbit: false, body: "L:2 <B 0x00> L:0" } },
  { id: 5, ts: "14:32:05.515", dir: "OUT", sf: "S6F11", label: "TRANSFER_INITIATED", matched: "accept transfer", detail: { stream: 6, function: 11, wbit: true, body: 'L:3 <U4 1001> <U4 5001> L:1 L:2 <U4 100> <A "LP01">' } },
  { id: 6, ts: "14:32:06.415", dir: "OUT", sf: "S6F11", label: "TRANSFER_COMPLETED", matched: "accept transfer", detail: { stream: 6, function: 11, wbit: true, body: 'L:3 <U4 1002> <U4 5002> L:1 L:2 <U4 100> <A "SHELF_A01">' } },
  { id: 7, ts: "14:32:06.420", dir: "IN", sf: "S6F12", label: "Event Ack", matched: null, detail: { stream: 6, function: 12, wbit: false, body: "<B 0x00>" } },
];

const Badge = ({ children, color, bg }) => (
  <span style={{ fontSize: 10, fontWeight: 600, padding: "1px 6px", borderRadius: 3, background: bg, color, letterSpacing: 0.5, whiteSpace: "nowrap" }}>{children}</span>
);

const Tab = ({ active, children, onClick, icon }) => (
  <button onClick={onClick} style={{
    background: active ? colors.surface : "transparent", color: active ? colors.textBright : colors.textDim,
    border: "none", borderBottom: active ? `2px solid ${colors.accent}` : "2px solid transparent",
    padding: "6px 12px", fontSize: 11, fontWeight: 500, cursor: "pointer", fontFamily: "inherit",
    display: "flex", alignItems: "center", gap: 4,
  }}>{icon && <span style={{ fontSize: 13 }}>{icon}</span>}{children}</button>
);

const SectionHeader = ({ children, right }) => (
  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "6px 10px", background: colors.surfaceAlt, borderBottom: `1px solid ${colors.border}`, fontSize: 10, fontWeight: 600, color: colors.textDim, textTransform: "uppercase", letterSpacing: 0.8 }}>
    <span>{children}</span>{right}
  </div>
);

const Field = ({ label, value, width, mono, hint }) => (
  <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
    <label style={{ fontSize: 10, color: colors.textDim, fontWeight: 500 }}>{label}</label>
    <input readOnly value={value} style={{
      background: colors.bg, border: `1px solid ${colors.border}`, borderRadius: 3, color: mono ? colors.teal : colors.textBright,
      padding: "4px 8px", fontSize: 11, fontFamily: "inherit", width: width || "100%", boxSizing: "border-box",
      outline: "none",
    }} onFocus={e => e.target.style.borderColor = colors.borderActive}
       onBlur={e => e.target.style.borderColor = colors.border} />
    {hint && <span style={{ fontSize: 9, color: colors.textDim }}>{hint}</span>}
  </div>
);

const Select = ({ label, value, options, width }) => (
  <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
    <label style={{ fontSize: 10, color: colors.textDim, fontWeight: 500 }}>{label}</label>
    <select value={value} readOnly style={{
      background: colors.bg, border: `1px solid ${colors.border}`, borderRadius: 3, color: colors.textBright,
      padding: "4px 8px", fontSize: 11, fontFamily: "inherit", width: width || "100%",
    }}>
      {options.map(o => <option key={o} value={o}>{o}</option>)}
    </select>
  </div>
);

const ActionTag = ({ type, children }) => {
  const c = type === "event" ? { fg: colors.yellow, bg: colors.yellowDim, icon: "⚡" }
    : type === "mutate" ? { fg: colors.orange, bg: "#3a2d1e", icon: "✎" }
    : { fg: colors.blue, bg: colors.accentDim, icon: "↩" };
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 6, padding: "3px 8px", background: c.bg, borderRadius: 3, fontSize: 10, color: c.fg }}>
      <span>{c.icon}</span>{children}
    </div>
  );
};

function RuleCard({ rule, expanded, onToggle }) {
  return (
    <div style={{ borderBottom: `1px solid ${colors.border}`, background: expanded ? colors.surfaceAlt + "88" : "transparent" }}>
      {/* Rule header */}
      <div onClick={onToggle} style={{ display: "flex", alignItems: "center", gap: 8, padding: "8px 10px", cursor: "pointer" }}
        onMouseEnter={e => { if (!expanded) e.currentTarget.style.background = colors.surfaceHover; }}
        onMouseLeave={e => { if (!expanded) e.currentTarget.style.background = "transparent"; }}>
        <span style={{ color: colors.textDim, fontSize: 10, transform: expanded ? "rotate(90deg)" : "none", transition: "transform 0.1s" }}>▶</span>
        <div style={{ width: 28, height: 16, borderRadius: 8, background: rule.enabled ? colors.accent : colors.border, position: "relative", cursor: "pointer", flexShrink: 0 }}>
          <div style={{ width: 12, height: 12, borderRadius: 6, background: colors.textBright, position: "absolute", top: 2, left: rule.enabled ? 14 : 2, transition: "left 0.15s" }} />
        </div>
        <Badge color={colors.yellow} bg={colors.yellowDim}>S{rule.match.stream}F{rule.match.function}</Badge>
        <span style={{ color: rule.enabled ? colors.textBright : colors.textDim, fontWeight: 500, fontSize: 12 }}>{rule.name}</span>
        <span style={{ marginLeft: "auto", fontSize: 10, color: colors.textDim }}>
          {rule.conditions.length} cond · {rule.events.length} action{rule.events.length !== 1 ? "s" : ""}
        </span>
      </div>

      {/* Expanded detail */}
      {expanded && (
        <div style={{ padding: "0 10px 12px 36px", display: "flex", flexDirection: "column", gap: 12 }}>
          {/* Match Pattern */}
          <div>
            <div style={{ fontSize: 10, color: colors.textDim, fontWeight: 600, textTransform: "uppercase", marginBottom: 6, letterSpacing: 0.5 }}>When received</div>
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Field label="Stream" value={rule.match.stream} width={50} mono />
              <Field label="Function" value={rule.match.function} width={50} mono />
              <Field label="RCMD" value={rule.match.rcmd} width={100} mono />
            </div>
          </div>

          {/* Conditions */}
          <div>
            <div style={{ fontSize: 10, color: colors.textDim, fontWeight: 600, textTransform: "uppercase", marginBottom: 6, letterSpacing: 0.5, display: "flex", alignItems: "center", gap: 6 }}>
              If conditions met
              <button style={{ background: colors.accentDim, color: colors.accent, border: "none", borderRadius: 3, padding: "1px 6px", fontSize: 9, cursor: "pointer", fontFamily: "inherit" }}>+ Add</button>
            </div>
            {rule.conditions.length === 0 ? (
              <span style={{ fontSize: 10, color: colors.textDim, fontStyle: "italic" }}>Always matches (no conditions)</span>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                {rule.conditions.map((c, i) => (
                  <div key={i} style={{ display: "flex", alignItems: "center", gap: 6, background: colors.bg, borderRadius: 3, padding: "4px 8px" }}>
                    <span style={{ fontSize: 11, color: colors.teal, flex: 1 }}>{c.field}</span>
                    <span style={{ fontSize: 10, color: colors.textDim }}>=</span>
                    <span style={{ fontSize: 11, color: colors.textBright, flex: 1 }}>{c.value}</span>
                    <button style={{ background: "none", border: "none", color: colors.red, fontSize: 12, cursor: "pointer", padding: "0 2px" }}>×</button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Reply */}
          <div>
            <div style={{ fontSize: 10, color: colors.textDim, fontWeight: 600, textTransform: "uppercase", marginBottom: 6, letterSpacing: 0.5 }}>Then reply</div>
            <div style={{ display: "flex", gap: 8 }}>
              <Field label="Stream" value={rule.reply.stream} width={50} mono />
              <Field label="Function" value={rule.reply.function} width={50} mono />
              <Field label="ACK" value={rule.reply.ack} width={50} mono hint={rule.reply.ack === 0 ? "success" : "reject"} />
            </div>
          </div>

          {/* Actions timeline */}
          <div>
            <div style={{ fontSize: 10, color: colors.textDim, fontWeight: 600, textTransform: "uppercase", marginBottom: 6, letterSpacing: 0.5, display: "flex", alignItems: "center", gap: 6 }}>
              Then do (timeline)
              <button style={{ background: colors.accentDim, color: colors.accent, border: "none", borderRadius: 3, padding: "1px 6px", fontSize: 9, cursor: "pointer", fontFamily: "inherit" }}>+ Add</button>
            </div>
            {rule.events.length === 0 ? (
              <span style={{ fontSize: 10, color: colors.textDim, fontStyle: "italic" }}>No side effects</span>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: 4, position: "relative", paddingLeft: 12 }}>
                {/* Timeline line */}
                <div style={{ position: "absolute", left: 3, top: 8, bottom: 8, width: 1, background: colors.border }} />
                {rule.events.map((ev, i) => (
                  <div key={i} style={{ display: "flex", alignItems: "center", gap: 8, position: "relative" }}>
                    <div style={{ position: "absolute", left: -12, width: 7, height: 7, borderRadius: "50%", background: ev.type === "event" ? colors.yellow : colors.orange, border: `2px solid ${colors.bg}` }} />
                    <span style={{ fontSize: 10, color: colors.textDim, width: 50, textAlign: "right", flexShrink: 0 }}>+{ev.delay}ms</span>
                    {ev.type === "event" ? (
                      <ActionTag type="event">S6F11 CEID={ev.ceid}</ActionTag>
                    ) : (
                      <ActionTag type="mutate">{ev.target} → {ev.value}</ActionTag>
                    )}
                    <button style={{ background: "none", border: "none", color: colors.red, fontSize: 12, cursor: "pointer", padding: "0 2px", marginLeft: "auto" }}>×</button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Action buttons */}
          <div style={{ display: "flex", gap: 6, paddingTop: 4 }}>
            <button style={{ background: colors.accentDim, color: colors.accent, border: `1px solid ${colors.accent}33`, borderRadius: 3, padding: "3px 10px", fontSize: 10, cursor: "pointer", fontFamily: "inherit" }}>Duplicate</button>
            <button style={{ background: colors.redDim, color: colors.red, border: `1px solid ${colors.red}33`, borderRadius: 3, padding: "3px 10px", fontSize: 10, cursor: "pointer", fontFamily: "inherit" }}>Delete</button>
            <span style={{ flex: 1 }} />
            <button style={{ background: colors.accentDim, color: colors.accent, border: `1px solid ${colors.accent}33`, borderRadius: 3, padding: "3px 10px", fontSize: 10, cursor: "pointer", fontFamily: "inherit" }}>Export YAML</button>
          </div>
        </div>
      )}
    </div>
  );
}

function HsmsConfigPanel() {
  const cfg = HSMS_CONFIG;
  return (
    <div style={{ overflow: "auto", flex: 1 }}>
      <SectionHeader>Connection</SectionHeader>
      <div style={{ padding: "10px 12px", display: "flex", flexDirection: "column", gap: 10 }}>
        <div style={{ display: "flex", gap: 8 }}>
          <Select label="Mode" value={cfg.mode} options={["passive", "active"]} width={100} />
          <Field label="Bind Address" value={cfg.ip} width={120} mono />
          <Field label="Port" value={cfg.port} width={70} mono />
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <Field label="Session ID" value={cfg.sessionId} width={70} mono />
          <Field label="Device ID" value={cfg.deviceId} width={70} mono />
        </div>
      </div>

      <SectionHeader right={<span style={{ fontSize: 9 }}>seconds</span>}>Timers</SectionHeader>
      <div style={{ padding: "10px 12px" }}>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
          <Field label="T3 (Reply)" value={cfg.t3} width={65} mono hint="reply timeout" />
          <Field label="T5 (Connect)" value={cfg.t5} width={65} mono hint="connect sep." />
          <Field label="T6 (Control)" value={cfg.t6} width={65} mono hint="control txn" />
          <Field label="T7 (Not Selected)" value={cfg.t7} width={65} mono hint="not selected" />
          <Field label="T8 (Byte)" value={cfg.t8} width={65} mono hint="inter-byte" />
        </div>
      </div>

      <SectionHeader>Device Identity</SectionHeader>
      <div style={{ padding: "10px 12px", display: "flex", flexDirection: "column", gap: 10 }}>
        <Field label="Model Name (MDLN)" value="STOCKER-SIM" mono />
        <Field label="Software Rev (SOFTREV)" value="1.0.0" mono />
      </div>

      <SectionHeader>Handshake Behavior</SectionHeader>
      <div style={{ padding: "10px 12px", display: "flex", flexDirection: "column", gap: 8 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <div style={{ width: 28, height: 16, borderRadius: 8, background: colors.accent, position: "relative", cursor: "pointer" }}>
            <div style={{ width: 12, height: 12, borderRadius: 6, background: colors.textBright, position: "absolute", top: 2, left: 14 }} />
          </div>
          <span style={{ fontSize: 11, color: colors.text }}>Auto-respond to S1F13 (Establish Comm)</span>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <div style={{ width: 28, height: 16, borderRadius: 8, background: colors.accent, position: "relative", cursor: "pointer" }}>
            <div style={{ width: 12, height: 12, borderRadius: 6, background: colors.textBright, position: "absolute", top: 2, left: 14 }} />
          </div>
          <span style={{ fontSize: 11, color: colors.text }}>Auto-respond to S1F1 (Are You There)</span>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <div style={{ width: 28, height: 16, borderRadius: 8, background: colors.border, position: "relative", cursor: "pointer" }}>
            <div style={{ width: 12, height: 12, borderRadius: 6, background: colors.textBright, position: "absolute", top: 2, left: 2 }} />
          </div>
          <span style={{ fontSize: 11, color: colors.textDim }}>Auto-respond to S2F25 (Loopback)</span>
        </div>
      </div>
    </div>
  );
}

export default function SimulatorWireframe() {
  const [leftTab, setLeftTab] = useState("rules");
  const [selectedMsg, setSelectedMsg] = useState(null);
  const [simRunning, setSimRunning] = useState(true);
  const [expandedRule, setExpandedRule] = useState(1);
  const [rightBottomTab, setRightBottomTab] = useState("detail");

  const sel = selectedMsg !== null ? MOCK_MESSAGES.find(m => m.id === selectedMsg) : null;

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100vh", background: colors.bg, color: colors.text, fontFamily: "'JetBrains Mono', 'Fira Code', 'SF Mono', monospace", fontSize: 12, overflow: "hidden" }}>
      {/* Toolbar */}
      <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "5px 12px", background: colors.surface, borderBottom: `1px solid ${colors.border}`, flexShrink: 0 }}>
        <span style={{ fontWeight: 700, fontSize: 13, color: colors.accent, marginRight: 8 }}>◆ SECSIM</span>
        <div style={{ display: "flex", alignItems: "center", gap: 6, marginRight: "auto" }}>
          <span style={{ display: "inline-block", width: 8, height: 8, borderRadius: "50%", background: simRunning ? colors.green : colors.textDim }} />
          <span style={{ fontSize: 11, color: simRunning ? colors.green : colors.textDim }}>{simRunning ? "LISTENING" : "STOPPED"}</span>
          <span style={{ fontSize: 10, color: colors.textDim, marginLeft: 4 }}>{HSMS_CONFIG.mode} · :{HSMS_CONFIG.port}</span>
        </div>
        <button onClick={() => setSimRunning(!simRunning)} style={{
          background: simRunning ? colors.redDim : colors.greenDim, color: simRunning ? colors.red : colors.green,
          border: `1px solid ${simRunning ? colors.red + "44" : colors.green + "44"}`, borderRadius: 4,
          padding: "3px 12px", fontSize: 11, cursor: "pointer", fontFamily: "inherit", fontWeight: 500,
        }}>{simRunning ? "■ Stop" : "▶ Start"}</button>
        <button style={{ background: colors.surfaceAlt, color: colors.yellow, border: `1px solid ${colors.yellow}33`, borderRadius: 4, padding: "3px 12px", fontSize: 11, cursor: "pointer", fontFamily: "inherit" }}>↻ Reload</button>
      </div>

      {/* Main split */}
      <div style={{ display: "flex", flex: 1, overflow: "hidden" }}>
        {/* LEFT PANEL */}
        <div style={{ width: "44%", display: "flex", flexDirection: "column", borderRight: `1px solid ${colors.border}`, overflow: "hidden" }}>
          <div style={{ display: "flex", borderBottom: `1px solid ${colors.border}`, flexShrink: 0 }}>
            <Tab active={leftTab === "rules"} onClick={() => setLeftTab("rules")} icon="⚙">Rules</Tab>
            <Tab active={leftTab === "state"} onClick={() => setLeftTab("state")} icon="◉">State</Tab>
            <Tab active={leftTab === "hsms"} onClick={() => setLeftTab("hsms")} icon="⇌">HSMS</Tab>
          </div>

          {leftTab === "rules" && (
            <div style={{ flex: 1, overflow: "auto" }}>
              <SectionHeader right={
                <button style={{ background: colors.accentDim, color: colors.accent, border: "none", borderRadius: 3, padding: "2px 8px", fontSize: 10, cursor: "pointer", fontFamily: "inherit" }}>+ New Rule</button>
              }>{MOCK_RULES.length} rules</SectionHeader>
              {MOCK_RULES.map(r => (
                <RuleCard key={r.id} rule={r} expanded={expandedRule === r.id}
                  onToggle={() => setExpandedRule(expandedRule === r.id ? null : r.id)} />
              ))}
            </div>
          )}

          {leftTab === "state" && (
            <div style={{ flex: 1, overflow: "auto" }}>
              <SectionHeader>Device</SectionHeader>
              <div style={{ padding: "8px 12px", display: "flex", gap: 16, fontSize: 11 }}>
                <div><span style={{ color: colors.textDim }}>Device:</span> <span style={{ color: colors.textBright }}>{MOCK_STATE.device}</span></div>
                <div><span style={{ color: colors.textDim }}>Mode:</span> <Badge color={colors.green} bg={colors.greenDim}>{MOCK_STATE.mode}</Badge></div>
              </div>
              <SectionHeader>Ports</SectionHeader>
              <div style={{ padding: "6px 12px" }}>
                {Object.entries(MOCK_STATE.ports).map(([k, v]) => (
                  <div key={k} style={{ display: "flex", alignItems: "center", padding: "3px 0", fontSize: 11 }}>
                    <span style={{ width: 50, color: colors.textDim }}>{k}</span>
                    <span style={{ display: "inline-block", width: 8, height: 8, borderRadius: "50%", background: v === "occupied" ? colors.blue : v === "empty" ? colors.textDim : colors.red, marginRight: 6 }} />
                    <span style={{ color: v === "blocked" ? colors.red : colors.text }}>{v}</span>
                  </div>
                ))}
              </div>
              <SectionHeader>Carriers</SectionHeader>
              <div style={{ padding: "6px 12px" }}>
                {Object.entries(MOCK_STATE.carriers).map(([k, v]) => (
                  <div key={k} style={{ display: "flex", alignItems: "center", padding: "3px 0", fontSize: 11 }}>
                    <span style={{ width: 70, color: colors.teal }}>{k}</span>
                    <span style={{ color: colors.textDim }}>→</span>
                    <span style={{ marginLeft: 6, color: colors.textBright }}>{v.location}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {leftTab === "hsms" && <HsmsConfigPanel />}
        </div>

        {/* RIGHT PANEL — Wireshark-style */}
        <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
          <div style={{ flex: 1, overflow: "auto", minHeight: 0 }}>
            <SectionHeader right={<span style={{ fontSize: 10 }}>{MOCK_MESSAGES.length} messages</span>}>Message Log</SectionHeader>
            <div style={{ display: "flex", padding: "4px 10px", background: colors.surfaceAlt, borderBottom: `1px solid ${colors.border}`, fontSize: 10, color: colors.textDim, fontWeight: 600, position: "sticky", top: 0, zIndex: 1 }}>
              <span style={{ width: 90 }}>Time</span>
              <span style={{ width: 36 }}>Dir</span>
              <span style={{ width: 55 }}>SxFy</span>
              <span style={{ flex: 1 }}>Info</span>
              <span style={{ width: 110 }}>Matched Rule</span>
            </div>
            {MOCK_MESSAGES.map(m => {
              const isSel = selectedMsg === m.id;
              const isIn = m.dir === "IN";
              return (
                <div key={m.id} onClick={() => setSelectedMsg(isSel ? null : m.id)} style={{
                  display: "flex", padding: "4px 10px", fontSize: 11, cursor: "pointer",
                  background: isSel ? colors.accent + "22" : "transparent",
                  borderLeft: isSel ? `2px solid ${colors.accent}` : "2px solid transparent",
                }} onMouseEnter={e => { if (!isSel) e.currentTarget.style.background = colors.surfaceAlt; }}
                   onMouseLeave={e => { if (!isSel) e.currentTarget.style.background = "transparent"; }}>
                  <span style={{ width: 90, color: colors.textDim }}>{m.ts}</span>
                  <span style={{ width: 36 }}><Badge color={isIn ? colors.green : colors.blue} bg={isIn ? colors.greenDim : colors.accentDim}>{isIn ? "IN" : "OUT"}</Badge></span>
                  <span style={{ width: 55, color: colors.yellow, fontWeight: 600 }}>{m.sf}</span>
                  <span style={{ flex: 1, color: colors.text }}>{m.label}</span>
                  <span style={{ width: 110, color: m.matched ? colors.accent : colors.textDim, fontSize: 10 }}>{m.matched || "—"}</span>
                </div>
              );
            })}
          </div>

          {/* Detail pane */}
          <div style={{ height: sel ? 180 : 0, borderTop: sel ? `1px solid ${colors.border}` : "none", overflow: "hidden", transition: "height 0.15s", flexShrink: 0 }}>
            {sel && (
              <div style={{ height: "100%", display: "flex", flexDirection: "column" }}>
                <div style={{ display: "flex", borderBottom: `1px solid ${colors.border}`, flexShrink: 0 }}>
                  <Tab active={rightBottomTab === "detail"} onClick={() => setRightBottomTab("detail")}>Decoded</Tab>
                  <Tab active={rightBottomTab === "raw"} onClick={() => setRightBottomTab("raw")}>Raw SML</Tab>
                  <Tab active={rightBottomTab === "rule"} onClick={() => setRightBottomTab("rule")}>Matched Rule</Tab>
                </div>
                <div style={{ flex: 1, overflow: "auto", padding: "8px 12px", fontSize: 11 }}>
                  {rightBottomTab === "detail" && (
                    <div>
                      <div style={{ display: "flex", gap: 16, marginBottom: 8 }}>
                        <span><span style={{ color: colors.textDim }}>Stream:</span> <span style={{ color: colors.yellow }}>{sel.detail.stream}</span></span>
                        <span><span style={{ color: colors.textDim }}>Function:</span> <span style={{ color: colors.yellow }}>{sel.detail.function}</span></span>
                        <span><span style={{ color: colors.textDim }}>W-bit:</span> <span style={{ color: sel.detail.wbit ? colors.green : colors.textDim }}>{sel.detail.wbit ? "yes" : "no"}</span></span>
                        <span><span style={{ color: colors.textDim }}>Direction:</span> <Badge color={sel.dir === "IN" ? colors.green : colors.blue} bg={sel.dir === "IN" ? colors.greenDim : colors.accentDim}>{sel.dir}</Badge></span>
                      </div>
                      <div style={{ color: colors.textDim, marginBottom: 4, fontSize: 10, textTransform: "uppercase" }}>Body (SML tree)</div>
                      <pre style={{ margin: 0, color: colors.teal, lineHeight: 1.6 }}>{sel.detail.body}</pre>
                    </div>
                  )}
                  {rightBottomTab === "raw" && <pre style={{ margin: 0, color: colors.textDim, lineHeight: 1.6 }}>{`${sel.sf}\n  W=${sel.detail.wbit ? "1" : "0"}\n  ${sel.detail.body}`}</pre>}
                  {rightBottomTab === "rule" && (
                    <div>{sel.matched ? (<><div style={{ color: colors.accent, fontWeight: 600, marginBottom: 6 }}>Rule: {sel.matched}</div><div style={{ color: colors.textDim }}>This message {sel.dir === "IN" ? "triggered" : "was generated by"} the "{sel.matched}" rule.</div></>) : (<span style={{ color: colors.textDim }}>No rule matched — session/handshake message</span>)}</div>
                  )}
                </div>
              </div>
            )}
          </div>

          {/* Status bar */}
          <div style={{ display: "flex", gap: 16, padding: "4px 12px", background: colors.surface, borderTop: `1px solid ${colors.border}`, fontSize: 10, color: colors.textDim, flexShrink: 0 }}>
            <span>HSMS: <span style={{ color: colors.green }}>SELECTED</span></span>
            <span>Session ID: {HSMS_CONFIG.sessionId}</span>
            <span>{HSMS_CONFIG.mode} · {HSMS_CONFIG.ip}:{HSMS_CONFIG.port}</span>
            <span>Messages: {MOCK_MESSAGES.length}</span>
            <span style={{ marginLeft: "auto" }}>Rules: {MOCK_RULES.length}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

