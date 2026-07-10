import { spawnSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
export const projectRoot = join(here, "..", "..");

export type ActionResult = {
  label: string;
  type: string;
  status: "accepted" | "rejected" | "failed";
  account?: string;
  mandate_id?: string;
  receipt_id?: string;
  reason?: string;
  message?: string;
  details?: Record<string, unknown>;
};

export type ScenarioResult = {
  name: string;
  results: ActionResult[];
  snapshot: {
    epoch: number;
    accounts: Array<{
      id: string;
      owner: string;
      type: string;
      parent_id?: string;
      mandate_id?: string;
      withdrawal_capability: string;
      allow_internal_transfers: boolean;
      allow_subaccounts: boolean;
    }>;
    balances: Array<{
      account: string;
      owner: string;
      type: string;
      asset: string;
      available: number;
      reserved: number;
      segregated: number;
      mandate_id?: string;
      segregation_id?: string;
    }>;
    mandates: Array<{
      id: string;
      parent_account: string;
      delegate_account: string;
      asset: string;
      daily_limit: number;
      withdrawal_limit: number;
      used_daily: number;
      remaining_withdrawal: number;
      direct_withdrawal: boolean;
      allow_operational_subaccounts: boolean;
    }>;
    receipts: Array<{
      id: string;
      kind: string;
      account: string;
      destination?: string;
      asset: string;
      amount: number;
      fee: number;
      mandate_id?: string;
      status: "processed" | "rejected" | "queued";
      reason?: string;
    }>;
    reserves: Array<{
      segregation_id: string;
      asset: string;
      custody_total: number;
      reserve_total: number;
      delta: number;
    }>;
    audit_issues: Array<{
      code: string;
      severity: string;
      account?: string;
      mandate_id?: string;
      asset?: string;
      amount?: number;
      message: string;
    }>;
    summary: {
      total_accounts: number;
      total_mandates: number;
      processed_receipts: number;
      rejected_receipts: number;
      critical_issues: number;
      high_issues: number;
      segregated_usdc: number;
      externalized_usdc: number;
      operational_usdc: number;
      delegated_locked_usdc: number;
    };
  };
};

export function runFixture(name: string): ScenarioResult {
  const fixturePath = join(projectRoot, "tests", "fixtures", `${name}.json`);
  const child = spawnSync("go", ["run", "./cmd/citadeldtl", "run", fixturePath], {
    cwd: projectRoot,
    encoding: "utf8",
  });
  if (child.status !== 0) {
    throw new Error(
      [
        `fixture ${name} failed`,
        `status: ${child.status}`,
        `error: ${child.error?.message ?? ""}`,
        `stdout: ${child.stdout}`,
        `stderr: ${child.stderr}`,
      ].join("\n"),
    );
  }
  return JSON.parse(child.stdout) as ScenarioResult;
}

export function listScenarios(): string[] {
  const child = spawnSync("go", ["run", "./cmd/citadeldtl", "list"], {
    cwd: projectRoot,
    encoding: "utf8",
  });
  if (child.status !== 0) {
    throw new Error(child.error?.message ?? child.stderr);
  }
  return child.stdout.trim().split(/\r?\n/).filter(Boolean);
}

export function resultByLabel(result: ScenarioResult, label: string): ActionResult {
  const found = result.results.find((entry) => entry.label === label);
  if (!found) {
    throw new Error(`missing action label ${label}`);
  }
  return found;
}

export function balanceOf(result: ScenarioResult, account: string, asset = "USDC") {
  return (
    result.snapshot.balances.find((line) => line.account === account && line.asset === asset) ?? {
      account,
      asset,
      available: 0,
      reserved: 0,
      segregated: 0,
    }
  );
}

export function accountById(result: ScenarioResult, id: string) {
  const found = result.snapshot.accounts.find((account) => account.id === id);
  if (!found) {
    throw new Error(`missing account ${id}`);
  }
  return found;
}

export function mandateById(result: ScenarioResult, id: string) {
  const found = result.snapshot.mandates.find((mandate) => mandate.id === id);
  if (!found) {
    throw new Error(`missing mandate ${id}`);
  }
  return found;
}

export function issueCodes(result: ScenarioResult): string[] {
  return result.snapshot.audit_issues.map((issue) => issue.code);
}
