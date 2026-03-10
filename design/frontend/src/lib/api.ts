import type {
  DeviceConfig,
  HsmsConfig,
  Rule,
  Snapshot,
} from "../types";
import { normalizeSnapshot } from "./normalizeSnapshot";

const API_BASE = import.meta.env.VITE_API_BASE ?? "";

function apiPath(path: string): string {
  return `${API_BASE}${path}`;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(apiPath(path), {
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
    ...init,
  });

  if (!response.ok) {
    let message = `Request failed with status ${response.status}`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload.error) {
        message = payload.error;
      }
    } catch {
      // Ignore JSON parse failures on error responses.
    }
    throw new Error(message);
  }

  return (await response.json()) as T;
}

async function requestSnapshot(path: string, init?: RequestInit): Promise<Snapshot> {
  return normalizeSnapshot(await request<Snapshot>(path, init));
}

export const api = {
  eventsUrl: () => apiPath("/api/events"),
  bootstrap: () => requestSnapshot("/api/bootstrap"),
  toggleRuntime: () => requestSnapshot("/api/runtime/toggle", { method: "POST" }),
  saveConfig: () => requestSnapshot("/api/config/save", { method: "POST" }),
  reloadConfig: () => requestSnapshot("/api/config/reload", { method: "POST" }),
  clearLog: () => requestSnapshot("/api/log/clear", { method: "POST" }),
  updateHsms: (config: HsmsConfig) =>
    requestSnapshot("/api/hsms", {
      method: "PUT",
      body: JSON.stringify(config),
    }),
  updateDevice: (device: DeviceConfig) =>
    requestSnapshot("/api/device", {
      method: "PUT",
      body: JSON.stringify(device),
    }),
  createRule: () => requestSnapshot("/api/rules", { method: "POST" }),
  updateRule: (rule: Rule) =>
    requestSnapshot(`/api/rules/${rule.id}`, {
      method: "PUT",
      body: JSON.stringify(rule),
    }),
  duplicateRule: (id: string) =>
    requestSnapshot(`/api/rules/${id}/duplicate`, {
      method: "POST",
    }),
  deleteRule: (id: string) =>
    requestSnapshot(`/api/rules/${id}`, {
      method: "DELETE",
    }),
  moveRule: (id: string, direction: "up" | "down") =>
    requestSnapshot(`/api/rules/${id}/move`, {
      method: "POST",
      body: JSON.stringify({ direction }),
    }),
};
