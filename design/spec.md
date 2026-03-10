# SECSIM UI — Implementation Spec

**Stack:** Python, PySide6/PyQt6
**Target:** Desktop dev tool, single-window, dark theme
**UX Reference:** Wireshark (message inspection), VS Code (panel layout)

---

## 1. Window Structure

```
┌─────────────────────────────────────────────────────────┐
│ TOOLBAR                                                 │
├──────────────────────┬──────────────────────────────────┤
│                      │                                  │
│   LEFT PANEL         │   RIGHT PANEL                    │
│   (config side)      │   (monitor side)                 │
│                      │                                  │
│   ┌──────────────┐   │   ┌────────────────────────────┐ │
│   │ Tab Bar      │   │   │ Message List (top)         │ │
│   ├──────────────┤   │   │                            │ │
│   │              │   │   │                            │ │
│   │ Tab Content  │   │   ├────────────────────────────┤ │
│   │              │   │   │ Detail Pane (bottom)       │ │
│   │              │   │   │ (collapsible)              │ │
│   └──────────────┘   │   └────────────────────────────┘ │
├──────────────────────┴──────────────────────────────────┤
│ STATUS BAR                                              │
└─────────────────────────────────────────────────────────┘
```

**Layout engine:** `QSplitter` (horizontal) for left/right. The splitter position persists across sessions via `QSettings`. Default split ratio is 44% left / 56% right.

The right panel uses a `QSplitter` (vertical) for message list / detail pane. Detail pane starts collapsed (height 0) and expands to ~180px when a message is selected.

---

## 2. Toolbar

A single horizontal bar pinned to the top of the window.

### 2.1 Elements (left to right)

| Element | Widget | Behavior |
|---|---|---|
| App logo | `QLabel` | Static text "◆ SECSIM", accent color, bold |
| Status indicator | `QLabel` + 8px circle | Green dot + "LISTENING" when sim active; gray dot + "STOPPED" when inactive |
| Connection summary | `QLabel` | Shows `{mode} · :{port}` from HSMS config. Updates live when config changes |
| *stretch spacer* | `QSpacerItem` | Pushes buttons to the right |
| Start/Stop button | `QPushButton` | Toggles sim state. Label: "▶ Start" / "■ Stop". Color: green-tinted when stopped, red-tinted when running |
| Reload button | `QPushButton` | Triggers config reload from disk without restarting the HSMS listener. Yellow-tinted |

### 2.2 Behavior

- **Start** creates the HSMS listener on the configured port. **Stop** closes the socket and drains pending events.
- **Reload** re-reads the YAML config file, re-parses rules, and resets the state store to `initial_state`. Does NOT drop the current HSMS session — the host stays connected. If the YAML is invalid, show a toast/status bar error and keep the old config.

---

## 3. Left Panel — Tab Bar

Three tabs: **Rules**, **State**, **HSMS**

Use a `QTabWidget` or manual `QStackedWidget` + custom tab buttons for visual control.

---

### 3.1 Rules Tab

This is the core editing surface. It replaces raw YAML editing with a structured card-based interface.

#### 3.1.1 Section Header

A bar at the top showing the rule count and a **"+ New Rule"** button on the right. Clicking it appends a new empty rule at the bottom with defaults:

```
name: "new rule"
enabled: true
match: { stream: 0, function: 0, rcmd: "" }
conditions: []
reply: { stream: 0, function: 0, ack: 0 }
events: []
```

#### 3.1.2 Rule Card (repeated for each rule)

Each rule is a collapsible card in a vertical `QScrollArea`. Only one card is expanded at a time (accordion behavior).

**Collapsed state** — a single row showing:

| Element | Detail |
|---|---|
| Expand arrow | `▶` glyph, rotates 90° when expanded |
| Enable toggle | A small toggle switch (28×16px). Controls `rule.enabled`. Disabled rules are still visible but grayed out and skipped by the rule matcher |
| SxFy badge | Yellow badge showing `S{stream}F{function}` from the match block |
| Rule name | Editable inline text (click to rename). White when enabled, dim when disabled |
| Summary | Right-aligned dim text: `{n} cond · {m} actions` |

**Expanded state** — reveals four sub-sections below the collapsed row:

##### A. "When Received" (match pattern)

