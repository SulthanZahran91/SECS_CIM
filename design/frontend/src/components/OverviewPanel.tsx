import { useState } from "react";
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
  const [expanded, setExpanded] = useState(false);
  const enabledRules = snapshot.rules.filter((rule) => rule.enabled).length;
  const matchedMessages = snapshot.messages.filter((message) => Boolean(message.matchedRule)).length;
  const lastMessage = snapshot.messages[snapshot.messages.length - 1];
  const portStates = Object.values(snapshot.state.ports);
  const occupiedPorts = portStates.filter((status) => status === "occupied").length;
  const focusState = summarizeFocus(snapshot);

  return (
    <section className="workspace-overview">
      <div className={`focus-banner tone-${focusState.tone}`}>
        <div className="focus-copy-block">
          <div className="focus-title-row">
            <Badge tone={focusState.tone}>{focusState.badge}</Badge>
            <h2 className="focus-title">{focusState.title}</h2>
          </div>
        </div>
        <button className="text-button" onClick={() => setExpanded(!expanded)} type="button" style={{ fontSize: 11 }}>
          {expanded ? "Less" : "More"}
        </button>
      </div>

      {expanded ? (
        <div className="overview-strip">
          <article className="overview-card">
            <span className="overview-label">Runtime</span>
            <div className="overview-value-row">
              <strong className={`overview-value ${runtimeValueClass(snapshot)}`}>
                {snapshot.runtime.listening ? snapshot.runtime.hsmsState : "STOPPED"}
              </strong>
              <Badge tone={snapshot.runtime.lastError ? "red" : snapshot.runtime.listening ? "green" : "neutral"}>
                {snapshot.runtime.lastError ? "Issue" : snapshot.runtime.listening ? "Live" : "Idle"}
              </Badge>
            </div>
            <p className="overview-copy">{snapshot.hsms.mode} {snapshot.hsms.ip}:{snapshot.hsms.port}</p>
          </article>

          <article className="overview-card">
            <span className="overview-label">Config</span>
            <div className="overview-value-row">
              <strong className={`overview-value ${snapshot.runtime.dirty ? "value-warn" : "value-good"}`}>
                {snapshot.runtime.dirty ? "Unsaved" : "Synced"}
              </strong>
              <Badge tone={snapshot.runtime.restartRequired ? "yellow" : "teal"}>
                {enabledRules}/{snapshot.rules.length} rules
              </Badge>
            </div>
          </article>

          <article className="overview-card">
            <span className="overview-label">Device</span>
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
            <p className="overview-copy">{occupiedPorts} occupied ports</p>
          </article>

          <article className="overview-card">
            <span className="overview-label">Traffic</span>
            <div className="overview-value-row">
              <strong className="overview-value">{snapshot.messages.length} msgs</strong>
              <Badge tone={matchedMessages > 0 ? "accent" : "neutral"}>
                {matchedMessages} matched
              </Badge>
            </div>
            <p className="overview-copy">
              {lastMessage ? `${lastMessage.direction} ${lastMessage.sf}` : "No traffic yet"}
            </p>
          </article>
        </div>
      ) : null}
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
