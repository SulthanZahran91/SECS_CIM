package sim

import (
	"context"
	"errors"
	"sync"
	"time"

	"secsim/design/backend/internal/model"
	"secsim/design/backend/internal/store"
)

var ErrNotRunning = errors.New("simulator is not running")

type Controller struct {
	store  *store.Store
	mu     sync.Mutex
	cancel context.CancelFunc
}

type Status struct {
	Running      bool   `json:"running"`
	HSMSState    string `json:"hsmsState"`
	MessageCount int    `json:"messageCount"`
	ConfigFile   string `json:"configFile"`
	Dirty        bool   `json:"dirty"`
}

func New(state *store.Store) *Controller {
	return &Controller{store: state}
}

func (c *Controller) Start() (model.Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		return c.store.Snapshot(), nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go c.schedulerLoop(ctx)

	return c.store.SetRuntime(true, "NOT CONNECTED"), nil
}

func (c *Controller) Stop() model.Snapshot {
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	return c.store.SetRuntime(false, "NOT CONNECTED")
}

func (c *Controller) Toggle() (model.Snapshot, error) {
	c.mu.Lock()
	running := c.cancel != nil
	c.mu.Unlock()

	if running {
		return c.Stop(), nil
	}

	return c.Start()
}

func (c *Controller) Status() Status {
	snapshot := c.store.Snapshot()

	c.mu.Lock()
	running := c.cancel != nil
	c.mu.Unlock()

	return Status{
		Running:      running,
		HSMSState:    snapshot.Runtime.HSMSState,
		MessageCount: len(snapshot.Messages),
		ConfigFile:   snapshot.Runtime.ConfigFile,
		Dirty:        snapshot.Runtime.Dirty,
	}
}

func (c *Controller) Inject(message store.InboundMessage) (store.RuntimeResult, error) {
	c.mu.Lock()
	running := c.cancel != nil
	c.mu.Unlock()

	if !running {
		return store.RuntimeResult{}, ErrNotRunning
	}

	now := time.Now().UTC()
	if message.Timestamp.IsZero() {
		message.Timestamp = now
	}

	return c.store.ProcessInbound(message, now), nil
}

func (c *Controller) schedulerLoop(ctx context.Context) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			_, _ = c.store.RunScheduled(now)
		}
	}
}
