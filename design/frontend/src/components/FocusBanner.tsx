import { Badge } from "./ui";
import type { Snapshot } from "../types";

interface FocusBannerProps {
  snapshot: Snapshot;
}

type BadgeTone = "green" | "blue" | "yellow" | "accent" | "red";

interface FocusState {
  tone: BadgeTone;
  badge: string;
  title: string;
}

export function FocusBanner({ snapshot }: FocusBannerProps) {
  const focus = summarizeFocus(snapshot);

  return (
    <div className={`focus-banner tone-${focus.tone}`}>
      <Badge tone={focus.tone}>{focus.badge}</Badge>
      <span className="focus-title">{focus.title}</span>
    </div>
  );
}

function summarizeFocus(snapshot: Snapshot): FocusState {
  if (snapshot.runtime.lastError) {
    return { tone: "red", badge: "Transport issue", title: snapshot.runtime.lastError };
  }
  if (snapshot.runtime.restartRequired) {
    return { tone: "yellow", badge: "Restart required", title: "Restart the runtime to apply connection changes" };
  }
  if (snapshot.runtime.dirty) {
    return { tone: "yellow", badge: "Unsaved", title: "Commit or discard the current config edits" };
  }
  if (!snapshot.runtime.listening) {
    return { tone: "accent", badge: "Stopped", title: "Start the simulator to begin" };
  }
  if (snapshot.messages.length === 0) {
    return { tone: "blue", badge: "Waiting", title: "Connect a host to begin exercising rules" };
  }
  return { tone: "green", badge: "Active", title: `${snapshot.messages.length} messages captured` };
}
