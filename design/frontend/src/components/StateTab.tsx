import { Badge, SectionHeader } from "./ui";
import type { DeviceConfig, StateSnapshot } from "../types";

interface StateTabProps {
  device: DeviceConfig;
  state: StateSnapshot;
}

export function StateTab({ device, state }: StateTabProps) {
  const ports = Object.entries(state.ports);
  const carriers = Object.entries(state.carriers);
  const occupiedPorts = ports.filter(([, status]) => status === "occupied").length;
  const blockedPorts = ports.filter(([, status]) => status === "blocked").length;

  return (
    <div className="panel-scroll">
      <div className="panel-scroll-content">
        <SectionHeader>State Overview</SectionHeader>
        <div className="section-body">
          <div className="state-overview-grid">
            <div className="state-metric-card">
              <span className="overview-label">Device</span>
              <strong className="overview-value">{device.name}</strong>
              <span className="overview-copy">Protocol {device.protocol.toUpperCase()}</span>
            </div>
            <div className="state-metric-card">
              <span className="overview-label">Current mode</span>
              <div className="overview-value-row">
                <strong className="overview-value">{state.mode}</strong>
                <Badge tone={state.mode === "online-remote" ? "green" : state.mode === "online-local" ? "yellow" : "red"}>
                  {state.mode}
                </Badge>
              </div>
            </div>
            <div className="state-metric-card">
              <span className="overview-label">Ports in use</span>
              <strong className="overview-value">{occupiedPorts}</strong>
              <span className="overview-copy">{blockedPorts} blocked paths require intervention</span>
            </div>
            <div className="state-metric-card">
              <span className="overview-label">Tracked carriers</span>
              <strong className="overview-value">{carriers.length}</strong>
              <span className="overview-copy">Live runtime state only, not persisted as edits</span>
            </div>
          </div>
        </div>

        <SectionHeader>Device</SectionHeader>
        <div className="section-body inline-summary">
          <div>
            <span className="subtle-text">Device:</span> <span className="bright-text">{device.name}</span>
          </div>
          <div>
            <span className="subtle-text">Mode:</span>{" "}
            <Badge tone={state.mode === "online-remote" ? "green" : state.mode === "online-local" ? "yellow" : "red"}>
              {state.mode}
            </Badge>
          </div>
        </div>

        <SectionHeader>Ports</SectionHeader>
        <div className="section-body stack-list">
          {ports.map(([name, status]) => (
            <div className="state-row" key={name}>
              <span className="port-name">{name}</span>
              <span className={`status-dot ${status}`} />
              <span className={status === "blocked" ? "text-red" : "bright-text"}>{status}</span>
            </div>
          ))}
        </div>

        <SectionHeader>Carriers</SectionHeader>
        <div className="section-body stack-list">
          {carriers.map(([carrierId, carrier]) => (
            <div className="state-row" key={carrierId}>
              <span className="carrier-name">{carrierId}</span>
              <span className="subtle-text">→</span>
              <span className="bright-text">{carrier.location}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