Three inline fields in a horizontal row:

| Field | Type | Width | Notes |
|---|---|---|---|
| Stream | `QSpinBox` | 50px | Range 0–127 |
| Function | `QSpinBox` | 50px | Range 0–255 |
| RCMD | `QLineEdit` | 100px | The remote command name string. Free text. Compared against the first item in S2F41's body list |

These define what inbound message this rule matches.

##### B. "If Conditions Met" (preconditions)

A vertical list of key-value condition rows. Each row:

```
[ field (QLineEdit) ] [ = ] [ value (QLineEdit) ] [ × delete button ]
```

- **field** is a dot-path into the state store, e.g. `ports.LP01`, `carrier_exists`, `carriers.CARR001.location`
- **value** is the expected value as a string
- A **"+ Add"** button below/beside the section header appends a new empty condition row

**Matching semantics** (for the spec, not the UI):
- ALL conditions must be true for the rule to fire (AND logic)
- `carrier_exists: X` is a special predicate meaning "key X exists in `state.carriers`"
- Everything else is a direct equality check: `resolve_path(state, field) == value`
- If a rule has zero conditions, it always matches (on the message pattern alone)

##### C. "Then Reply" (response)

Three inline fields:

| Field | Type | Width | Notes |
|---|---|---|---|
| Stream | `QSpinBox` | 50px | Usually same stream as match |
| Function | `QSpinBox` | 50px | Usually match.function + 1 |
| ACK | `QSpinBox` | 50px | 0 = success. Show a hint label: "success" for 0, "reject" for nonzero |

This defines the immediate synchronous reply message.

##### D. "Then Do" (actions timeline)

A vertical list of timed actions, rendered with a visual timeline (vertical line on the left, dots at each node). Each action row:

```
[ +{delay}ms label ] [ action tag ] [ × delete button ]
```

Two action types, distinguished by a dropdown or by the "+ Add" button offering both:

**Event action** (`type: "event"`):
- Fields: `delay` (ms, QSpinBox), `ceid` (string, QLineEdit)
- Displayed as: yellow ⚡ tag reading `S6F11 CEID={ceid}`
- At runtime: after `delay` ms from the reply, emit an S6F11 with this CEID

**Mutate action** (`type: "mutate"`):
- Fields: `delay` (ms, QSpinBox), `target` (dot-path, QLineEdit), `value` (string, QLineEdit)
- Displayed as: orange ✎ tag reading `{target} → {value}`
- At runtime: after `delay` ms, update `state[target] = value`

Actions are ordered by delay. Multiple actions can share the same delay (they fire simultaneously).

**"+ Add" button** should offer a dropdown/menu: "Add Event" or "Add State Mutation".

##### E. Card Action Buttons

A row at the bottom of the expanded card:

| Button | Behavior |
|---|---|
| Duplicate | Deep-copies this rule, appends below with name "{name} (copy)" |
| Delete | Removes after confirmation dialog |
| Export YAML | Copies this single rule as a YAML snippet to clipboard |

#### 3.1.3 Rule Ordering

Rules are matched **top-to-bottom, first match wins**. The order in the list matters. Support drag-to-reorder on the collapsed card headers (use `QDrag` or a simple up/down arrow pair).

#### 3.1.4 Persistence

The Rules tab is the **authoritative editor**. On every edit, the in-memory rule list updates immediately. Changes are written to the YAML config file on:
- Explicit "Save" (Ctrl+S keyboard shortcut)
- App close (with a "save changes?" dialog if dirty)

The file format matches the YAML schema from the design doc.

---

### 3.2 State Tab

Read-only live view of the simulator's in-memory state. Updates in real-time as rules mutate state.

#### 3.2.1 Device Section

Horizontal row showing:
- **Device:** the device name from config (string)
- **Mode:** the current control mode as a colored badge (`online-remote` = green, `online-local` = yellow, `offline` = red)

#### 3.2.2 Ports Section

A vertical list. Each row:

```
[ port name (dim) ] [ status dot ] [ status text ]
```

Status dot colors: blue = occupied, gray = empty, red = blocked.

#### 3.2.3 Carriers Section

A vertical list. Each row:

```
[ carrier ID (teal) ] [ → ] [ location (bright) ]
```

