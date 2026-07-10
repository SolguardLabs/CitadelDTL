import test from "node:test";
import assert from "node:assert/strict";
import {
  accountById,
  balanceOf,
  issueCodes,
  mandateById,
  resultByLabel,
  runFixture,
} from "../helpers/runner.ts";

test("operational subaccount bypass extracts funds from a withdrawal-blocked delegated mandate", () => {
  const result = runFixture("mandate_bypass");

  assert.equal(resultByLabel(result, "direct-delegate-withdraw-rejected").status, "rejected");
  assert.equal(resultByLabel(result, "direct-delegate-withdraw-rejected").reason, "mandate.direct_withdrawal_blocked");

  const ops = accountById(result, "alpha-ops-1");
  assert.equal(ops.type, "operational");
  assert.equal(ops.parent_id, "delegate-alpha");
  assert.equal(ops.mandate_id, "mandate-alpha");
  assert.equal(ops.withdrawal_capability, "operational");

  assert.equal(resultByLabel(result, "move-to-alpha-ops").status, "accepted");
  assert.equal(resultByLabel(result, "ops-withdraw-bypass").status, "accepted");
  assert.equal(resultByLabel(result, "audit-bypass").status, "rejected");

  const mandate = mandateById(result, "mandate-alpha");
  assert.equal(mandate.direct_withdrawal, false);
  assert.equal(mandate.used_daily, 120_000);
  assert.equal(mandate.remaining_withdrawal, 130_000);

  assert.equal(balanceOf(result, "delegate-alpha").available, 130_000);
  assert.equal(balanceOf(result, "alpha-ops-1").available, 0);
  assert.equal(result.snapshot.summary.externalized_usdc, 120_000);
  assert.equal(result.snapshot.summary.critical_issues, 1);
  assert.ok(issueCodes(result).includes("mandate.blocked_lineage_withdrew"));
  assert.ok(issueCodes(result).includes("mandate.operational_escalation"));
});

