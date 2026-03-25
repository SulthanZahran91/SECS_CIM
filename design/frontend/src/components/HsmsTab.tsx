import { Badge, CollapsibleSection, LabeledInput, LabeledSelect, TogglePill } from "./ui";
import type { DeviceConfig, HsmsConfig } from "../types";

const hostStartupProfileOptions = [
  { value: "disabled", label: "Disabled" },
  { value: "stocker", label: "Stocker / Minimal" },
  { value: "conveyor", label: "Conveyor / Captured Trace" },
];

interface HsmsTabProps {
  hsms: HsmsConfig;
  device: DeviceConfig;
  restartRequired: boolean;
  onChangeHsms: (config: HsmsConfig) => void;
  onChangeDevice: (device: DeviceConfig) => void;
}

function toNumber(value: string): number {
  const parsed = Number.parseInt(value, 10);
  return Number.isNaN(parsed) ? 0 : parsed;
}

function describeHostStartupProfile(profile: string): string {
  switch (profile) {
    case "stocker":
      return "Current minimal bring-up: S1F13, S1F17, S2F31, plus S6F11 acknowledgements.";
    case "conveyor":
      return "Conveyor startup based on the captured working host/equipment trace: S1F17 bring-up, pause/event acknowledgement, and early status polling.";
    default:
      return "No automated host bring-up runs after HSMS select.";
  }
}

function isWildcardBindAddress(value: string): boolean {
  const normalized = value.trim();
  return normalized === "0.0.0.0" || normalized === "::" || normalized === "[::]";
}

