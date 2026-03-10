import type { RuntimeState, HsmsConfig } from "../types";

interface ToolbarProps {
  runtime: RuntimeState;
  hsms: HsmsConfig;
  onToggleRuntime: () => void;
  onReload: () => void;
  onSave: () => void;
}

export function Toolbar({ runtime, hsms, onToggleRuntime, onReload, onSave }: ToolbarProps) {
  const statusLabel = runtime.listening ? "LISTENING" : "STOPPED";

  return (
    <header className="toolbar">
      <div className="toolbar-brand">◆ SECSIM</div>
      <div className="toolbar-status">
        <span className={`status-dot ${runtime.listening ? "green" : "neutral"}`} />
        <span className={`status-text ${runtime.listening ? "green" : ""}`}>{statusLabel}</span>
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

