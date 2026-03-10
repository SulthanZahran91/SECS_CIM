import { LabeledInput, LabeledSelect, SectionHeader, TogglePill } from "./ui";
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
  return (
    <div className="panel-scroll">
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
          />
          <LabeledInput
            label="Bind Address"
            value={hsms.ip}
            onChange={(value) => onChangeHsms({ ...hsms, ip: value })}
            width={180}
            mono
          />
          <LabeledInput
            label="Port"
            value={hsms.port}
            onChange={(value) => onChangeHsms({ ...hsms, port: toNumber(value) })}
            width={120}
            type="number"
            mono
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
          />
          <LabeledInput
            label="Device ID"
            value={hsms.deviceId}
            onChange={(value) => onChangeHsms({ ...hsms, deviceId: toNumber(value) })}
            width={120}
            type="number"
            mono
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
          />
          <LabeledInput
            label="T5"
            value={hsms.timers.t5}
            onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t5: toNumber(value) } })}
            width={92}
            type="number"
            mono
            hint="connect sep."
          />
          <LabeledInput
            label="T6"
            value={hsms.timers.t6}
            onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t6: toNumber(value) } })}
            width={92}
            type="number"
            mono
            hint="control txn"
          />
          <LabeledInput
            label="T7"
            value={hsms.timers.t7}
            onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t7: toNumber(value) } })}
            width={92}
            type="number"
            mono
            hint="not selected"
          />
          <LabeledInput
            label="T8"
            value={hsms.timers.t8}
            onChange={(value) => onChangeHsms({ ...hsms, timers: { ...hsms.timers, t8: toNumber(value) } })}
            width={92}
            type="number"
            mono
            hint="inter-byte"
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
          />
          <LabeledInput
            label="Protocol"
            value={device.protocol}
            onChange={(value) => onChangeDevice({ ...device, protocol: value })}
            width={140}
            mono
          />
          <LabeledInput
            label="MDLN"
            value={device.mdln}
            onChange={(value) => onChangeDevice({ ...device, mdln: value })}
            width={180}
            mono
          />
          <LabeledInput
            label="SOFTREV"
            value={device.softrev}
            onChange={(value) => onChangeDevice({ ...device, softrev: value })}
            width={140}
            mono
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
          <span>Auto-respond to S1F13 (Establish Comm)</span>
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
          <span>Auto-respond to S1F1 (Are You There)</span>
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
          <span>Auto-respond to S2F25 (Loopback)</span>
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
          <span>Active-mode host startup (S1F13, S1F17, S2F31, S6F12)</span>
        </div>
      </div>
    </div>
  );
}
