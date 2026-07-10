import test from "node:test";
import assert from "node:assert/strict";
import { balanceOf, resultByLabel, runFixture } from "../helpers/runner.ts";

test("deposits create matched segregated custody and reserve balances", () => {
  const result = runFixture("deposits");

  assert.equal(result.name, "deposits");
  assert.equal(resultByLabel(result, "deposit-cold-reserve").status, "accepted");
  assert.equal(resultByLabel(result, "deposit-hot-wallet").status, "accepted");
  assert.equal(resultByLabel(result, "audit-clean-deposits").status, "accepted");

  assert.equal(balanceOf(result, "inst-cold").available, 1_000_000);
  assert.equal(balanceOf(result, "inst-hot").available, 250_000);
  assert.equal(balanceOf(result, "reserve-usdc-main").segregated, 1_250_000);
  assert.equal(result.snapshot.reserves[0].delta, 0);
  assert.equal(result.snapshot.summary.critical_issues, 0);
});

