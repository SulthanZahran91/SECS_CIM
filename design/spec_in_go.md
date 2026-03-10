# SECSIM — Implementation Spec

**Architecture:** Go backend (single binary) + embedded web UI (React)
**Transport:** HSMS-SS implemented from scratch in Go
**UI delivery:** Go serves static files + WebSocket for real-time updates, REST for config CRUD
**UX Reference:** Wireshark (message inspection)

---

## 1. High-Level Architecture

```
┌──────────────────────────────────────────────────────────┐
│                      Go Binary                           │
│                                                          │
│  ┌──────────────┐   ┌──────────────┐   ┌─────────────┐  │
│  │ HSMS Server   │──▶│ Rule Engine  │──▶│ State Store │  │
│  │ (TCP :5000)   │   │              │   │ (in-memory) │  │
│  └──────┬───────┘   └──────┬───────┘   └──────┬──────┘  │
│         │                  │                   │         │
│         ▼                  ▼                   ▼         │
│  ┌─────────────────────────────────────────────────────┐ │
│  │              Event Bus (Go channels)                │ │
│  └─────────────────────┬───────────────────────────────┘ │
│                        │                                 │
│  ┌─────────────────────▼───────────────────────────────┐ │
│  │              HTTP Server (:8080)                     │ │
│  │   GET  /                → serve React SPA           │ │
│  │   GET  /api/config      → read config               │ │
│  │   PUT  /api/config      → update config             │ │
│  │   GET  /api/state       → read current state        │ │
│  │   POST /api/sim/start   → start HSMS listener       │ │
│  │   POST /api/sim/stop    → stop HSMS listener        │ │
│  │   POST /api/sim/reload  → reload config from disk   │ │
│  │   WS   /ws              → real-time event stream    │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌─────────────────────────────────────────────────────┐ │
│  │  Embedded Static Files (embed.FS)                   │ │
│  │  React build output baked into the Go binary        │ │
│  └─────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘

        Browser (:8080)
┌──────────────────────────────────────────┐
│  React SPA                               │
│  ┌──────────────┬───────────────────────┐│
│  │ Left Panel   │ Right Panel           ││
│  │ Rules/State/ │ Message Log +         ││
│  │ HSMS Config  │ Detail Pane           ││
│  └──────────────┴───────────────────────┘│
└──────────────────────────────────────────┘
```

The Go binary is the **entire application** — one `go build`, one executable, no Python env, no Node runtime, no external dependencies. The browser is just a view.

---

## 2. Go Backend — Package Structure

```
secsim/
├── cmd/
│   └── secsim/
│       └── main.go              # CLI entry, flag parsing, startup
├── internal/
│   ├── hsms/
│   │   ├── conn.go              # HSMS-SS TCP connection management
│   │   ├── frame.go             # 10-byte header + body framing
│   │   ├── message.go           # SECS-II message encode/decode
│   │   ├── session.go           # Select/deselect state machine
│   │   ├── items.go             # SECS-II data item types (List, ASCII, Binary, Uint)
│   │   └── timers.go            # T3, T5, T6, T7, T8 timeout management
│   ├── engine/
│   │   ├── rule.go              # Rule data structures
│   │   ├── matcher.go           # Top-to-bottom rule matching
│   │   ├── actions.go           # Delayed event emission + state mutation
│   │   └── scheduler.go         # Timer-based action queue (time.AfterFunc)
│   ├── state/
│   │   ├── store.go             # In-memory state with dot-path get/set
│   │   └── snapshot.go          # Serialize/deserialize state snapshots
│   ├── config/
│   │   ├── config.go            # Top-level SimConfig struct
│   │   ├── loader.go            # YAML parse + validate
│   │   └── writer.go            # Serialize back to YAML
│   ├── api/
│   │   ├── server.go            # HTTP server setup, routes
│   │   ├── handlers.go          # REST endpoint handlers
│   │   └── ws.go                # WebSocket hub (gorilla/websocket or nhooyr)
│   └── bus/
│       └── bus.go               # Event bus: typed channels + fan-out to WS clients
├── web/                          # React frontend source
│   ├── src/
│   │   ├── App.tsx
│   │   ├── components/
│   │   │   ├── Toolbar.tsx
│   │   │   ├── RulesPanel.tsx
│   │   │   ├── RuleCard.tsx
│   │   │   ├── StatePanel.tsx
│   │   │   ├── HsmsConfigPanel.tsx
│   │   │   ├── MessageList.tsx
│   │   │   └── DetailPane.tsx
│   │   ├── hooks/
│   │   │   ├── useWebSocket.ts   # WS connection + reconnect logic
│   │   │   └── useApi.ts         # REST helpers
│   │   └── types.ts              # Shared type definitions
│   ├── package.json
│   └── vite.config.ts
├── static/                       # Built React output (embedded via go:embed)
├── go.mod
└── config.example.yaml
```

