import type { ReactNode } from "react";

type BadgeTone = "green" | "blue" | "yellow" | "accent" | "red" | "teal" | "neutral";

interface BadgeProps {
  tone: BadgeTone;
  children: ReactNode;
}

interface SectionHeaderProps {
  children: ReactNode;
  right?: ReactNode;
}

interface TabButtonProps {
  active: boolean;
  children: ReactNode;
  icon?: string;
  onClick: () => void;
}

interface LabeledInputProps {
  label: string;
  value: string | number;
  onChange: (value: string) => void;
  width?: number | string;
  type?: "text" | "number";
  hint?: string;
  mono?: boolean;
  min?: number;
  max?: number;
}

interface LabeledSelectProps {
  label: string;
  value: string;
  options: string[];
  onChange: (value: string) => void;
  width?: number | string;
}

interface TogglePillProps {
  checked: boolean;
  onToggle: () => void;
  disabled?: boolean;
}

interface ActionButtonProps {
  children: ReactNode;
  onClick: () => void;
  variant?: "neutral" | "accent" | "danger" | "warning";
}

export function Badge({ tone, children }: BadgeProps) {
  return <span className={`badge badge-${tone}`}>{children}</span>;
}

export function SectionHeader({ children, right }: SectionHeaderProps) {
  return (
    <div className="section-header">
      <span>{children}</span>
      {right ? <div>{right}</div> : null}
    </div>
  );
}

export function TabButton({ active, children, icon, onClick }: TabButtonProps) {
  return (
    <button className={`tab-button ${active ? "active" : ""}`} onClick={onClick} type="button">
      {icon ? <span className="tab-icon">{icon}</span> : null}
      <span>{children}</span>
    </button>
  );
}

export function LabeledInput({
  label,
  value,
  onChange,
  width,
  type = "text",
  hint,
  mono,
  min,
  max,
}: LabeledInputProps) {
  return (
    <label className="field-group" style={width ? { width } : undefined}>
      <span className="field-label">{label}</span>
      <input
        className={`field-input ${mono ? "mono" : ""}`}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        type={type}
        min={min}
        max={max}
      />
      {hint ? <span className="field-hint">{hint}</span> : null}
    </label>
  );
}

export function LabeledSelect({
  label,
  value,
  options,
  onChange,
  width,
}: LabeledSelectProps) {
  return (
    <label className="field-group" style={width ? { width } : undefined}>
      <span className="field-label">{label}</span>
      <select className="field-input" value={value} onChange={(event) => onChange(event.target.value)}>
        {options.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    </label>
  );
}

export function TogglePill({ checked, onToggle, disabled }: TogglePillProps) {
  return (
    <button
      className={`toggle-pill ${checked ? "checked" : ""}`}
      onClick={onToggle}
      type="button"
      disabled={disabled}
      aria-pressed={checked}
    >
      <span className="toggle-thumb" />
    </button>
  );
}

export function ActionButton({ children, onClick, variant = "neutral" }: ActionButtonProps) {
  return (
    <button className={`action-button ${variant}`} onClick={onClick} type="button">
      {children}
    </button>
  );
}

