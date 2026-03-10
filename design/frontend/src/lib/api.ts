import type {
  DeviceConfig,
  HsmsConfig,
  Rule,
  Snapshot,
} from "../types";

const API_BASE = import.meta.env.VITE_API_BASE ?? "";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
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

export const api = {
  bootstrap: () => request<Snapshot>("/api/bootstrap"),
  toggleRuntime: () => request<Snapshot>("/api/runtime/toggle", { method: "POST" }),
  saveConfig: () => request<Snapshot>("/api/config/save", { method: "POST" }),
  reloadConfig: () => request<Snapshot>("/api/config/reload", { method: "POST" }),
  clearLog: () => request<Snapshot>("/api/log/clear", { method: "POST" }),
  updateHsms: (config: HsmsConfig) =>
    request<Snapshot>("/api/hsms", {
      method: "PUT",
      body: JSON.stringify(config),
    }),
  updateDevice: (device: DeviceConfig) =>
    request<Snapshot>("/api/device", {
      method: "PUT",
      body: JSON.stringify(device),
    }),
  createRule: () => request<Snapshot>("/api/rules", { method: "POST" }),
  updateRule: (rule: Rule) =>
    request<Snapshot>(`/api/rules/${rule.id}`, {
      method: "PUT",
      body: JSON.stringify(rule),
    }),
  duplicateRule: (id: string) =>
    request<Snapshot>(`/api/rules/${id}/duplicate`, {
      method: "POST",
    }),
  deleteRule: (id: string) =>
    request<Snapshot>(`/api/rules/${id}`, {
      method: "DELETE",
    }),
  moveRule: (id: string, direction: "up" | "down") =>
    request<Snapshot>(`/api/rules/${id}/move`, {
      method: "POST",
      body: JSON.stringify({ direction }),
    }),
};

