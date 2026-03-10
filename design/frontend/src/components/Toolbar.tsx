import type { RuntimeState, HsmsConfig } from "../types";
import { Badge } from "./ui";

interface ToolbarProps {
  runtime: RuntimeState;
  hsms: HsmsConfig;
  onToggleRuntime: () => void;
  onReload: () => void;
  onSave: () => void;
}

export function Toolbar({ runtime, hsms, onToggleRuntime, onReload, onSave }: ToolbarProps) {
  const statusLabel = runtime.listening ? runtime.hsmsState : "STOPPED";
  const statusTone = runtime.lastError ? "blocked" : runtime.hsmsState === "SELECTED" ? "green" : runtime.listening ? "occupied" : "neutral";
  const statusTextClass = runtime.lastError ? "text-red" : runtime.hsmsState === "SELECTED" ? "green" : "";

  return (
    <header className="toolbar">
      <div className="toolbar-brand">◆ SECSIM</div>
      <div className="toolbar-status">
        <span className={`status-dot ${statusTone}`} />
        <span className={`status-text ${statusTextClass}`}>{statusLabel}</span>
        {runtime.lastError ? <Badge tone="red">Transport issue</Badge> : null}
        <span className="toolbar-summary">
          {hsms.mode} · {hsms.ip}:{hsms.port}
        </span>
      </div>
      <div className="toolbar-actions">
        <button className="toolbar-button neutral" onClick={onSave} type="button">
          Save
        </button>
        <button className="toolbar-button warning" onClick={onReload} type="button">
          Reload
        </button>
        <button
          className={`toolbar-button ${runtime.listening ? "danger" : "success"}`}
          onClick={onToggleRuntime}
          type="button"
        >
          {runtime.listening ? "■ Stop" : "▶ Start"}
        </button>
      </div>
    </header>
  );
}