export function HsmsTab({ hsms, device, restartRequired, onChangeHsms, onChangeDevice }: HsmsTabProps) {
  const errors = {
    mode: hsms.mode === "passive" || hsms.mode === "active" ? "" : "Choose passive or active.",
    ip: !hsms.ip.trim()
      ? "Address is required."
      : hsms.mode === "active" && isWildcardBindAddress(hsms.ip)
        ? "Active mode must target a concrete host address. Use 127.0.0.1 for a local passive equipment."
        : "",
    port: validateRange(hsms.port, 1, 65535, "Use a TCP port between 1 and 65535."),
    sessionId: validateRange(hsms.sessionId, 0, 65535, "Use a 16-bit session ID."),
    deviceId: validateRange(hsms.deviceId, 0, 65535, "Use a 16-bit device ID."),
    t3: validatePositive(hsms.timers.t3, "T3 must be greater than zero."),
    t5: validatePositive(hsms.timers.t5, "T5 must be greater than zero."),
    t6: validatePositive(hsms.timers.t6, "T6 must be greater than zero."),
    t7: validatePositive(hsms.timers.t7, "T7 must be greater than zero."),
    t8: validatePositive(hsms.timers.t8, "T8 must be greater than zero."),
    deviceName: device.name.trim() ? "" : "Device name is required.",
    protocol: device.protocol.trim() ? "" : "Protocol label is required.",
    mdln: device.mdln.trim() ? "" : "MDLN is required.",
    softrev: device.softrev.trim() ? "" : "SOFTREV is required.",
  };

  const issues = Object.values(errors).filter(Boolean);
  const hostStartupEnabled = hsms.handshake.hostStartupProfile !== "disabled";
  const hostStartupWarning =
    hostStartupEnabled && hsms.mode !== "active"
      ? "Active-mode host startup only runs when the connection mode is active."
      : "";
  const hostStartupDescription = describeHostStartupProfile(hsms.handshake.hostStartupProfile);
  const summaryTone = issues.length > 0 ? "red" : restartRequired || hostStartupWarning ? "yellow" : "green";
  const summaryLabel =
    issues.length > 0
      ? `${issues.length} validation issue${issues.length === 1 ? "" : "s"}`
      : hostStartupWarning
        ? "Startup note"
        : restartRequired
          ? "Restart pending"
          : "Ready";

  return (
    <div className="panel-scroll">
      <div className="panel-scroll-content">
        {issues.length > 0 || hostStartupWarning ? (
          <CollapsibleSection title="Validation Issues" defaultOpen={false} right={<Badge tone={summaryTone}>{summaryLabel}</Badge>}>
            <div className="rule-readiness-list">
              {issues.map((issue) => (
                <div className="meta-note" key={issue}>{issue}</div>
              ))}
            </div>
            {hostStartupWarning ? <div className="meta-note">{hostStartupWarning}</div> : null}
          </CollapsibleSection>
        ) : null}

        <CollapsibleSection title="Connection" right={restartRequired ? <span className="warning-text">restart required</span> : null}>
          <div className="field-row">
            <LabeledSelect
              label="Mode"
              value={hsms.mode}
              onChange={(value) => onChangeHsms({ ...hsms, mode: value })}
              options={["passive", "active"]}
              width={120}
              error={errors.mode}
            />
            <LabeledInput
              label="Address"
              value={hsms.ip}
              onChange={(value) => onChangeHsms({ ...hsms, ip: value })}
              width="1fr"
              mono
              error={errors.ip}
            />
            <LabeledInput
              label="Port"
              value={hsms.port}
              onChange={(value) => onChangeHsms({ ...hsms, port: toNumber(value) })}
              width={90}
              type="number"
              mono
              error={errors.port}
            />
          </div>
          <div className="field-row" style={{ marginTop: 8 }}>
            <LabeledInput
              label="Session ID"
              value={hsms.sessionId}
              onChange={(value) => onChangeHsms({ ...hsms, sessionId: toNumber(value) })}
              width={90}
              type="number"
              mono
              hint="sim cfg"
              error={errors.sessionId}
            />
            <LabeledInput
              label="Device ID (Header)"
              value={hsms.deviceId}
              onChange={(value) => onChangeHsms({ ...hsms, deviceId: toNumber(value) })}
              width={90}
              type="number"
              mono
              hint="HSMS 4-5"
              error={errors.deviceId}
            />
          </div>
          <div className="meta-note">The HSMS wire header uses Device ID in bytes 4-5. Session ID is kept separately for simulator config compatibility.</div>
        </CollapsibleSection>

        <CollapsibleSection title="Timers (seconds)" defaultOpen={false}>
          <div className="field-row compact">
            <LabeledInput
              label="T3"
              value={hsms.timers.t3}
              onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t3: toNumber(value) } })}
              width={72}
              type="number"
              mono
              hint="reply"
              error={errors.t3}
            />
            <LabeledInput
              label="T5"
              value={hsms.timers.t5}
              onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t5: toNumber(value) } })}
              width={72}
              type="number"
              mono
              hint="connect"
              error={errors.t5}
            />
            <LabeledInput
              label="T6"
              value={hsms.timers.t6}
              onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t6: toNumber(value) } })}
              width={72}
              type="number"
              mono
              hint="ctrl txn"
              error={errors.t6}
            />
            <LabeledInput
              label="T7"
              value={hsms.timers.t7}
              onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t7: toNumber(value) } })}
              width={72}
              type="number"
              mono
              hint="not sel."
              error={errors.t7}
            />
            <LabeledInput
              label="T8"
              value={hsms.timers.t8}
              onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t8: toNumber(value) } })}
              width={72}
              type="number"
              mono
              hint="byte"
              error={errors.t8}
            />
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Device Identity" defaultOpen={false}>
          <div className="field-row">
            <LabeledInput
              label="Name"
              value={device.name}
              onChange={(value) => onChangeDevice({ ...device, name: value })}
              width="1fr"
              mono
              error={errors.deviceName}
            />
            <LabeledInput
              label="Protocol"
              value={device.protocol}
              onChange={(value) => onChangeDevice({ ...device, protocol: value })}
              width={100}
              mono
              error={errors.protocol}
            />
          </div>
          <div className="field-row" style={{ marginTop: 8 }}>
            <LabeledInput
              label="MDLN"
              value={device.mdln}
              onChange={(value) => onChangeDevice({ ...device, mdln: value })}
              width="1fr"
              mono
              error={errors.mdln}
            />
            <LabeledInput
              label="SOFTREV"
              value={device.softrev}
              onChange={(value) => onChangeDevice({ ...device, softrev: value })}
              width="1fr"
              mono
              error={errors.softrev}
            />
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Handshake" defaultOpen={false}>
          <div className="stack-list">
            <div className="toggle-row">
              <TogglePill
                checked={hsms.handshake.autoS1f13}
                onToggle={() =>
                  onChangeHsms({
                    ...hsms,
                    handshake: { ...hsms.handshake, autoS1f13: !hsms.handshake.autoS1f13 },
                  })
                }
              />
              <span className="toggle-copy-title">S1F13 Establish Comm</span>
            </div>
            <div className="toggle-row">
              <TogglePill
                checked={hsms.handshake.autoS1f1}
                onToggle={() =>
                  onChangeHsms({
                    ...hsms,
                    handshake: { ...hsms.handshake, autoS1f1: !hsms.handshake.autoS1f1 },
                  })
                }
              />
              <span className="toggle-copy-title">S1F1 Are You There</span>
            </div>
            <div className="toggle-row">
              <TogglePill
                checked={hsms.handshake.autoS1f17}
                onToggle={() =>
                  onChangeHsms({
                    ...hsms,
                    handshake: { ...hsms.handshake, autoS1f17: !hsms.handshake.autoS1f17 },
                  })
                }
              />
              <span className="toggle-copy-title">S1F17 Request On-Line</span>
            </div>
            <div className="toggle-row">
              <TogglePill
                checked={hsms.handshake.autoS2f25}
                onToggle={() =>
                  onChangeHsms({
                    ...hsms,
                    handshake: { ...hsms.handshake, autoS2f25: !hsms.handshake.autoS2f25 },
                  })
                }
              />
              <span className="toggle-copy-title">S2F25 Loopback</span>
            </div>
            <div className="toggle-row">
              <div style={{ flex: 1 }}>
                <LabeledSelect
                  label="Host startup profile"
                  value={hsms.handshake.hostStartupProfile}
                  onChange={(value) =>
                    onChangeHsms({
                      ...hsms,
                      handshake: {
                        ...hsms.handshake,
                        hostStartupProfile: value,
                        autoHostStartup: value !== "disabled",
                      },
                    })
                  }
                  options={hostStartupProfileOptions}
                  width="100%"
                />
                <div className="meta-note">{hostStartupDescription}</div>
              </div>
              <Badge tone="yellow">Active only</Badge>
            </div>
          </div>
        </CollapsibleSection>
      </div>
    </div>
  );
}

function validateRange(value: number, min: number, max: number, message: string): string {
  return value >= min && value <= max ? "" : message;
}

function validatePositive(value: number, message: string): string {
  return value > 0 ? "" : message;
}
