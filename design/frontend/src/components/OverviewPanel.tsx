import { Badge } from "./ui";
import type { Snapshot } from "../types";

interface OverviewPanelProps {
  snapshot: Snapshot;
}

type BadgeTone = "green" | "blue" | "yellow" | "accent" | "red" | "teal" | "neutral";

interface FocusState {
  tone: BadgeTone;
  badge: string;
  title: string;
  copy: string;
}

export function OverviewPanel({ snapshot }: OverviewPanelProps) {
  const enabledRules = snapshot.rules.filter((rule) => rule.enabled).length;
  const matchedMessages = snapshot.messages.filter((message) => Boolean(message.matchedRule)).length;
  const lastMessage = snapshot.messages[snapshot.messages.length - 1];
  const portStates = Object.values(snapshot.state.ports);
  const occupiedPorts = portStates.filter((status) => status === "occupied").length;
  const blockedPorts = portStates.filter((status) => status === "blocked").length;
  const emptyPorts = portStates.filter((status) => status === "empty").length;
  const focusState = summarizeFocus(snapshot);

  return (
    <section className="workspace-overview">
      <div className="overview-strip">
        <article className="overview-card">
          <span className="overview-label">Runtime health</span>
          <div className="overview-value-row">
            <strong className={`overview-value ${runtimeValueClass(snapshot)}`}>
              {snapshot.runtime.listening ? snapshot.runtime.hsmsState : "STOPPED"}
            </strong>
            <Badge tone={snapshot.runtime.lastError ? "red" : snapshot.runtime.listening ? "green" : "neutral"}>
              {snapshot.runtime.lastError ? "Issue detected" : snapshot.runtime.listening ? "Live" : "Idle"}
            </Badge>
          </div>
          <p className="overview-copy">
            {snapshot.hsms.mode} endpoint {snapshot.hsms.ip}:{snapshot.hsms.port}
          </p>
          <div className="overview-metrics">
            <div>
              <span className="overview-metric-label">Session</span>
              <span className="overview-metric-value mono">{snapshot.hsms.sessionId}</span>
            </div>
            <div>
              <span className="overview-metric-label">Device ID</span>
              <span className="overview-metric-value mono">{snapshot.hsms.deviceId}</span>
            </div>
          </div>
        </article>

        <article className="overview-card">
          <span className="overview-label">Config readiness</span>
          <div className="overview-value-row">
            <strong className={`overview-value ${snapshot.runtime.dirty ? "value-warn" : "value-good"}`}>
              {snapshot.runtime.dirty ? "Unsaved edits" : "Synced to disk"}
            </strong>
            <Badge tone={snapshot.runtime.restartRequired ? "yellow" : "teal"}>
              {snapshot.runtime.restartRequired ? "Restart pending" : "Hot changes applied"}
            </Badge>
          </div>
          <p className="overview-copy">Baseline file: {snapshot.runtime.configFile}</p>
          <div className="overview-metrics">
            <div>
              <span className="overview-metric-label">Rules ready</span>
              <span className="overview-metric-value">
                {enabledRules}/{snapshot.rules.length}
              </span>
            </div>
            <div>
              <span className="overview-metric-label">Reload risk</span>
              <span className="overview-metric-value">
                {snapshot.runtime.dirty ? "Would discard edits" : "Clean"}
              </span>
            </div>
          </div>
        </article>

        <article className="overview-card">
          <span className="overview-label">Simulator posture</span>
          <div className="overview-value-row">
            <strong className="overview-value">{snapshot.device.name}</strong>
            <Badge
              tone={
                snapshot.state.mode === "online-remote"
                  ? "green"
                  : snapshot.state.mode === "online-local"
                    ? "yellow"
                    : "red"
              }
            >
              {snapshot.state.mode}
            </Badge>
          </div>
          <p className="overview-copy">
            {snapshot.device.protocol.toUpperCase()} simulator, {snapshot.device.mdln} / {snapshot.device.softrev}
          </p>
          <div className="overview-metrics">
            <div>
              <span className="overview-metric-label">Occupied</span>
              <span className="overview-metric-value">{occupiedPorts}</span>
            </div>
            <div>
              <span className="overview-metric-label">Empty</span>
              <span className="overview-metric-value">{emptyPorts}</span>
            </div>
            <div>
              <span className="overview-metric-label">Blocked</span>
              <span className="overview-metric-value">{blockedPorts}</span>
            </div>
          </div>
        </article>

        <article className="overview-card">
          <span className="overview-label">Recent activity</span>
          <div className="overview-value-row">
            <strong className="overview-value">{snapshot.messages.length} messages</strong>
            <Badge tone={matchedMessages > 0 ? "accent" : "neutral"}>
              {matchedMessages} rule-linked
            </Badge>
          </div>
          <p className="overview-copy">
            {lastMessage
              ? `Latest ${lastMessage.direction} ${lastMessage.sf} at ${lastMessage.timestamp}`
              : "No traffic captured yet."}
          </p>
          <div className="overview-metrics">
            <div>
              <span className="overview-metric-label">Tracked carriers</span>
              <span className="overview-metric-value">{Object.keys(snapshot.state.carriers).length}</span>
            </div>
            <div>
              <span className="overview-metric-label">Last label</span>
              <span className="overview-metric-value mono">{lastMessage?.label ?? "waiting"}</span>
            </div>
          </div>
        </article>
      </div>

      <div className={`focus-banner tone-${focusState.tone}`}>
        <div className="focus-copy-block">
          <span className="focus-label">Suggested next step</span>
          <div className="focus-title-row">
            <h2 className="focus-title">{focusState.title}</h2>
            <Badge tone={focusState.tone}>{focusState.badge}</Badge>
          </div>
          <p className="focus-text">{focusState.copy}</p>
        </div>
        <div className="shortcut-cluster" aria-label="Keyboard shortcuts">
          <span className="shortcut-chip">Ctrl/Cmd+S Save</span>
          <span className="shortcut-chip">Ctrl/Cmd+R Reload</span>
          <span className="shortcut-chip">Ctrl/Cmd+1-3 Switch tabs</span>
          <span className="shortcut-chip">Ctrl/Cmd+L Clear log</span>
        </div>
      </div>
    </section>
  );
}