---

## 3. HSMS-SS Protocol Layer (`internal/hsms/`)

Since there's no Go SECS/GEM library, this is implemented from scratch. Scoped to **only what the simulator needs** — not a general-purpose library.

### 3.1 HSMS Frame Format

```
Bytes 0–3:   Message length (uint32, big-endian, excludes these 4 bytes)
Bytes 4–5:   Session ID (uint16, big-endian)
Byte  6:     Header byte 2 — W-bit (bit 7) + Stream (bits 6–0)
Byte  7:     Function
Bytes 8–9:   PType (always 0) + SType (0 = data, 1–9 = control)
Bytes 10+:   SECS-II body (only for data messages, SType=0)
```

Implement as:

```go
type Frame struct {
    SessionID uint16
    Stream    byte
    Function  byte
    WBit      bool
    SType     byte      // 0=data, 1=select.req, 2=select.rsp, ...
    Body      []byte    // raw SECS-II encoded body
}

func ReadFrame(r io.Reader) (*Frame, error)  // reads 4-byte length, then payload
func WriteFrame(w io.Writer, f *Frame) error
```

### 3.2 SECS-II Data Items (`items.go`)

Only implement the item types your messages actually use:

| Format Code | Type | Go representation |
|---|---|---|
| `000000` (0) | List (L) | `[]Item` |
| `010000` (16) | Binary (B) | `[]byte` |
| `010100` (20) | Boolean | `bool` |
| `100000` (32) | ASCII (A) | `string` |
| `101000` (40) | U4 | `uint32` |
| `101100` (44) | U1 | `uint8` |

```go
type Item struct {
    Type     byte
    Children []Item   // for List
    Value    any      // string, uint32, []byte, etc.
}

func DecodeItem(data []byte) (Item, int, error)   // recursive decoder
func EncodeItem(item Item) []byte                  // recursive encoder
```

This is the most tedious part to implement. Keep it minimal — add types only when a real message needs them.

### 3.3 Session State Machine

```
NOT CONNECTED
  │
  ▼ (TCP accept)
CONNECTED
  │
  ▼ (recv Select.req → send Select.rsp with status=0)
SELECTED ◀──────────────────────────────────────────┐
  │                                                  │
  ├── recv Data Message → process via rule engine     │
  ├── recv Linktest.req → send Linktest.rsp           │
  ├── recv Separate → go to NOT CONNECTED             │
  └── T7 timeout (if stuck in CONNECTED) → close     ┘
```

Implement as a simple state enum + switch in the connection goroutine.

### 3.4 Timers to Implement

| Timer | Where Used | Behavior |
|---|---|---|
| T3 | After sending W-bit=1 message | If no reply within T3, log timeout warning |
| T5 | After connection drops | Wait T5 before accepting new connection |
| T6 | After sending control message | If no control response within T6, close |
| T7 | After TCP connect, before Select | If not selected within T7, close |
| T8 | During frame read | Inter-byte timeout (often handled by TCP read deadline) |

For a simulator, T7 and T6 are the most important. T3 is less critical since the simulator is mostly reactive (the host drives the conversation).

---

## 4. Rule Engine (`internal/engine/`)

