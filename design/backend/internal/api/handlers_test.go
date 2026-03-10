package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"secsim/design/backend/internal/model"
	"secsim/design/backend/internal/sim"
	"secsim/design/backend/internal/store"
)

func newTestMux() *http.ServeMux {
	return newTestMuxWithStore(store.New())
}

func newTestMuxWithStore(state *store.Store) *http.ServeMux {
	mux := http.NewServeMux()
	Register(mux, state, sim.New(state))
	return mux
}

func doRequest(t *testing.T, mux *http.ServeMux, method string, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	return recorder
}

func decodeSnapshot(t *testing.T, recorder *httptest.ResponseRecorder) model.Snapshot {
	t.Helper()

	var snapshot model.Snapshot
	if err := json.Unmarshal(recorder.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	return snapshot
}

func decodeMap(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode map: %v", err)
	}

	return payload
}

func decodeSSESnapshot(t *testing.T, reader *bufio.Reader) model.Snapshot {
	t.Helper()

	type result struct {
		snapshot model.Snapshot
		err      error
	}

	done := make(chan result, 1)
	go func() {
		var data strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				done <- result{err: err}
				return
			}

			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				if data.Len() == 0 {
					continue
				}

				var snapshot model.Snapshot
				done <- result{
					snapshot: snapshot,
					err:      json.Unmarshal([]byte(data.String()), &snapshot),
				}
				return
			}
			if strings.HasPrefix(line, "data: ") {
				data.WriteString(strings.TrimPrefix(line, "data: "))
			}
		}
	}()

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("decode SSE snapshot: %v", result.err)
		}
		return result.snapshot
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE snapshot")
		return model.Snapshot{}
	}
}

func TestHealthEndpointReturnsJSONAndCORSHeaders(t *testing.T) {
	mux := newTestMux()

	recorder := doRequest(t, mux, http.MethodGet, "/api/health", nil)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected wildcard CORS header, got %q", got)
	}

	if !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("expected health response body, got %s", recorder.Body.String())
	}
}

func TestOptionsRequestReturnsNoContent(t *testing.T) {
	mux := newTestMux()

	recorder := doRequest(t, mux, http.MethodOptions, "/api/health", nil)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for preflight, got %d", recorder.Code)
	}
}

func TestEventsEndpointStreamsInitialAndUpdatedSnapshots(t *testing.T) {
	state := store.New()
	mux := newTestMuxWithStore(state)
	server := httptest.NewServer(mux)
	defer server.Close()

	response, err := http.Get(server.URL + "/api/events")
	if err != nil {
		t.Fatalf("open events stream: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from events stream, got %d", response.StatusCode)
	}
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got %q", got)
	}

	reader := bufio.NewReader(response.Body)
	initial := decodeSSESnapshot(t, reader)
	if len(initial.Messages) == 0 {
		t.Fatalf("expected initial stream snapshot to include seeded messages")
	}

	state.ClearLog()
	updated := decodeSSESnapshot(t, reader)
	if len(updated.Messages) != 0 {
		t.Fatalf("expected updated stream snapshot after log clear, got %d messages", len(updated.Messages))
	}
}

func TestBootstrapSerializesEmptyCollectionsAsArrays(t *testing.T) {
	mux := newTestMux()

	recorder := doRequest(t, mux, http.MethodGet, "/api/bootstrap", nil)
	payload := decodeMap(t, recorder)

	rules := payload["rules"].([]any)
	secondRule := rules[1].(map[string]any)
	thirdRule := rules[2].(map[string]any)

	if secondRule["actions"] == nil {
		t.Fatalf("expected second rule actions to serialize as [] not null")
	}
	if thirdRule["conditions"] == nil {
		t.Fatalf("expected third rule conditions to serialize as [] not null")
	}
	if payload["messages"] == nil {
		t.Fatalf("expected messages to serialize as [] or populated array, not null")
	}
}

func TestRuleLifecycleEndpoints(t *testing.T) {
	mux := newTestMux()

	createRecorder := doRequest(t, mux, http.MethodPost, "/api/rules", nil)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d", createRecorder.Code)
	}

	createdSnapshot := decodeSnapshot(t, createRecorder)
	newRule := createdSnapshot.Rules[len(createdSnapshot.Rules)-1]
	if newRule.Name != "new rule" {
		t.Fatalf("expected default rule name, got %q", newRule.Name)
	}

	newRule.Name = ""
	newRule.Actions = []model.RuleAction{
		{ID: "action-z", DelayMS: 800, Type: "event", CEID: "LATE"},
		{ID: "action-a", DelayMS: 100, Type: "event", CEID: "EARLY"},
	}
	updateRecorder := doRequest(t, mux, http.MethodPut, "/api/rules/"+newRule.ID, newRule)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from update, got %d", updateRecorder.Code)
	}

	updatedSnapshot := decodeSnapshot(t, updateRecorder)
	updatedRule := updatedSnapshot.Rules[len(updatedSnapshot.Rules)-1]
	if updatedRule.Name != "unnamed rule" {
		t.Fatalf("expected defaulted blank name, got %q", updatedRule.Name)
	}
	if updatedRule.Actions[0].ID != "action-a" {
		t.Fatalf("expected actions to be sorted by delay, got %#v", updatedRule.Actions)
	}

	duplicateRecorder := doRequest(t, mux, http.MethodPost, "/api/rules/"+newRule.ID+"/duplicate", nil)
	if duplicateRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from duplicate, got %d", duplicateRecorder.Code)
	}

	duplicateSnapshot := decodeSnapshot(t, duplicateRecorder)
	duplicateRule := duplicateSnapshot.Rules[len(duplicateSnapshot.Rules)-1]
	if duplicateRule.Name != "unnamed rule (copy)" {
		t.Fatalf("expected duplicate rule name suffix, got %q", duplicateRule.Name)
	}

	deleteRecorder := doRequest(t, mux, http.MethodDelete, "/api/rules/"+duplicateRule.ID, nil)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from delete, got %d", deleteRecorder.Code)
	}

	deletedSnapshot := decodeSnapshot(t, deleteRecorder)
	if len(deletedSnapshot.Rules) != 4 {
		t.Fatalf("expected delete to restore rule count to 4, got %d", len(deletedSnapshot.Rules))
	}
}

