import { describe, expect, it, vi, beforeEach } from "vitest";
import { api } from "./api";
import { makeSnapshot } from "../test/fixtures";

describe("api client", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  it("requests bootstrap state from the API", async () => {
    const snapshot = makeSnapshot();
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify(snapshot), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const result = await api.bootstrap();

    expect(result).toEqual(snapshot);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/bootstrap",
      expect.objectContaining({
        headers: { "Content-Type": "application/json" },
      }),
    );
  });

  it("builds the live events URL from the API base", () => {
    expect(api.eventsUrl()).toBe("/api/events");
  });

  it("sends JSON bodies for rule updates", async () => {
    const snapshot = makeSnapshot();
    const rule = snapshot.rules[0];
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify(snapshot), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await api.updateRule(rule);

    expect(fetchMock).toHaveBeenCalledWith(
      `/api/rules/${rule.id}`,
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify(rule),
      }),
    );
  });

  it("surfaces API error messages from JSON responses", async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ error: "bad request" }), {
        status: 400,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(api.saveConfig()).rejects.toThrow("bad request");
  });

  it("normalizes null collections in snapshot responses", async () => {
    const rule = makeSnapshot().rules[0];
    rule.actions = [
      {
        ...rule.actions[0],
        reports: [
          {
            rptid: "5001",
            values: null as never,
          },
        ],
      },
    ];
    const malformed = {
      ...makeSnapshot(),
      rules: [
        {
          ...rule,
          conditions: null,
        },
        {
          ...makeSnapshot().rules[1],
          conditions: [],
          actions: null,
        },
      ],
      messages: null,
    };
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify(malformed), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const result = await api.bootstrap();

    expect(result.rules[0].conditions).toEqual([]);
    expect(result.rules[0].actions[0].reports?.[0].values).toEqual([]);
    expect(result.rules[1].actions).toEqual([]);
    expect(result.messages).toEqual([]);
  });
});
