#!/usr/bin/env node

import http from "node:http";

const baseURL = (process.env.BIFROST_E2E_BASE_URL || process.env.BIFROST_BASE_URL || "http://localhost:8080").replace(/\/+$/, "");
const adminAuthHeader = process.env.BIFROST_E2E_AUTH_HEADER || "";
const runID = `${process.pid}-${Date.now()}`;
const modelName = "cache-order-model";
const providers = {
  openai: `midconv-openai-${runID}`,
  anthropic: `midconv-anthropic-${runID}`,
};
const captured = new Map();

function recordCapture(path, body) {
  const entries = captured.get(path) || [];
  entries.push(body);
  captured.set(path, entries);
}

function listen(server, host = "127.0.0.1") {
  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, host, () => {
      server.off("error", reject);
      resolve(server.address().port);
    });
  });
}

function close(server) {
  return new Promise((resolve) => server.close(() => resolve()));
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    req.on("data", (chunk) => chunks.push(chunk));
    req.on("end", () => resolve(Buffer.concat(chunks)));
    req.on("error", reject);
  });
}

function createCaptureUpstream() {
  return http.createServer(async (req, res) => {
    const body = JSON.parse((await readBody(req)).toString("utf8"));
    recordCapture(req.url, body);

    if (req.url === "/openai/v1/chat/completions") {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(JSON.stringify({
        id: `chatcmpl-${runID}`,
        object: "chat.completion",
        created: Math.floor(Date.now() / 1000),
        model: modelName,
        choices: [{ index: 0, message: { role: "assistant", content: "ok" }, finish_reason: "stop" }],
        usage: { prompt_tokens: 4, completion_tokens: 1, total_tokens: 5 },
      }));
      return;
    }
    if (req.url === "/openai/v1/responses") {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(JSON.stringify({
        id: `resp-${runID}`,
        object: "response",
        created_at: Math.floor(Date.now() / 1000),
        status: "completed",
        model: modelName,
        output: [{ id: `msg-${runID}`, type: "message", status: "completed", role: "assistant", content: [{ type: "output_text", text: "ok", annotations: [] }] }],
        usage: { input_tokens: 4, output_tokens: 1, total_tokens: 5 },
      }));
      return;
    }
    if (req.url === "/anthropic/v1/messages") {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(JSON.stringify({
        id: `msg-${runID}`,
        type: "message",
        role: "assistant",
        content: [{ type: "text", text: "ok" }],
        model: modelName,
        stop_reason: "end_turn",
        stop_sequence: null,
        usage: { input_tokens: 4, output_tokens: 1 },
      }));
      return;
    }
    res.writeHead(404);
    res.end("not found");
  });
}

async function request(method, path, body) {
  const headers = adminAuthHeader ? { Authorization: adminAuthHeader } : {};
  if (body !== undefined) headers["content-type"] = "application/json";
  const response = await fetch(`${baseURL}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await response.text();
  let json;
  try { json = text ? JSON.parse(text) : null; } catch { json = null; }
  return { ok: response.ok, status: response.status, text, json };
}

async function mustRequest(method, path, body) {
  const response = await request(method, path, body);
  if (!response.ok) throw new Error(`${method} ${path} failed with ${response.status}: ${response.text}`);
  return response;
}

async function addProvider(name, baseProviderType, port, basePath) {
  await mustRequest("POST", "/api/providers", {
    provider: name,
    custom_provider_config: {
      base_provider_type: baseProviderType,
      is_key_less: true,
      preserve_mid_conversation_system_messages: true,
      allowed_requests: {
        chat_completion: true,
        chat_completion_stream: true,
        responses: true,
        responses_stream: true,
      },
      request_path_overrides: baseProviderType === "anthropic" ? {
        chat_completion: `http://127.0.0.1:${port}/anthropic/v1/messages`,
        responses: `http://127.0.0.1:${port}/anthropic/v1/messages`,
      } : {
        chat_completion: `http://127.0.0.1:${port}/openai/v1/chat/completions`,
        responses: `http://127.0.0.1:${port}/openai/v1/responses`,
      },
    },
    network_config: {
      base_url: `http://127.0.0.1:${port}${basePath}`,
      allow_private_network: true,
      default_request_timeout_in_seconds: 10,
      max_retries: 0,
      retry_backoff_initial: 500,
      retry_backoff_max: 5000,
    },
    concurrency_and_buffer_size: { concurrency: 4, buffer_size: 16 },
    keys: [],
  });
}