func TestMoveRuleRejectsUnsupportedDirection(t *testing.T) {
	mux := newTestMux()

	recorder := doRequest(t, mux, http.MethodPost, "/api/rules/rule-1/move", map[string]string{
		"direction": "sideways",
	})

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported move, got %d", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), "unsupported move direction") {
		t.Fatalf("expected move error message, got %s", recorder.Body.String())
	}
}

func TestUpdateHSMSRejectsUnknownFields(t *testing.T) {
	mux := newTestMux()

	req := httptest.NewRequest(http.MethodPut, "/api/hsms", strings.NewReader(`{"mode":"passive","unknown":true}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid payload, got %d", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), "unknown field") {
		t.Fatalf("expected unknown field error, got %s", recorder.Body.String())
	}
}

func TestDeleteRuleReturnsNotFoundForMissingID(t *testing.T) {
	mux := newTestMux()

	recorder := doRequest(t, mux, http.MethodDelete, "/api/rules/missing", nil)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing rule, got %d", recorder.Code)
	}
}

func TestReloadEndpointReturnsValidationErrorsForInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stocker-sim.yaml")
	fileStore, err := store.NewFromFile(path)
	if err != nil {
		t.Fatalf("create file-backed store: %v", err)
	}

	if _, err := fileStore.Save(); err != nil {
		t.Fatalf("seed config file: %v", err)
	}
	if err := os.WriteFile(path, []byte("rules:\n  - name: broken\n    events: ["), 0o644); err != nil {
		t.Fatalf("write invalid YAML: %v", err)
	}

	mux := newTestMuxWithStore(fileStore)
	recorder := doRequest(t, mux, http.MethodPost, "/api/config/reload", nil)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid YAML reload, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "invalid config") {
		t.Fatalf("expected invalid config error body, got %s", recorder.Body.String())
	}
}

func TestSimLifecycleAndInjectEndpoints(t *testing.T) {
	mux := newTestMux()

	statusBefore := doRequest(t, mux, http.MethodGet, "/api/sim/status", nil)
	if statusBefore.Code != http.StatusOK {
		t.Fatalf("expected 200 from status, got %d", statusBefore.Code)
	}
	if decodeMap(t, statusBefore)["running"] != false {
		t.Fatalf("expected simulator to start stopped")
	}

	startRecorder := doRequest(t, mux, http.MethodPost, "/api/sim/start", nil)
	if startRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from start, got %d", startRecorder.Code)
	}
	started := decodeSnapshot(t, startRecorder)
	if !started.Runtime.Listening || started.Runtime.HSMSState != "NOT CONNECTED" {
		t.Fatalf("expected simulator to start in listening/not-connected state, got %#v", started.Runtime)
	}

	injectRecorder := doRequest(t, mux, http.MethodPost, "/api/sim/inject", map[string]any{
		"stream":   2,
		"function": 41,
		"wbit":     true,
		"rcmd":     "TRANSFER",
		"fields": map[string]string{
			"SourcePort": "LP01",
		},
	})
	if injectRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from inject, got %d: %s", injectRecorder.Code, injectRecorder.Body.String())
	}
	injected := decodeSnapshot(t, injectRecorder)
	if len(injected.Messages) != 9 {
		t.Fatalf("expected inject to append inbound + reply messages, got %d", len(injected.Messages))
	}
	if injected.Messages[len(injected.Messages)-2].MatchedRuleID != "rule-1" {
		t.Fatalf("expected injected inbound message to match rule-1, got %#v", injected.Messages[len(injected.Messages)-2])
	}

	stopRecorder := doRequest(t, mux, http.MethodPost, "/api/sim/stop", nil)
	if stopRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from stop, got %d", stopRecorder.Code)
	}
	stopped := decodeSnapshot(t, stopRecorder)
	if stopped.Runtime.Listening {
		t.Fatalf("expected simulator to stop, got %#v", stopped.Runtime)
	}

	injectWhileStopped := doRequest(t, mux, http.MethodPost, "/api/sim/inject", map[string]any{
		"stream":   2,
		"function": 41,
	})
	if injectWhileStopped.Code != http.StatusConflict {
		t.Fatalf("expected 409 when injecting while stopped, got %d", injectWhileStopped.Code)
	}
}