Location updates live when a mutate action fires (e.g., `CARR001` moves from `LP01` to `SHELF_A01`).

#### 3.2.4 Future Consideration

This tab could support **manual state injection** (click a value to override it) for testing edge cases. Not needed for v1 — read-only is sufficient.

---

### 3.3 HSMS Tab

Configuration panel for the HSMS/SECS-II transport layer. All values are editable. Changes require a restart of the listener to take effect (show a "restart required" indicator when dirty).

#### 3.3.1 Connection Section

| Field | Widget | Default | Notes |
|---|---|---|---|
| Mode | `QComboBox` | `passive` | `passive` = listen for incoming connections (simulator is equipment). `active` = connect to a remote host (less common for a sim) |
| Bind Address | `QLineEdit` | `0.0.0.0` | IP to bind to. `0.0.0.0` accepts from any interface |
| Port | `QSpinBox` | `5000` | Range 1024–65535 |

#### 3.3.2 Session Section

| Field | Widget | Default | Notes |
|---|---|---|---|
| Session ID | `QSpinBox` | `1` | HSMS session identifier. Must match the host's expected session ID |
| Device ID | `QSpinBox` | `0` | SECS-II device ID in message headers |

#### 3.3.3 Timers Section

All values in seconds. Displayed in a compact row with short hint labels below each field.

| Timer | Default | Hint |
|---|---|---|
| T3 | 45 | Reply timeout |
| T5 | 10 | Connect separation |
| T6 | 5 | Control transaction |
| T7 | 10 | Not selected |
| T8 | 5 | Inter-byte |

These correspond directly to the HSMS standard timers. The simulator uses them to detect host timeouts and to pace its own responses.

#### 3.3.4 Device Identity Section

| Field | Default | Used In |
|---|---|---|
| MDLN (Model Name) | `STOCKER-SIM` | S1F14 reply body |
| SOFTREV (Software Rev) | `1.0.0` | S1F14 reply body |

#### 3.3.5 Handshake Behavior Section

Toggle switches controlling auto-responses to standard messages:

| Toggle | Default | Behavior when ON |
|---|---|---|
| Auto S1F13 (Establish Comm) | ON | Automatically reply with S1F14 using MDLN/SOFTREV. If OFF, the message is logged but not replied to (for testing timeout behavior) |
| Auto S1F1 (Are You There) | ON | Automatically reply with S1F2 |
| Auto S2F25 (Loopback) | OFF | Echo back S2F26 with the same data. Rarely needed |

These exist so the user doesn't have to write rules for standard housekeeping messages.

---

## 4. Right Panel — Message Monitor

Modeled after Wireshark's three-pane layout, but with two panes (list + detail).

### 4.1 Message List (top)

A `QTableView` or `QTreeView` backed by a custom model.

#### 4.1.1 Columns

| Column | Width | Content |
|---|---|---|
| Time | 90px, fixed | Timestamp with millisecond precision: `HH:MM:SS.mmm` |
| Dir | 36px, fixed | Badge: green "IN" or blue "OUT" |
| SxFy | 55px, fixed | Message ID like `S2F41`, yellow, bold |
| Info | stretch | Human-readable label. For S2F41: show the RCMD name. For S6F11: show the CEID. For handshake: show "Establish Comm", etc. |
| Matched Rule | 110px, fixed | Rule name if a rule matched this message, or "—" if none. Accent-colored when present, dim when absent |

#### 4.1.2 Row Behavior

- **Click** a row: selects it, opens the detail pane below with that message's data. Click again to deselect (collapses detail pane).
- **Highlight**: selected row has accent-tinted background + left border. Hover shows subtle background tint on unselected rows.
- **Auto-scroll**: new messages appear at the bottom. If the user is scrolled to the bottom, auto-scroll follows. If the user has scrolled up (reading history), auto-scroll stops until they scroll back to the bottom.
- **Context menu** (right-click): Copy SML, Copy raw hex, Jump to rule (opens the rule in the left panel), Clear log.

#### 4.1.3 Filtering (v2 consideration)

A filter bar above the list with:
- Direction filter: All / IN / OUT
- SxFy filter: text input
- Rule filter: dropdown of rule names
- Free text search across the Info column