const chatMessages = [
  { role: "system", content: "initial-system" },
  { role: "user", content: "hello" },
  { role: "assistant", content: "hi" },
  { role: "system", content: [{ type: "text", text: "later-system", cache_control: { type: "ephemeral" } }] },
  { role: "user", content: "continue" },
];

const responsesInput = chatMessages.map((message) => ({ type: "message", ...message }));

function requireCaptured(path, index = 0) {
  const body = captured.get(path)?.[index];
  if (!body) throw new Error(`capture upstream did not receive ${path} request ${index + 1}`);
  return body;
}

function assertOpenAI(path) {
  const body = requireCaptured(path);
  const messages = body.messages || body.input;
  const roles = messages.map((message) => message.role);
  const expected = ["system", "user", "assistant", "developer", "user"];
  if (JSON.stringify(roles) !== JSON.stringify(expected)) {
    throw new Error(`${path} roles=${JSON.stringify(roles)}, want ${JSON.stringify(expected)}`);
  }
}

function systemText(system) {
  if (typeof system === "string") return system;
  return (system || []).map((block) => block.text || "").join("\n");
}

function assertAnthropic(index) {
  const body = requireCaptured("/anthropic/v1/messages", index);
  const topLevel = systemText(body.system);
  if (!topLevel.includes("initial-system") || topLevel.includes("later-system")) {
    throw new Error(`Anthropic top-level system changed order: ${JSON.stringify(body.system)}`);
  }
  const hit = body.messages.find((message) => JSON.stringify(message.content).includes("later-system"));
  if (!hit || hit.role !== "user" || !JSON.stringify(hit.content).includes("system-reminder")) {
    throw new Error(`Anthropic later system was not an in-place wrapped user message: ${JSON.stringify(body.messages)}`);
  }
  const blocks = Array.isArray(hit.content) ? hit.content : [];
  if (!blocks.some((block) => block.cache_control?.type === "ephemeral")) {
    throw new Error(`Anthropic wrapped user message lost cache_control: ${JSON.stringify(hit)}`);
  }
}

async function main() {
  console.log("Running local mid-conversation system preservation check...");
  const upstream = createCaptureUpstream();
  const port = await listen(upstream);
  try {
    await addProvider(providers.openai, "openai", port, "/openai");
    await addProvider(providers.anthropic, "anthropic", port, "");

    await mustRequest("POST", "/v1/chat/completions", { model: `${providers.openai}/${modelName}`, messages: chatMessages });
    await mustRequest("POST", "/v1/responses", { model: `${providers.openai}/${modelName}`, input: responsesInput });
    assertOpenAI("/openai/v1/chat/completions");
    assertOpenAI("/openai/v1/responses");

    await mustRequest("POST", "/v1/chat/completions", { model: `${providers.anthropic}/${modelName}`, messages: chatMessages, max_tokens: 64 });
    assertAnthropic(0);
    await mustRequest("POST", "/v1/responses", { model: `${providers.anthropic}/${modelName}`, input: responsesInput, max_output_tokens: 64 });
    assertAnthropic(1);

    console.log("Local mid-conversation system preservation check passed (4 wire paths).");
  } finally {
    await Promise.all([
      request("DELETE", `/api/providers/${encodeURIComponent(providers.openai)}`),
      request("DELETE", `/api/providers/${encodeURIComponent(providers.anthropic)}`),
    ]);
    await close(upstream);
  }
}

main().catch((error) => {
  console.error(`Local mid-conversation system preservation check failed: ${error.message}`);
  process.exit(1);
});