### 4.1 Rule Structure

```go
type Rule struct {
    Name       string       `yaml:"name"`
    Enabled    bool         `yaml:"enabled"`
    Match      MatchPattern `yaml:"match"`
    Conditions []Condition  `yaml:"conditions"`
    Reply      ReplyDef     `yaml:"reply"`
    Events     []ActionDef  `yaml:"events"`
    Order      int          // position in list, set on load
}

type MatchPattern struct {
    Stream   byte   `yaml:"stream"`
    Function byte   `yaml:"function"`
    RCMD     string `yaml:"rcmd"`
}

type Condition struct {
    Field string `yaml:"field"`
    Value string `yaml:"value"`
}

type ReplyDef struct {
    Stream   byte `yaml:"stream"`
    Function byte `yaml:"function"`
    Ack      byte `yaml:"ack"`
}

type ActionDef struct {
    DelayMs int    `yaml:"delay_ms"`
    Type    string `yaml:"type"`    // "event" or "mutate"
    CEID    string `yaml:"ceid"`    // for type=event
    Target  string `yaml:"target"`  // for type=mutate (dot-path)
    Value   string `yaml:"value"`   // for type=mutate
}
```

### 4.2 Matching Algorithm

```
func (e *Engine) Match(msg *hsms.Message) *Rule:
    for each rule in e.rules (ordered by Rule.Order):
        if !rule.Enabled → skip
        if rule.Match.Stream != msg.Stream → skip
        if rule.Match.Function != msg.Function → skip
        if rule.Match.RCMD != "" AND extractRCMD(msg) != rule.Match.RCMD → skip
        if !allConditionsMet(rule.Conditions, e.state) → skip
        return rule    // first match wins
    return nil         // no match
```

`extractRCMD` parses the SECS-II body of S2F41: it expects `L:2 <A "cmdname"> L:n ...` and returns the command name string.

### 4.3 Action Scheduling

When a rule matches:

```
1. Build reply frame → send immediately over HSMS
2. For each action in rule.Events:
     time.AfterFunc(action.DelayMs * time.Millisecond, func() {
         switch action.Type:
         case "event":
             build S6F11 frame with CEID → send
         case "mutate":
             state.Set(action.Target, action.Value)
     })
3. Emit events to the bus for each step (for UI consumption)
```

Use `time.AfterFunc` — it runs the callback in its own goroutine, which is fine since both `hsms.Send()` and `state.Set()` are goroutine-safe (behind a `sync.RWMutex`).

---

## 5. State Store (`internal/state/`)

```go
type Store struct {
    mu   sync.RWMutex
    data map[string]any  // flat or nested, accessed via dot-paths
}

func (s *Store) Get(path string) (string, bool)    // "ports.LP01" → "occupied"
func (s *Store) Set(path string, value string)      // "ports.LP01" = "empty"
func (s *Store) Exists(path string) bool            // "carriers.CARR001" → true
func (s *Store) Snapshot() map[string]any            // deep copy for API/UI
func (s *Store) Reset(initial map[string]any)        // reload from config
```

Dot-path resolution: split on `.`, walk the nested `map[string]any` tree. `Set` creates intermediate maps if they don't exist.

---

## 6. Event Bus (`internal/bus/`)

Central pub/sub that bridges backend → WebSocket clients.

```go
type EventType string

const (
    EventMessage     EventType = "message"       // HSMS message logged
    EventStateChange EventType = "state_change"  // state mutated
    EventRuleMatch   EventType = "rule_match"    // rule matched
    EventHSMSStatus  EventType = "hsms_status"   // connection state changed
    EventConfigDirty EventType = "config_dirty"  // unsaved config changes
)

type Event struct {
    Type    EventType       `json:"type"`
    Payload json.RawMessage `json:"payload"`
    Time    time.Time       `json:"time"`
}

type Bus struct {
    subscribers map[chan Event]struct{}
    mu          sync.RWMutex
}

func (b *Bus) Publish(e Event)                  // fan-out to all subscribers
func (b *Bus) Subscribe() chan Event             // returns a new channel
func (b *Bus) Unsubscribe(ch chan Event)         // removes channel
```