function summarizeFocus(snapshot: Snapshot): FocusState {
  if (snapshot.runtime.lastError) {
    return {
      tone: "red",
      badge: "Transport issue",
      title: "Resolve the HSMS connection failure",
      copy: snapshot.runtime.lastError,
    };
  }

  if (snapshot.runtime.restartRequired) {
    return {
      tone: "yellow",
      badge: "Restart required",
      title: "Restart the runtime to apply connection changes",
      copy: "Mode, address, or port changed while the simulator was active.",
    };
  }

  if (snapshot.runtime.dirty) {
    return {
      tone: "yellow",
      badge: "Unsaved work",
      title: "Commit or discard the current config edits",
      copy: "Save to keep the current rule and connection setup, or reload to return to the last file-backed baseline.",
    };
  }

  if (!snapshot.runtime.listening) {
    return {
      tone: "accent",
      badge: "Runtime stopped",
      title: "Start the simulator before validating host flows",
      copy: "The UI is ready, but no HSMS transport is running yet.",
    };
  }

  if (snapshot.messages.length === 0) {
    return {
      tone: "blue",
      badge: "Waiting for traffic",
      title: "Connect a host or probe to begin exercising rules",
      copy: "Once traffic arrives, the live log and detail pane will fill in automatically.",
    };
  }

  return {
    tone: "green",
    badge: "Active session",
    title: "Use the filtered log to inspect the latest protocol flow",
    copy: "The runtime is live. Focus on the most recent rule-linked messages or inspect raw payloads from the detail pane.",
  };
}

function runtimeValueClass(snapshot: Snapshot): string {
  if (snapshot.runtime.lastError) {
    return "value-danger";
  }

  if (snapshot.runtime.listening && snapshot.runtime.hsmsState === "SELECTED") {
    return "value-good";
  }

  if (snapshot.runtime.listening) {
    return "value-accent";
  }

  return "";
}
