package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"secsim/design/backend/internal/model"
	"secsim/design/backend/internal/store"
)

func newTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	Register(mux, store.New())
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
