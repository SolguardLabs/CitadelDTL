import { createHash, randomUUID } from "node:crypto";

export const PPM = 1_000_000n;

export type ClientOptions = {
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
  maxResponseBytes?: number;
  allowInsecureLocalhost?: boolean;
};

export type RequestOptions = {
  idempotencyKey?: string;
  signal?: AbortSignal;
};

export type MandateDraft = {
  id: string;
  parentAccount: string;
  delegateAccount: string;
  asset: "USDC" | "EURC" | "USD";
  dailyLimit: bigint;
  withdrawalLimit: bigint;
  settlementLimit: bigint;
  directWithdrawal: boolean;
  allowInternalTransfers: boolean;
  allowOperationalSubaccounts: boolean;
  label: string;
};

export type TransferDraft = {
  from: string;
  to: string;
  asset: "USDC" | "EURC" | "USD";
  amount: bigint;
  reference: string;
};

export type CapitalInput = {
  custodyLiability: bigint;
  reserveAssets: bigint;
  pendingOutflows: bigint;
  liquidReserve: bigint;
  reserveHaircutPPM: bigint;
  outflowShockPPM: bigint;
  operationalCostPPM: bigint;
};

export type CapitalMetrics = {
  effectiveReserve: bigint;
  stressedOutflows: bigint;
  operationalBuffer: bigint;
  requiredCapital: bigint;
  capitalDeficit: bigint;
  coveragePPM: bigint;
  liquidityPPM: bigint;
};

export class CitadelClient {
  readonly #baseURL: URL;
  readonly #fetch: typeof fetch;
  readonly #timeoutMs: number;
  readonly #maxResponseBytes: number;

  constructor(baseURL: string, options: ClientOptions = {}) {
    const parsed = new URL(baseURL);
    const local =
      parsed.hostname === "localhost" ||
      parsed.hostname === "127.0.0.1" ||
      parsed.hostname === "::1";
    if (
      parsed.protocol !== "https:" &&
      !(local && options.allowInsecureLocalhost)
    ) {
      throw new Error(
        "CitadelClient requires HTTPS outside an explicitly allowed local environment",
      );
    }
    if (parsed.username || parsed.password || parsed.search || parsed.hash) {
      throw new Error(
        "CitadelClient base URL cannot include credentials, query, or fragment",
      );
    }
    parsed.pathname = parsed.pathname.replace(/\/+$/, "") + "/";
    this.#baseURL = parsed;
    this.#fetch = options.fetchImpl ?? fetch;
    this.#timeoutMs = boundedInteger(
      options.timeoutMs ?? 8_000,
      100,
      60_000,
      "timeoutMs",
    );
    this.#maxResponseBytes = boundedInteger(
      options.maxResponseBytes ?? 1_000_000,
      1_024,
      8_000_000,
      "maxResponseBytes",
    );
  }

  snapshot<T = unknown>(options: RequestOptions = {}): Promise<T> {
    return this.#request<T>("v1/snapshot", { method: "GET" }, options);
  }

  audit<T = unknown>(options: RequestOptions = {}): Promise<T> {
    return this.#request<T>(
      "v1/audit",
      { method: "POST", body: "{}" },
      options,
    );
  }

  createMandate<T = unknown>(
    draft: MandateDraft,
    options: RequestOptions = {},
  ): Promise<T> {
    return this.#request<T>(
      "v1/mandates",
      { method: "POST", body: canonicalJSON(canonicalMandate(draft)) },
      { ...options, idempotencyKey: options.idempotencyKey ?? randomUUID() },
    );
  }

  transfer<T = unknown>(
    draft: TransferDraft,
    options: RequestOptions = {},
  ): Promise<T> {
    return this.#request<T>(
      "v1/transfers",
      { method: "POST", body: canonicalJSON(canonicalTransfer(draft)) },
      { ...options, idempotencyKey: options.idempotencyKey ?? randomUUID() },
    );
  }

  async #request<T>(
    path: string,
    init: RequestInit,
    options: RequestOptions,
  ): Promise<T> {
    const url = new URL(path, this.#baseURL);
    if (
      url.origin !== this.#baseURL.origin ||
      !url.pathname.startsWith(this.#baseURL.pathname)
    ) {
      throw new Error(
        "request path escapes the configured CitadelDTL endpoint",
      );
    }
    const controller = new AbortController();
    const timeout = setTimeout(
      () => controller.abort(new Error("request timeout")),
      this.#timeoutMs,
    );
    const abortFromCaller = () => controller.abort(options.signal?.reason);
    options.signal?.addEventListener("abort", abortFromCaller, { once: true });
    try {
      const headers = new Headers(init.headers);
      headers.set("accept", "application/json");
      headers.set("content-type", "application/json");
      if (options.idempotencyKey) {
        headers.set(
          "idempotency-key",
          normalizeToken(options.idempotencyKey, "idempotency key"),
        );
      }
      const response = await this.#fetch(url, {
        ...init,
        headers,
        cache: "no-store",
        credentials: "omit",
        redirect: "error",
        signal: controller.signal,
      });
      const contentType =
        response.headers.get("content-type")?.toLowerCase() ?? "";
      if (!contentType.startsWith("application/json")) {
        throw new Error(
          `unexpected response content type: ${contentType || "missing"}`,
        );
      }
      const declaredLength = Number(
        response.headers.get("content-length") ?? "0",
      );
      if (
        Number.isFinite(declaredLength) &&
        declaredLength > this.#maxResponseBytes
      ) {
        throw new Error("response exceeds configured size limit");
      }
      const text = await response.text();
      if (Buffer.byteLength(text, "utf8") > this.#maxResponseBytes) {
        throw new Error("response exceeds configured size limit");
      }
      const body = text === "" ? null : (JSON.parse(text) as unknown);
      if (!response.ok) {
        throw new CitadelHTTPError(response.status, response.statusText, body);
      }
      return body as T;
    } finally {
      clearTimeout(timeout);
      options.signal?.removeEventListener("abort", abortFromCaller);
    }
  }
}

