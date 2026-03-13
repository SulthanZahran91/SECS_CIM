import { Badge, CollapsibleSection } from "./ui";
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
        <CollapsibleSection
          title="Device"
          right={
            <Badge tone={state.mode === "online-remote" ? "green" : state.mode === "online-local" ? "yellow" : "red"}>
              {state.mode}
            </Badge>
          }
        >
          <div className="inline-summary">
            <span className="bright-text">{device.name}</span>
            <span className="subtle-text">{device.protocol.toUpperCase()}</span>
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Ports" right={<span className="subtle-text">{occupiedPorts} occupied{blockedPorts > 0 ? `, ${blockedPorts} blocked` : ""}</span>}>
          <div className="stack-list">
            {ports.map(([name, status]) => (
              <div className="state-row" key={name}>
                <span className="port-name">{name}</span>
                <span className={`status-dot ${status}`} />
                <span className={status === "blocked" ? "text-red" : "bright-text"}>{status}</span>
              </div>
            ))}
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Carriers" right={<span className="subtle-text">{carriers.length} tracked</span>}>
          <div className="stack-list">
            {carriers.length === 0 ? (
              <div className="empty-copy">No carriers tracked.</div>
            ) : (
              carriers.map(([carrierId, carrier]) => (
                <div className="state-row" key={carrierId}>
                  <span className="carrier-name">{carrierId}</span>
                  <span className="subtle-text">→</span>
                  <span className="bright-text">{carrier.location}</span>
                </div>
              ))
            )}
          </div>
        </CollapsibleSection>
      </div>
    </div>
  );
}
