package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"secsim/design/backend/internal/model"
	"secsim/design/backend/internal/store"
)

type Handler struct {
	store *store.Store
}

type moveRuleRequest struct {
	Direction string `json:"direction"`
}

func Register(mux *http.ServeMux, state *store.Store) {
	handler := &Handler{store: state}

	mux.Handle("/api/health", withCORS(http.HandlerFunc(handler.health)))
	mux.Handle("/api/bootstrap", withCORS(http.HandlerFunc(handler.bootstrap)))
	mux.Handle("/api/runtime/toggle", withCORS(http.HandlerFunc(handler.toggleRuntime)))
	mux.Handle("/api/config/save", withCORS(http.HandlerFunc(handler.saveConfig)))
	mux.Handle("/api/config/reload", withCORS(http.HandlerFunc(handler.reloadConfig)))
	mux.Handle("/api/log/clear", withCORS(http.HandlerFunc(handler.clearLog)))
	mux.Handle("/api/hsms", withCORS(http.HandlerFunc(handler.updateHSMS)))
	mux.Handle("/api/device", withCORS(http.HandlerFunc(handler.updateDevice)))
	mux.Handle("/api/rules", withCORS(http.HandlerFunc(handler.rules)))
	mux.Handle("/api/rules/", withCORS(http.HandlerFunc(handler.ruleByID)))
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
	})
}

func (h *Handler) bootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, h.store.Snapshot())
}

func (h *Handler) toggleRuntime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, h.store.ToggleRuntime())
}

func (h *Handler) saveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, h.store.Save())
}

func (h *Handler) reloadConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, h.store.Reload())
}

func (h *Handler) clearLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, h.store.ClearLog())
}

func (h *Handler) updateHSMS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload model.HsmsConfig
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, h.store.UpdateHSMS(payload))
}

func (h *Handler) updateDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload model.DeviceConfig
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, h.store.UpdateDevice(payload))
}

func (h *Handler) rules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		writeJSON(w, http.StatusCreated, h.store.NewRule())
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) ruleByID(w http.ResponseWriter, r *http.Request) {
	segments := pathSegments("/api/rules/", r.URL.Path)
	if len(segments) == 0 {
		writeError(w, http.StatusNotFound, "rule endpoint not found")
		return
	}

	id := segments[0]
	if len(segments) == 1 {
		switch r.Method {
		case http.MethodPut:
			var payload model.Rule
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			payload.ID = id
			snapshot, err := h.store.UpdateRule(payload)
			if err != nil {
				h.writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, snapshot)
		case http.MethodDelete:
			snapshot, err := h.store.DeleteRule(id)
			if err != nil {
				h.writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, snapshot)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	switch segments[1] {
	case "duplicate":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		snapshot, err := h.store.DuplicateRule(id)
		if err != nil {
			h.writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	case "move":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var payload moveRuleRequest
		if err := decodeJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		snapshot, err := h.store.MoveRule(id, payload.Direction)
		if err != nil {
			h.writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	default:
		writeError(w, http.StatusNotFound, "rule action not found")
	}
}

func (h *Handler) writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrRuleNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeError(w, http.StatusBadRequest, err.Error())
}

func decodeJSON(r *http.Request, destination any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	return decoder.Decode(destination)
}

func pathSegments(prefix string, path string) []string {
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" {
		return nil
	}

	return strings.Split(trimmed, "/")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