export class CitadelHTTPError extends Error {
  constructor(
    readonly status: number,
    readonly statusText: string,
    readonly body: unknown,
  ) {
    super(
      `CitadelDTL request failed with HTTP ${status}${statusText ? ` ${statusText}` : ""}`,
    );
    this.name = "CitadelHTTPError";
  }
}

export function canonicalMandate(draft: MandateDraft): Record<string, unknown> {
  const id = normalizeToken(draft.id, "mandate id");
  const parent = normalizeToken(draft.parentAccount, "parent account");
  const delegate = normalizeToken(draft.delegateAccount, "delegate account");
  const label = draft.label.trim();
  if (label === "") throw new Error("label is required");
  return {
    allow_internal_transfers: draft.allowInternalTransfers,
    allow_operational_subaccounts: draft.allowOperationalSubaccounts,
    asset: draft.asset,
    daily_limit: positiveDecimal(draft.dailyLimit, "daily limit"),
    delegate_account: delegate,
    direct_withdrawal: draft.directWithdrawal,
    id,
    label,
    parent_account: parent,
    settlement_limit: nonNegativeDecimal(
      draft.settlementLimit,
      "settlement limit",
    ),
    withdrawal_limit: nonNegativeDecimal(
      draft.withdrawalLimit,
      "withdrawal limit",
    ),
  };
}

export function canonicalTransfer(
  draft: TransferDraft,
): Record<string, string> {
  return {
    amount: positiveDecimal(draft.amount, "transfer amount"),
    asset: draft.asset,
    from: normalizeToken(draft.from, "source account"),
    reference: normalizeToken(draft.reference, "reference"),
    to: normalizeToken(draft.to, "destination account"),
  };
}

export function canonicalJSON(value: unknown): string {
  return JSON.stringify(sortValue(value));
}

export function payloadHash(value: unknown): string {
  return createHash("sha256")
    .update(canonicalJSON(value), "utf8")
    .digest("hex");
}

export function computeCapitalMetrics(input: CapitalInput): CapitalMetrics {
  for (const [field, value] of Object.entries(input)) {
    if (value < 0n) throw new Error(`${field} must be non-negative`);
  }
  if (input.reserveHaircutPPM > PPM)
    throw new Error("reserveHaircutPPM exceeds one million");
  if (input.liquidReserve > input.reserveAssets)
    throw new Error("liquid reserve exceeds total reserve assets");
  const effectiveReserve = mulDivFloor(
    input.reserveAssets,
    PPM - input.reserveHaircutPPM,
    PPM,
  );
  const stressedOutflows = mulDivCeil(
    input.pendingOutflows,
    PPM + input.outflowShockPPM,
    PPM,
  );
  const operationalBuffer = mulDivCeil(
    input.custodyLiability,
    input.operationalCostPPM,
    PPM,
  );
  const requiredCapital =
    input.custodyLiability + stressedOutflows + operationalBuffer;
  return {
    effectiveReserve,
    stressedOutflows,
    operationalBuffer,
    requiredCapital,
    capitalDeficit:
      requiredCapital > effectiveReserve
        ? requiredCapital - effectiveReserve
        : 0n,
    coveragePPM: ratioPPM(effectiveReserve, requiredCapital),
    liquidityPPM: ratioPPM(input.liquidReserve, stressedOutflows),
  };
}

function sortValue(value: unknown): unknown {
  if (typeof value === "bigint") return value.toString(10);
  if (Array.isArray(value)) return value.map(sortValue);
  if (value !== null && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>)
        .sort(([left], [right]) => left.localeCompare(right))
        .map(([key, entry]) => [key, sortValue(entry)]),
    );
  }
  if (typeof value === "number" && !Number.isSafeInteger(value)) {
    throw new Error("canonical JSON accepts only safe integer numbers");
  }
  return value;
}

function mulDivFloor(a: bigint, b: bigint, denominator: bigint): bigint {
  if (denominator <= 0n) throw new Error("denominator must be positive");
  return (a * b) / denominator;
}

function mulDivCeil(a: bigint, b: bigint, denominator: bigint): bigint {
  if (denominator <= 0n) throw new Error("denominator must be positive");
  const product = a * b;
  return product === 0n ? 0n : (product + denominator - 1n) / denominator;
}

function ratioPPM(numerator: bigint, denominator: bigint): bigint {
  if (denominator === 0n) return numerator === 0n ? 0n : (1n << 256n) - 1n;
  return mulDivFloor(numerator, PPM, denominator);
}

function positiveDecimal(value: bigint, field: string): string {
  if (value <= 0n) throw new Error(`${field} must be positive`);
  return value.toString(10);
}

function nonNegativeDecimal(value: bigint, field: string): string {
  if (value < 0n) throw new Error(`${field} must be non-negative`);
  return value.toString(10);
}

function normalizeToken(value: string, field: string): string {
  const normalized = value.trim();
  if (normalized === "" || normalized !== value || /\s/.test(value)) {
    throw new Error(`${field} must be non-empty and contain no whitespace`);
  }
  if (value.length > 128) throw new Error(`${field} is too long`);
  return value;
}

function boundedInteger(
  value: number,
  minimum: number,
  maximum: number,
  field: string,
): number {
  if (!Number.isInteger(value) || value < minimum || value > maximum) {
    throw new Error(
      `${field} must be an integer within [${minimum}, ${maximum}]`,
    );
  }
  return value;
}