Each WebSocket client gets its own `Subscribe()` channel. The WS handler goroutine reads from the channel and writes JSON to the socket.

---

## 7. HTTP API (`internal/api/`)

### 7.1 REST Endpoints

| Method | Path | Request Body | Response | Notes |
|---|---|---|---|---|
| `GET` | `/api/config` | — | Full `SimConfig` as JSON | Current in-memory config |
| `PUT` | `/api/config` | `SimConfig` JSON | Updated config | Validates, applies, marks dirty |
| `PUT` | `/api/config/hsms` | `HsmsConfig` JSON | Updated HSMS section | Partial update |
| `PUT` | `/api/config/rules` | `[]Rule` JSON | Updated rules | Replaces entire rule list |
| `PUT` | `/api/config/rules/:index` | `Rule` JSON | Updated single rule | By position |
| `POST` | `/api/config/rules` | `Rule` JSON | Created rule | Appends to end |
| `DELETE` | `/api/config/rules/:index` | — | 204 | Delete by position |
| `POST` | `/api/config/rules/reorder` | `{ from: int, to: int }` | Reordered list | Drag-drop support |
| `POST` | `/api/config/save` | — | 200 | Write current config to YAML file |
| `POST` | `/api/config/reload` | — | Reloaded config | Re-read YAML from disk, reset state |
| `GET` | `/api/state` | — | State snapshot JSON | Current state store contents |
| `POST` | `/api/sim/start` | — | 200 / error | Start HSMS listener |
| `POST` | `/api/sim/stop` | — | 200 | Stop HSMS listener |
| `GET` | `/api/sim/status` | — | `{ running, hsms_state, ... }` | Current sim status |

### 7.2 WebSocket (`/ws`)

Single WebSocket endpoint. On connect, the server:

1. Sends a `hsms_status` event with current connection state
2. Sends a `state_change` event with full state snapshot
3. Streams all subsequent events as JSON lines

Message format (server → client):

```json
{
  "type": "message",
  "time": "2025-01-15T14:32:05.210Z",
  "payload": {
    "id": 3,
    "timestamp": "14:32:05.210",
    "direction": "IN",
    "stream": 2,
    "function": 41,
    "wbit": true,
    "label": "Remote Command",
    "body_sml": "L:2 <A \"TRANSFER\"> L:2 ...",
    "matched_rule": "accept transfer"
  }
}
```

```json
{
  "type": "state_change",
  "time": "2025-01-15T14:32:06.415Z",
  "payload": {
    "path": "carriers.CARR001.location",
    "old_value": "LP01",
    "new_value": "SHELF_A01"
  }
}
```

The client can also send commands over the WebSocket (optional, v2):
- `{ "action": "clear_log" }` — clear server-side message buffer
- `{ "action": "inject_message", ... }` — manually send a SECS-II message for testing

---

## 8. Web Frontend (`web/`)

React + TypeScript + Vite. Built output goes into `static/` and is embedded into the Go binary via `go:embed`.

### 8.1 Build & Embed

```go
//go:embed static/*
var staticFiles embed.FS

func main() {
    // Serve React app
    http.Handle("/", http.FileServer(http.FS(staticFiles)))
    // API routes...
}
```

During development, run Vite dev server on `:5173` with a proxy to the Go backend on `:8080`. For production, `vite build` → copy to `static/` → `go build`.

### 8.2 Window Layout

Same as original wireframe spec — side-by-side split:

```
┌─────────────────────────────────────────────────────────┐
│ TOOLBAR                                                 │
├──────────────────────┬──────────────────────────────────┤
│   LEFT PANEL (44%)   │   RIGHT PANEL (56%)              │
│   ┌──────────────┐   │   ┌────────────────────────────┐ │
│   │ [Rules]      │   │   │ Message List               │ │
│   │ [State]      │   │   │ (scrolling, auto-tail)     │ │
│   │ [HSMS]       │   │   │                            │ │
│   │              │   │   ├────────────────────────────┤ │
│   │ Tab Content  │   │   │ Detail Pane                │ │
│   │              │   │   │ (Decoded / Raw / Rule)     │ │
│   └──────────────┘   │   └────────────────────────────┘ │
├──────────────────────┴──────────────────────────────────┤
│ STATUS BAR                                              │
└─────────────────────────────────────────────────────────┘
```

