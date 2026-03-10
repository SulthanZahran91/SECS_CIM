import { Badge, SectionHeader } from "./ui";
import type { DeviceConfig, StateSnapshot } from "../types";

interface StateTabProps {
  device: DeviceConfig;
  state: StateSnapshot;
}

export function StateTab({ device, state }: StateTabProps) {
  return (
    <div className="panel-scroll">
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
        {Object.entries(state.ports).map(([name, status]) => (
          <div className="state-row" key={name}>
            <span className="port-name">{name}</span>
            <span className={`status-dot ${status}`} />
            <span className={status === "blocked" ? "text-red" : "bright-text"}>{status}</span>
          </div>
        ))}
      </div>

      <SectionHeader>Carriers</SectionHeader>
      <div className="section-body stack-list">
        {Object.entries(state.carriers).map(([carrierId, carrier]) => (
          <div className="state-row" key={carrierId}>
            <span className="carrier-name">{carrierId}</span>
            <span className="subtle-text">→</span>
            <span className="bright-text">{carrier.location}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