Not required for v1. The list is small enough for visual scanning during single-device debugging.

### 4.2 Detail Pane (bottom, collapsible)

Appears below the message list when a message is selected. Uses a vertical `QSplitter` so the user can drag the boundary.

Three sub-tabs:

#### 4.2.1 "Decoded" Tab

Shows structured fields:

```
Stream: {n}    Function: {n}    W-bit: {yes/no}    Direction: {IN/OUT badge}

Body (SML tree):
  L:2
    <A "TRANSFER">
    L:2
      L:2 <A "CarrierID"> <A "CARR001">
      L:2 <A "SourcePort"> <A "LP01">
```

The body is rendered as indented SML (SECS Message Language) text. Use a `QTextEdit` (read-only) with monospace font. Apply syntax coloring:
- `L:n` list headers → dim
- `<A "...">` ASCII items → teal
- `<B 0x..>` binary items → yellow
- `<U4 ...>` numeric items → blue

#### 4.2.2 "Raw SML" Tab

The flat SML one-liner as-is, no tree formatting. Useful for copy-paste into other tools.

#### 4.2.3 "Matched Rule" Tab

If the message was triggered by or generated by a rule:
- Show the rule name (linked — clicking it switches the left panel to the Rules tab and expands that rule)
- Show which conditions were evaluated and their results (for debugging why a rule did/didn't match)

If no rule matched: show "No rule matched — session/handshake message".

---

## 5. Status Bar

A single row at the bottom of the window. Always visible.

### 5.1 Elements (left to right)

| Element | Content |
|---|---|
| HSMS state | `HSMS: {state}` where state is one of: `NOT CONNECTED`, `CONNECTED`, `SELECTED`. Green when SELECTED |
| Session ID | `Session ID: {n}` |
| Connection | `{mode} · {ip}:{port}` |
| Message count | `Messages: {n}` (total since last clear) |
| *stretch spacer* | |
| Rule count | `Rules: {n}` |
| Config file | Filename of loaded config, or "unsaved" if new |
| Dirty indicator | `●` dot if there are unsaved changes to rules/config |

---

## 6. Data Flow

### 6.1 Config File → Internal State

```
YAML file on disk
  ↓ (parse on startup / reload)
SimConfig object
  ├── hsms: HsmsConfig
  ├── device: DeviceConfig (name, protocol, identity)
  ├── initial_state: StateSnapshot
  └── rules: List[Rule]
```

On load:
1. Parse YAML → `SimConfig`
2. Deep-copy `initial_state` into the live `StateStore`
3. Compile `rules` into `List[CompiledRule]` (pre-validate dot-paths, cache field lookups)
4. Apply `HsmsConfig` to the listener (restart if port/mode changed)

### 6.2 Message Processing Pipeline

```
HSMS socket receives bytes
  ↓
Message parser (HSMS frame → SECS-II message)
  ↓
Auto-response check (S1F13, S1F1, S2F25)
  ├── if auto-handled → send reply, log, done
  └── else ↓
Rule matcher (iterate rules top-to-bottom)
  ├── check message pattern (stream, function, rcmd)
  ├── check conditions against StateStore
  ├── first full match wins
  └── if no match → log as unmatched, no reply
        ↓
Execute matched rule:
  1. Build reply message from ReplyTemplate → send immediately
  2. Schedule each action:
     - EventAction: after delay, build S6F11 → send
     - MutateAction: after delay, update StateStore
  3. Log everything (inbound, reply, each emitted event)
```

### 6.3 UI Update Signals

Use Qt signals/slots to decouple the backend from the UI:

| Signal | Source | Consumed By |
|---|---|---|
| `message_logged(MessageRecord)` | Message pipeline | Message List model |
| `state_changed(path, old, new)` | StateStore | State Tab |
| `rule_matched(rule_name, msg_id)` | Rule matcher | Message List (matched column) |
| `hsms_state_changed(state)` | HSMS listener | Toolbar status, Status bar |
| `config_dirty_changed(bool)` | Config manager | Status bar dirty indicator |

---

## 7. Threading Model

```
Main thread (Qt event loop)
  ├── All UI rendering
  ├── Config editing
  └── Signal/slot dispatch

HSMS thread (QThread)
  ├── Socket accept / read / write
  ├── Message parse
  ├── Rule match + reply send
  └── Emits signals to main thread

Timer thread (QThread or QTimer on HSMS thread)
  ├── Manages delayed action queue
  └── Fires events/mutations at scheduled times
```

**Critical rule:** Never access the UI from the HSMS thread. All communication is via `Qt.QueuedConnection` signals. The `StateStore` needs a lock (`QMutex`) since the HSMS thread writes and the UI thread reads.

---

## 8. Keyboard Shortcuts

| Shortcut | Action |
|---|---|
| `Ctrl+S` | Save config to YAML file |
| `Ctrl+R` | Reload config from disk |
| `Ctrl+N` | New rule |
| `Ctrl+L` | Clear message log |
| `Ctrl+1` | Switch to Rules tab |
| `Ctrl+2` | Switch to State tab |
| `Ctrl+3` | Switch to HSMS tab |
| `Escape` | Deselect current message (collapse detail pane) |
| `Ctrl+F` | Focus message filter (v2) |

---

## 9. Color Scheme

All colors defined in a single `Theme` dict/dataclass for easy swapping.

| Token | Hex | Usage |
|---|---|---|
| `bg` | `#1e1e2e` | Window background |
| `surface` | `#252536` | Toolbar, status bar, panels |
| `surface_alt` | `#2a2a3d` | Section headers, column headers |
| `border` | `#353548` | Panel borders, separators |
| `border_active` | `#6c6cff` | Focused inputs, selected items |
| `text` | `#cdd6f4` | Default text |
| `text_dim` | `#6c7086` | Labels, hints, timestamps |
| `text_bright` | `#f0f0ff` | Emphasized text, active items |
| `accent` | `#6c6cff` | Primary accent (selection, links, rule names) |
| `green` | `#a6e3a1` | Status: connected/enabled, IN badges |
| `red` | `#f38ba8` | Status: error/blocked, delete buttons |
| `yellow` | `#f9e2af` | SxFy labels, event actions |
| `blue` | `#89b4fa` | OUT badges, occupied ports |
| `teal` | `#94e2d5` | SML values, carrier IDs |
| `orange` | `#fab387` | Mutate actions |

---

## 10. File Format Reference

The YAML config file that this UI edits:

```yaml
# HSMS transport configuration
hsms:
  mode: passive          # passive | active
  ip: "0.0.0.0"
  port: 5000
  session_id: 1
  device_id: 0
  timers:
    t3: 45
    t5: 10
    t6: 5
    t7: 10
    t8: 5

# Device identity (used in S1F14 replies)
device:
  name: stocker-A
  protocol: e88
  mdln: "STOCKER-SIM"
  softrev: "1.0.0"

# Auto-response toggles for standard messages
handshake:
  auto_s1f13: true
  auto_s1f1: true
  auto_s2f25: false

# Initial state snapshot — loaded on start/reload
initial_state:
  mode: online-remote
  ports:
    LP01: occupied
    LP02: empty
  carriers:
    CARR001:
      location: LP01

# Rules — matched top-to-bottom, first match wins
rules:
  - name: accept transfer
    enabled: true
    match:
      stream: 2
      function: 41
      rcmd: TRANSFER
    conditions:
      - field: carrier_exists
        value: CARR001
      - field: source_equals
        value: LP01
    reply:
      stream: 2
      function: 42
      ack: 0
    events:
      - delay_ms: 300
        type: event
        ceid: TRANSFER_INITIATED
      - delay_ms: 1200
        type: mutate
        target: carriers.CARR001.location
        value: SHELF_A01
      - delay_ms: 1200
        type: event
        ceid: TRANSFER_COMPLETED
```

---

## 11. Implementation Order

Suggested build sequence, each step produces a testable increment:

1. **HSMS listener + message parser** — can accept connections, parse/log messages to stdout
2. **Rule engine + state store** — load YAML, match rules, send replies, emit timed events
3. **Message monitor UI** — right panel only, read-only, connected to signals from step 2
4. **HSMS config tab** — edit transport settings, restart listener
5. **State tab** — live state view
6. **Rules tab** — visual editor, read from/write to YAML
7. **Polish** — keyboard shortcuts, drag reorder, context menus, filter bar
