import { startTransition, useEffect, useState } from "react";
import { Toolbar } from "./components/Toolbar";
import { RulesTab } from "./components/RulesTab";
import { StateTab } from "./components/StateTab";
import { HsmsTab } from "./components/HsmsTab";
import { MessageMonitor } from "./components/MessageMonitor";
import { TabButton } from "./components/ui";
import { api } from "./lib/api";
import { normalizeSnapshot } from "./lib/normalizeSnapshot";
import { ruleToYaml } from "./lib/ruleToYaml";
import type { DetailTab, DeviceConfig, HsmsConfig, LeftTab, Rule, Snapshot } from "./types";

function markDirty(snapshot: Snapshot): Snapshot {
  return {
    ...snapshot,
    runtime: {
      ...snapshot.runtime,
      dirty: true,
    },
  };
}

export default function App() {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [leftTab, setLeftTab] = useState<LeftTab>("rules");
  const [expandedRuleId, setExpandedRuleId] = useState<string | null>(null);
  const [selectedMessageId, setSelectedMessageId] = useState<string | null>(null);
  const [detailTab, setDetailTab] = useState<DetailTab>("decoded");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => {
    void load();
  }, []);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (!snapshot) {
        return;
      }

      const shortcut = event.ctrlKey || event.metaKey;
      if (shortcut && event.key.toLowerCase() === "s") {
        event.preventDefault();
        void run(api.saveConfig, "Config saved");
        return;
      }
      if (shortcut && event.key.toLowerCase() === "r") {
        event.preventDefault();
        void run(api.reloadConfig, "Baseline restored");
        return;
      }
      if (shortcut && event.key.toLowerCase() === "n") {
        event.preventDefault();
        setLeftTab("rules");
        void run(api.createRule, "Rule created");
        return;
      }
      if (shortcut && event.key.toLowerCase() === "l") {
        event.preventDefault();
        void run(api.clearLog, "Message log cleared");
        return;
      }
      if (shortcut && event.key === "1") {
        event.preventDefault();
        setLeftTab("rules");
        return;
      }
      if (shortcut && event.key === "2") {
        event.preventDefault();
        setLeftTab("state");
        return;
      }
      if (shortcut && event.key === "3") {
        event.preventDefault();
        setLeftTab("hsms");
        return;
      }
      if (event.key === "Escape") {
        setSelectedMessageId(null);
      }
    }

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [snapshot]);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const nextSnapshot = await api.bootstrap();
      replaceSnapshot(nextSnapshot);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "Failed to load simulator state");
    } finally {
      setLoading(false);
    }
  }

  function replaceSnapshot(nextSnapshot: Snapshot) {
    const normalizedSnapshot = normalizeSnapshot(nextSnapshot);
    startTransition(() => {
      setSnapshot(normalizedSnapshot);
      setExpandedRuleId((current) => {
        if (current && normalizedSnapshot.rules.some((rule) => rule.id === current)) {
          return current;
        }
        return normalizedSnapshot.rules[0]?.id ?? null;
      });
      setSelectedMessageId((current) => {
        if (current && normalizedSnapshot.messages.some((message) => message.id === current)) {
          return current;
        }
        return null;
      });
    });
  }

  async function run(action: () => Promise<Snapshot>, nextNotice?: string) {
    setError(null);
    try {
      const nextSnapshot = await action();
      replaceSnapshot(nextSnapshot);
      if (nextNotice) {
        setNotice(nextNotice);
      }
    } catch (actionError) {
      setError(actionError instanceof Error ? actionError.message : "Request failed");
    }
  }

  function applyOptimistic(update: (current: Snapshot) => Snapshot, action: () => Promise<Snapshot>) {
    setError(null);
    setNotice(null);
    startTransition(() => {
      setSnapshot((current) => (current ? update(current) : current));
    });
    void action()
      .then((nextSnapshot) => {
        replaceSnapshot(nextSnapshot);
      })
      .catch((actionError) => {
        setError(actionError instanceof Error ? actionError.message : "Request failed");
        void load();
      });
  }

  function handleRuleChange(rule: Rule) {
    applyOptimistic(
      (current) =>
        markDirty({
          ...current,
          rules: current.rules.map((currentRule) => (currentRule.id === rule.id ? rule : currentRule)),
        }),
      () => api.updateRule(rule),
    );
  }

  function handleHsmsChange(config: HsmsConfig) {
    applyOptimistic(
      (current) => markDirty({ ...current, hsms: config }),
      () => api.updateHsms(config),
    );
  }

  function handleDeviceChange(device: DeviceConfig) {
    applyOptimistic(
      (current) => markDirty({ ...current, device }),
      () => api.updateDevice(device),
    );
  }

  async function handleExportRule(rule: Rule) {
    try {
      await navigator.clipboard.writeText(ruleToYaml(rule));
      setNotice(`Copied ${rule.name} as YAML`);
      setError(null);
    } catch {
      setError("Clipboard export failed");
    }
  }

  if (loading) {
    return <div className="loading-screen">Loading SECSIM scaffold…</div>;
  }

  if (!snapshot) {
    return (
      <div className="loading-screen" role={error ? "alert" : undefined}>
        {error ?? "No simulator state available."}
      </div>
    );
  }

  return (
    <div className="app-shell">
      <Toolbar
        runtime={snapshot.runtime}
        hsms={snapshot.hsms}
        onToggleRuntime={() => void run(api.toggleRuntime)}
        onReload={() => void run(api.reloadConfig, "Baseline restored")}
        onSave={() => void run(api.saveConfig, "Config saved")}
      />

      {error ? <div className="banner error">{error}</div> : null}
      {!error && notice ? <div className="banner notice">{notice}</div> : null}

      <div className="main-split">
        <section className="left-panel">
          <div className="tab-row">
            <TabButton active={leftTab === "rules"} icon="⚙" onClick={() => setLeftTab("rules")}>
              Rules
            </TabButton>
            <TabButton active={leftTab === "state"} icon="◉" onClick={() => setLeftTab("state")}>
              State
            </TabButton>
            <TabButton active={leftTab === "hsms"} icon="⇌" onClick={() => setLeftTab("hsms")}>
              HSMS
            </TabButton>
          </div>

          {leftTab === "rules" ? (
            <RulesTab
              rules={snapshot.rules}
              expandedRuleId={expandedRuleId}
              onToggleRule={(id) => setExpandedRuleId((current) => (current === id ? null : id))}
              onCreateRule={() => void run(api.createRule, "Rule created")}
              onChangeRule={handleRuleChange}
              onDuplicateRule={(id) => void run(() => api.duplicateRule(id), "Rule duplicated")}
              onDeleteRule={(id) => {
                if (window.confirm("Delete this rule?")) {
                  void run(() => api.deleteRule(id), "Rule deleted");
                }
              }}
              onMoveRule={(id, direction) => void run(() => api.moveRule(id, direction))}
              onExportRule={(rule) => void handleExportRule(rule)}
            />
          ) : null}

          {leftTab === "state" ? <StateTab device={snapshot.device} state={snapshot.state} /> : null}

          {leftTab === "hsms" ? (
            <HsmsTab
              hsms={snapshot.hsms}
              device={snapshot.device}
              dirty={snapshot.runtime.dirty}
              onChangeHsms={handleHsmsChange}
              onChangeDevice={handleDeviceChange}
            />
          ) : null}
        </section>

        <section className="right-panel">
          <MessageMonitor
            messages={snapshot.messages}
            selectedMessageId={selectedMessageId}
            detailTab={detailTab}
            onSelectMessage={setSelectedMessageId}
            onChangeDetailTab={setDetailTab}
            onJumpToRule={(ruleId) => {
              if (!ruleId) {
                return;
              }
              setLeftTab("rules");
              setExpandedRuleId(ruleId);
            }}
            onClearLog={() => void run(api.clearLog, "Message log cleared")}
          />

          <footer className="status-bar">
            <span>
              HSMS: <span className={snapshot.runtime.hsmsState === "SELECTED" ? "text-green" : ""}>{snapshot.runtime.hsmsState}</span>
            </span>
            <span>Session ID: {snapshot.hsms.sessionId}</span>
            <span>
              {snapshot.hsms.mode} · {snapshot.hsms.ip}:{snapshot.hsms.port}
            </span>
            <span>Messages: {snapshot.messages.length}</span>
            <span className="status-spacer" />
            <span>Rules: {snapshot.rules.length}</span>
            <span>{snapshot.runtime.configFile}</span>
            <span className={`dirty-dot ${snapshot.runtime.dirty ? "visible" : ""}`}>●</span>
          </footer>
        </section>
      </div>
    </div>
  );
}