The split is resizable via a CSS drag handle. Position is stored in `localStorage` (browser-side persistence is fine for UI preferences).

### 8.3 Component Tree

```
<App>
  <Toolbar
    simStatus={status}        // from WS: running/stopped, hsms state
    onStart, onStop, onReload // call REST endpoints
  />
  <SplitPane defaultLeft="44%">
    <LeftPanel>
      <TabBar active={tab} />
      {tab === "rules"  && <RulesPanel rules={rules} onUpdate={putRules} />}
      {tab === "state"  && <StatePanel state={state} />}
      {tab === "hsms"   && <HsmsConfigPanel config={config.hsms} onUpdate={putHsms} />}
    </LeftPanel>
    <RightPanel>
      <MessageList
        messages={messages}     // accumulated from WS events
        selected={selectedId}
        onSelect={setSelectedId}
      />
      {selectedId && (
        <DetailPane
          message={messages.find(m => m.id === selectedId)}
          onJumpToRule={name => { setTab("rules"); expandRule(name); }}
        />
      )}
    </RightPanel>
  </SplitPane>
  <StatusBar status={status} config={config} dirty={dirty} />
</App>
```

### 8.4 State Management

No external state library needed. Use React hooks:

```
useWebSocket(url)      → { messages[], state{}, hsmsStatus, connected }
useApi(baseUrl)        → { getConfig, putConfig, putRules, postStart, ... }
useState / useReducer  → local UI state (selected tab, expanded rule, etc.)
```

`useWebSocket` handles:
- Auto-reconnect with exponential backoff (1s, 2s, 4s, max 10s)
- Accumulates messages into a capped ring buffer (keep last 10,000 messages in memory)
- Merges `state_change` events into a live state object
- Tracks `hsms_status` for toolbar/status bar

### 8.5 Rules Panel — Visual Rule Editor

Each rule renders as a collapsible `<RuleCard>`. Refer to the wireframe for visual design. Key interaction details:

**Editing flow:**
1. User modifies a field in the rule card (e.g., changes ACK from 0 to 3)
2. Component calls `onUpdate(ruleIndex, updatedRule)` — optimistic local update
3. Parent debounces (300ms) then calls `PUT /api/config/rules/:index`
4. Backend validates, applies to in-memory engine, publishes `config_dirty` event
5. Status bar shows dirty indicator

**Reordering:**
- Drag handle on collapsed card headers
- Use a lightweight drag library (`@dnd-kit/sortable` or similar)
- On drop, call `POST /api/config/rules/reorder` with `{ from, to }`

**Add / Delete / Duplicate:**
- "+ New Rule" button at top → `POST /api/config/rules` with defaults
- "Delete" button → confirmation dialog → `DELETE /api/config/rules/:index`
- "Duplicate" → deep copy locally, then `POST /api/config/rules`

### 8.6 HSMS Config Panel

Form fields that map directly to `config.hsms`. On any field change, debounce 500ms then `PUT /api/config/hsms`. The backend compares old vs new — if `mode`, `ip`, or `port` changed, it returns a response header `X-Restart-Required: true` and the UI shows a toast: "HSMS settings changed. Restart the simulator to apply."

Timers and handshake toggles apply immediately without restart.

### 8.7 State Panel

Read-only, driven entirely by WebSocket `state_change` events. On first WS connect, the client fetches `GET /api/state` for the initial snapshot, then applies incremental updates.

Animate value changes briefly (flash the changed value in accent color for 500ms) so the user can see mutations as they happen.

### 8.8 Message List

Virtual scrolling is important — if the simulator runs for a while, there may be thousands of messages. Use a library like `@tanstack/react-virtual` or a simple windowed list.

