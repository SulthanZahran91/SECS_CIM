import { Badge, LabeledInput, LabeledSelect, SectionHeader, TogglePill } from "./ui";
import type { DeviceConfig, HsmsConfig } from "../types";

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

export function HsmsTab({ hsms, device, restartRequired, onChangeHsms, onChangeDevice }: HsmsTabProps) {
  const errors = {
    mode: hsms.mode === "passive" || hsms.mode === "active" ? "" : "Choose passive or active.",
    ip: hsms.ip.trim() ? "" : "Address is required.",
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
  const hostStartupWarning =
    hsms.handshake.autoHostStartup && hsms.mode !== "active"
      ? "Active-mode host startup only runs when the connection mode is active."
      : "";
  const summaryTone = issues.length > 0 ? "red" : restartRequired || hostStartupWarning ? "yellow" : "green";
  const summaryLabel =
    issues.length > 0 ? `${issues.length} validation issue${issues.length === 1 ? "" : "s"}` : restartRequired ? "Restart pending" : "Ready";

  return (
    <div className="panel-scroll">
      <div className="panel-scroll-content">
        <SectionHeader right={<Badge tone={summaryTone}>{summaryLabel}</Badge>}>Connection Readiness</SectionHeader>
        <div className="section-body">
          <div className={`hsms-callout ${issues.length > 0 ? "warning" : restartRequired || hostStartupWarning ? "pending" : "ready"}`}>
            <div className="hsms-callout-header">
              <div className="hsms-callout-copy-block">
                <div className="rule-section-title">What applies when</div>
                <p className="overview-copy">
                  Save writes the working config to disk. Restart applies mode, address, and port changes to the live runtime.
                </p>
              </div>
              <Badge tone={summaryTone}>{summaryLabel}</Badge>
            </div>
            {issues.length > 0 ? (
              <div className="rule-readiness-list">
                {issues.map((issue) => (
                  <div className="meta-note" key={issue}>
                    {issue}
                  </div>
                ))}
              </div>
            ) : (
              <div className="meta-note">No blocking validation issues detected in the current HSMS/device profile.</div>
            )}
            {hostStartupWarning ? <div className="meta-note">{hostStartupWarning}</div> : null}
          </div>
        </div>

        <SectionHeader right={restartRequired ? <span className="warning-text">restart required</span> : null}>
          Connection
        </SectionHeader>
        <div className="section-body">
          <div className="field-row">
            <LabeledSelect
              label="Mode"
              value={hsms.mode}
              onChange={(value) => onChangeHsms({ ...hsms, mode: value })}
              options={["passive", "active"]}
              width={140}
              error={errors.mode}
            />
            <LabeledInput
              label="Bind Address"
              value={hsms.ip}
              onChange={(value) => onChangeHsms({ ...hsms, ip: value })}
              width={180}
              mono
              error={errors.ip}
            />
            <LabeledInput
              label="Port"
              value={hsms.port}
              onChange={(value) => onChangeHsms({ ...hsms, port: toNumber(value) })}
              width={120}
              type="number"
              mono
              error={errors.port}
            />
          </div>
        </div>

        <SectionHeader>Session</SectionHeader>
        <div className="section-body">
          <div className="field-row">
            <LabeledInput
              label="Session ID"
              value={hsms.sessionId}
              onChange={(value) => onChangeHsms({ ...hsms, sessionId: toNumber(value) })}
              width={120}
              type="number"
              mono
              error={errors.sessionId}
            />
            <LabeledInput
              label="Device ID"
              value={hsms.deviceId}
              onChange={(value) => onChangeHsms({ ...hsms, deviceId: toNumber(value) })}
              width={120}
              type="number"
              mono
              error={errors.deviceId}
            />
          </div>
        </div>

        <SectionHeader right={<span className="subtle-text">seconds</span>}>Timers</SectionHeader>
        <div className="section-body">
          <div className="field-row compact">
            <LabeledInput
              label="T3"
              value={hsms.timers.t3}
              onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t3: toNumber(value) } })}
              width={92}
              type="number"
              mono
              hint="reply timeout"
              error={errors.t3}
            />
            <LabeledInput
              label="T5"
              value={hsms.timers.t5}
              onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t5: toNumber(value) } })}
              width={92}
              type="number"
              mono
              hint="connect sep."
              error={errors.t5}
            />
            <LabeledInput
              label="T6"
              value={hsms.timers.t6}
              onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t6: toNumber(value) } })}
              width={92}
              type="number"
              mono
              hint="control txn"
              error={errors.t6}
            />
            <LabeledInput
              label="T7"
              value={hsms.timers.t7}
              onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t7: toNumber(value) } })}
              width={92}
              type="number"
              mono
              hint="not selected"
              error={errors.t7}
            />
            <LabeledInput
              label="T8"
              value={hsms.timers.t8}
              onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t8: toNumber(value) } })}
              width={92}
              type="number"
              mono
              hint="inter-byte"
              error={errors.t8}
            />
          </div>
        </div>

        <SectionHeader>Device Identity</SectionHeader>
        <div className="section-body">
          <div className="field-row">
            <LabeledInput
              label="Device Name"
              value={device.name}
              onChange={(value) => onChangeDevice({ ...device, name: value })}
              width={180}
              mono
              error={errors.deviceName}
            />
            <LabeledInput
              label="Protocol"
              value={device.protocol}
              onChange={(value) => onChangeDevice({ ...device, protocol: value })}
              width={140}
              mono
              error={errors.protocol}
            />
            <LabeledInput
              label="MDLN"
              value={device.mdln}
              onChange={(value) => onChangeDevice({ ...device, mdln: value })}
              width={180}
              mono
              error={errors.mdln}
            />
            <LabeledInput
              label="SOFTREV"
              value={device.softrev}
              onChange={(value) => onChangeDevice({ ...device, softrev: value })}
              width={140}
              mono
              error={errors.softrev}
            />
          </div>
        </div>

        <SectionHeader>Handshake Behavior</SectionHeader>
        <div className="section-body stack-list">
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
            <div className="toggle-copy-block">
              <span className="toggle-copy-title">Auto-respond to S1F13 (Establish Comm)</span>
              <span className="toggle-copy-hint">Useful when acting as equipment and expecting a host handshake.</span>
            </div>
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
            <div className="toggle-copy-block">
              <span className="toggle-copy-title">Auto-respond to S1F1 (Are You There)</span>
              <span className="toggle-copy-hint">Keeps common availability checks from needing an explicit rule.</span>
            </div>
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
            <div className="toggle-copy-block">
              <span className="toggle-copy-title">Auto-respond to S2F25 (Loopback)</span>
              <span className="toggle-copy-hint">Good default when the host uses loopback for transport validation.</span>
            </div>
          </div>
          <div className="toggle-row">
            <TogglePill
              checked={hsms.handshake.autoHostStartup}
              onToggle={() =>
                onChangeHsms({
                  ...hsms,
                  handshake: { ...hsms.handshake, autoHostStartup: !hsms.handshake.autoHostStartup },
                })
              }
            />
            <div className="toggle-copy-block">
              <div className="toggle-copy-header">
                <span className="toggle-copy-title">Active-mode host startup (S1F13, S1F17, S2F31, S6F12)</span>
                <Badge tone="yellow">Active only</Badge>
              </div>
              <span className="toggle-copy-hint">Preloads a minimal host-side bring-up sequence after connect/select.</span>
            </div>
          </div>
        </div>
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
