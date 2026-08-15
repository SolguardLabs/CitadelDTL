import test from "node:test";
import assert from "node:assert/strict";
import { balanceOf, resultByLabel, runFixture } from "../helpers/runner.ts";

test("liquidation and settlement receipts update omnibus balances and audit without critical issues", () => {
  const result = runFixture("settlement_audit");

  assert.equal(resultByLabel(result, "liquidate-hot").status, "accepted");
  assert.equal(resultByLabel(result, "settle-omnibus").status, "accepted");
  assert.equal(resultByLabel(result, "audit-settlement").status, "accepted");

  assert.equal(balanceOf(result, "inst-hot").available, 375_000);
  assert.equal(balanceOf(result, "settlement-usdc").available, 75_000);
  assert.equal(result.snapshot.summary.processed_receipts, 2);
  assert.equal(result.snapshot.summary.critical_issues, 0);
});
