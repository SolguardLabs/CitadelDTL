import assert from "node:assert/strict";
import test from "node:test";
import {
  CitadelClient,
  CitadelHTTPError,
  canonicalJSON,
  canonicalMandate,
  computeCapitalMetrics,
  payloadHash,
} from "../../sdk/citadelClient.ts";

test("capital metrics preserve integer rounding parity", () => {
  const metrics = computeCapitalMetrics({
    custodyLiability: 1_000n,
    reserveAssets: 900n,
    pendingOutflows: 100n,
    liquidReserve: 50n,
    reserveHaircutPPM: 100_000n,
    outflowShockPPM: 500_000n,
    operationalCostPPM: 10_000n,
  });
  assert.deepEqual(metrics, {
    effectiveReserve: 810n,
    stressedOutflows: 150n,
    operationalBuffer: 10n,
    requiredCapital: 1_160n,
    capitalDeficit: 350n,
    coveragePPM: 698_275n,
    liquidityPPM: 333_333n,
  });
});

test("canonical mandate emits decimal strings and sorted JSON", () => {
  const mandate = canonicalMandate({
    id: "mandate-a",
    parentAccount: "treasury",
    delegateAccount: "delegate-a",
    asset: "USDC",
    dailyLimit: 250_000n,
    withdrawalLimit: 100_000n,
    settlementLimit: 125_000n,
    directWithdrawal: false,
    allowInternalTransfers: true,
    allowOperationalSubaccounts: true,
    label: "Operaciones institucionales",
  });
  const encoded = canonicalJSON(mandate);
  assert.match(encoded, /"daily_limit":"250000"/);
  assert.equal(
    encoded.indexOf("allow_internal_transfers") < encoded.indexOf("asset"),
    true,
  );
  assert.equal(payloadHash(mandate).length, 64);
});

test("client enforces transport policy", () => {
  assert.throws(
    () => new CitadelClient("http://custody.example"),
    /requires HTTPS/,
  );
  assert.throws(
    () => new CitadelClient("https://user:secret@custody.example"),
    /cannot include credentials/,
  );
  assert.doesNotThrow(
    () =>
      new CitadelClient("http://127.0.0.1:8080", {
        allowInsecureLocalhost: true,
      }),
  );
});

test("client sends deterministic safety headers", async () => {
  let observed: { url: string; init: RequestInit } | undefined;
  const fetchImpl: typeof fetch = async (input, init = {}) => {
    observed = { url: String(input), init };
    return new Response(JSON.stringify({ accepted: true }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  };
  const client = new CitadelClient("https://custody.example/api", {
    fetchImpl,
  });
  const response = await client.transfer<{ accepted: boolean }>(
    {
      from: "delegate-a",
      to: "settlement",
      asset: "USDC",
      amount: 10n,
      reference: "move-001",
    },
    { idempotencyKey: "idem-001" },
  );
  assert.equal(response.accepted, true);
  assert.equal(observed?.url, "https://custody.example/api/v1/transfers");
  assert.equal(
    new Headers(observed?.init.headers).get("idempotency-key"),
    "idem-001",
  );
  assert.equal(observed?.init.redirect, "error");
  assert.equal(observed?.init.credentials, "omit");
});

test("client rejects non-JSON and oversized responses", async () => {
  const textClient = new CitadelClient("https://custody.example", {
    fetchImpl: async () =>
      new Response("ok", { headers: { "content-type": "text/plain" } }),
  });
  await assert.rejects(
    () => textClient.snapshot(),
    /unexpected response content type/,
  );

  const largeClient = new CitadelClient("https://custody.example", {
    maxResponseBytes: 1_024,
    fetchImpl: async () =>
      new Response(JSON.stringify({ value: "x".repeat(2_000) }), {
        headers: { "content-type": "application/json" },
      }),
  });
  await assert.rejects(() => largeClient.snapshot(), /size limit/);
});

test("client exposes structured HTTP failures", async () => {
  const client = new CitadelClient("https://custody.example", {
    fetchImpl: async () =>
      new Response(JSON.stringify({ code: "limit_exceeded" }), {
        status: 409,
        statusText: "Conflict",
        headers: { "content-type": "application/json" },
      }),
  });
  await assert.rejects(
    () => client.audit(),
    (error: unknown) =>
      error instanceof CitadelHTTPError && error.status === 409,
  );
});

test("capital and mandate inputs fail closed", () => {
  assert.throws(
    () =>
      computeCapitalMetrics({
        custodyLiability: 1n,
        reserveAssets: 1n,
        pendingOutflows: 0n,
        liquidReserve: 2n,
        reserveHaircutPPM: 0n,
        outflowShockPPM: 0n,
        operationalCostPPM: 0n,
      }),
    /liquid reserve/,
  );
  assert.throws(
    () =>
      canonicalMandate({
        id: " mandate-a",
        parentAccount: "treasury",
        delegateAccount: "delegate-a",
        asset: "USDC",
        dailyLimit: 1n,
        withdrawalLimit: 0n,
        settlementLimit: 0n,
        directWithdrawal: false,
        allowInternalTransfers: true,
        allowOperationalSubaccounts: false,
        label: "Mandato",
      }),
    /mandate id/,
  );
});