**Auto-scroll behavior:**
- Track whether the user is "at the bottom" (within 50px of scroll end)
- If at bottom: auto-scroll on new messages
- If scrolled up: stop auto-scrolling, show a "↓ New messages" pill at the bottom
- Clicking the pill scrolls to bottom and resumes auto-scroll

### 8.9 Detail Pane — SML Syntax Highlighting

The "Decoded" tab renders the SML body with color highlighting. Since this is HTML, use `<span>` elements with inline styles or CSS classes:

| Token | Color | Example |
|---|---|---|
| List header | dim (`#6c7086`) | `L:2` |
| ASCII item | teal (`#94e2d5`) | `<A "TRANSFER">` |
| Binary item | yellow (`#f9e2af`) | `<B 0x00>` |
| Numeric item | blue (`#89b4fa`) | `<U4 1001>` |
| Punctuation | dim | `< > :` |

Write a simple tokenizer that splits the SML string by regex and wraps each token in a styled span. Not a full parser — just enough for visual scanning.

---

## 9. Data Flow

### 9.1 Startup Sequence

```
1. Parse CLI flags (--config path, --port 8080, --open-browser)
2. Load YAML config → SimConfig
3. Initialize StateStore from config.initial_state
4. Compile rules into Engine
5. Start Event Bus
6. Start HTTP server on :8080
7. Start HSMS listener on config.hsms.port (if auto-start enabled)
8. Open browser to http://localhost:8080 (if --open-browser)
9. Block on signal (SIGINT/SIGTERM) → graceful shutdown
```

### 9.2 Inbound Message Processing

```
HSMS socket receives TCP data
  ↓
frame.ReadFrame() → Frame
  ↓
Control message? (SType > 0)
  ├── Select.req → send Select.rsp, update session state
  ├── Linktest.req → send Linktest.rsp
  ├── Separate → close connection
  └── done (publish hsms_status event)
  ↓
Data message (SType == 0)
  ↓
Auto-response check:
  ├── S1F13 + auto_s1f13 → send S1F14, publish message event, done
  ├── S1F1  + auto_s1f1  → send S1F2, publish message event, done
  └── else ↓
  ↓
engine.Match(msg)
  ├── nil → publish message event (unmatched), no reply
  └── rule → execute:
        1. Build reply → hsms.Send() → publish message event (reply)
        2. Schedule actions → time.AfterFunc for each
           ├── event action → build S6F11 → hsms.Send() → publish message event
           └── mutate action → state.Set() → publish state_change event
        3. Publish rule_match event
```

### 9.3 Event Flow (Backend → UI)

```
Any backend event
  ↓
bus.Publish(Event{type, payload, time})
  ↓
Fan-out to all subscriber channels
  ↓
Each WS handler goroutine:
  json.Marshal(event) → websocket.WriteMessage()
  ↓
Browser receives JSON
  ↓
useWebSocket hook dispatches:
  "message"      → append to messages[]
  "state_change" → merge into state{}
  "hsms_status"  → update status
  "rule_match"   → update matched_rule on existing message
  "config_dirty" → update dirty flag
```

---

## 10. Threading / Goroutine Model

```
main goroutine
  ├── HTTP server (net/http serves each request in its own goroutine)
  └── blocks on os.Signal

HSMS listener goroutine
  ├── net.Listen → Accept loop
  └── per-connection goroutine:
        ├── read loop: ReadFrame → process → send reply
        └── write: via channel (serialized writes to avoid interleaving)

Scheduler goroutines
  └── time.AfterFunc spawns a goroutine per delayed action

WebSocket goroutines (one per connected browser tab)
  ├── read loop: client→server commands (v2)
  └── write loop: reads from bus subscriber channel → writes to WS

Event Bus
  └── Publish() iterates subscribers under RLock, sends to channels
      (use buffered channels with a small buffer, drop if full to avoid blocking)
```

