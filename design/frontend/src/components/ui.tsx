import { useState, type ReactNode } from "react";

type BadgeTone = "green" | "blue" | "yellow" | "accent" | "red" | "teal" | "neutral";

interface BadgeProps {
  tone: BadgeTone;
  children: ReactNode;
}

interface SectionHeaderProps {
  children: ReactNode;
  right?: ReactNode;
}

interface CollapsibleSectionProps {
  title: string;
  children: ReactNode;
  right?: ReactNode;
  defaultOpen?: boolean;
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
  error?: string;
}

interface LabeledSelectProps {
  label: string;
  value: string;
  options: string[];
  onChange: (value: string) => void;
  width?: number | string;
  error?: string;
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
  className?: string;
}

export function Badge({ tone, children }: BadgeProps) {
  return <span className={`badge badge-${tone}`}>{children}</span>;
}

export function SectionHeader({ children, right }: SectionHeaderProps) {
  return (
    <div className="section-header">
      <span className="section-title">{children}</span>
      {right ? <div className="section-actions">{right}</div> : null}
    </div>
  );
}

export function TabButton({ active, children, icon, onClick }: TabButtonProps) {
  return (
    <button className={`tab-button ${active ? "active" : ""}`} onClick={onClick} type="button">
      {icon ? <span className="tab-icon">{icon}</span> : null}
      <span className="tab-label">{children}</span>
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
  error,
}: LabeledInputProps) {
  return (
    <label className={`field-group ${error ? "has-error" : ""}`} style={width ? { width } : undefined}>
      <span className="field-label">{label}</span>
      <input
        className={`field-input ${mono ? "mono" : ""} ${error ? "invalid" : ""}`}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        type={type}
        min={min}
        max={max}
        spellCheck={false}
      />
      {hint ? <span className="field-hint">{hint}</span> : null}
      {error ? <span className="field-error">{error}</span> : null}
    </label>
  );
}

export function LabeledSelect({
  label,
  value,
  options,
  onChange,
  width,
  error,
}: LabeledSelectProps) {
  return (
    <label className={`field-group ${error ? "has-error" : ""}`} style={width ? { width } : undefined}>
      <span className="field-label">{label}</span>
      <div className="select-wrapper">
        <select
          className={`field-input ${error ? "invalid" : ""}`}
          value={value}
          onChange={(event) => onChange(event.target.value)}
        >
          {options.map((option) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </select>
      </div>
      {error ? <span className="field-error">{error}</span> : null}
    </label>
  );
}

export function TogglePill({ checked, onToggle, disabled }: TogglePillProps) {
  return (
    <button
      className={`toggle-pill ${checked ? "checked" : ""}`}
      onClick={(e) => {
        e.stopPropagation();
        onToggle();
      }}
      type="button"
      disabled={disabled}
      aria-pressed={checked}
    >
      <span className="toggle-thumb" />
    </button>
  );
}

export function ActionButton({ children, onClick, variant = "neutral", className = "" }: ActionButtonProps) {
  return (
    <button className={`action-button ${variant} ${className}`} onClick={onClick} type="button">
      {children}
    </button>
  );
}

export function CollapsibleSection({ title, children, right, defaultOpen = true }: CollapsibleSectionProps) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className="collapsible-section">
      <button className="collapsible-header" onClick={() => setOpen(!open)} type="button">
        <span className={`collapsible-chevron ${open ? "open" : ""}`}>▶</span>
        <span className="collapsible-title">{title}</span>
        {right ? <span className="collapsible-right">{right}</span> : null}
      </button>
      {open ? <div className="collapsible-body">{children}</div> : null}
    </div>
  );
}