**Concurrency safety:**
- `StateStore`: protected by `sync.RWMutex` — reads use `RLock`, writes use `Lock`
- `Engine.rules`: replace atomically via `atomic.Value` or `sync.RWMutex` on config update
- HSMS write path: serialize through a write channel per connection to prevent frame interleaving
- Bus subscriber channels: buffered (cap 256). If a slow WS client can't keep up, drop events rather than blocking the publisher

---

## 11. Config File Format

The YAML file that the UI edits and the backend loads:

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
        type: mutate
        target: ports.LP01
        value: empty
      - delay_ms: 1200
        type: event
        ceid: TRANSFER_COMPLETED
```

---

## 12. CLI Interface

```
secsim [flags]

Flags:
  --config string    Path to YAML config file (default: "config.yaml")
  --port int         HTTP server port (default: 8080)
  --open             Auto-open browser on startup
  --no-autostart     Don't start HSMS listener automatically
  --verbose          Enable debug logging for HSMS frames
```

---

## 13. Color Scheme

Same as wireframe. Defined as CSS custom properties in the React app and as a Go-side constant for log coloring (if terminal output is colored).

| Token | Hex | Usage |
|---|---|---|
| `--bg` | `#1e1e2e` | Window background |
| `--surface` | `#252536` | Toolbar, status bar |
| `--surface-alt` | `#2a2a3d` | Section headers |
| `--border` | `#353548` | Separators |
| `--border-active` | `#6c6cff` | Focus rings, selection |
| `--text` | `#cdd6f4` | Default text |
| `--text-dim` | `#6c7086` | Labels, hints |
| `--text-bright` | `#f0f0ff` | Emphasized text |
| `--accent` | `#6c6cff` | Primary accent |
| `--green` | `#a6e3a1` | Connected, IN direction |
| `--red` | `#f38ba8` | Error, blocked, delete |
| `--yellow` | `#f9e2af` | SxFy labels, events |
| `--blue` | `#89b4fa` | OUT direction, occupied |
| `--teal` | `#94e2d5` | SML values, carrier IDs |
| `--orange` | `#fab387` | Mutate actions |

---

## 14. Go Dependencies

Minimal dependency set:

| Package | Purpose |
|---|---|
| `gopkg.in/yaml.v3` | YAML config parse/write |
| `nhooyr.io/websocket` | WebSocket (lighter than gorilla, stdlib-friendly) |
| Standard library only | `net`, `encoding/binary`, `sync`, `time`, `embed`, `net/http`, `encoding/json` |

No web framework — `net/http` + a small router (or `http.ServeMux` from Go 1.22+ with path params) is sufficient for this API surface.

---

## 15. Implementation Order

Each step produces a testable increment:

1. **HSMS framing** — `ReadFrame`/`WriteFrame` + unit tests with raw bytes. Test with a simple TCP client sending hand-crafted frames.

2. **SECS-II item codec** — `EncodeItem`/`DecodeItem` for List, ASCII, Binary, U4. Unit test round-trip encoding.

3. **HSMS session** — TCP listener, Select/Deselect state machine, Linktest. Test by connecting from a SECS/GEM host or a Python script using `secsgem`.

4. **Config loader** — Parse YAML into `SimConfig`. Validate rules. Unit test with sample configs.

5. **State store** — Dot-path get/set/exists with mutex. Unit test concurrent access.

6. **Rule engine** — Match + execute. Test by feeding mock messages and asserting replies + state changes.

7. **Event bus + WebSocket** — Publish events, stream to WS clients. Test with `wscat` or a browser console.

8. **REST API** — Config CRUD endpoints. Test with `curl`.

9. **React shell** — Toolbar + split layout + WebSocket hook. Connect to running Go backend.

10. **Message monitor** — Message list + detail pane. This is the first "useful" UI milestone.

11. **HSMS config panel** — Form fields + REST integration.

12. **State panel** — Live state view from WS events.

13. **Rule editor** — Visual card editor + REST integration. Most complex UI component.

14. **Polish** — Keyboard shortcuts, drag reorder, SML syntax highlighting, virtual scrolling, auto-scroll behavior.
