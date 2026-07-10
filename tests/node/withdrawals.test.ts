import test from "node:test";
import assert from "node:assert/strict";
import { balanceOf, resultByLabel, runFixture } from "../helpers/runner.ts";

test("withdrawal controls reject insufficient funds and direct delegated withdrawals", () => {
  const result = runFixture("withdrawal_controls");

  assert.equal(resultByLabel(result, "hot-withdraw-ok").status, "accepted");
  assert.equal(resultByLabel(result, "hot-withdraw-too-much").status, "rejected");
  assert.equal(resultByLabel(result, "delegate-direct-withdraw-rejected").status, "rejected");
  assert.equal(resultByLabel(result, "delegate-direct-withdraw-rejected").reason, "mandate.direct_withdrawal_blocked");

  assert.equal(balanceOf(result, "inst-hot").available, 220_000);
  assert.equal(balanceOf(result, "delegate-blocked").available, 150_000);
  assert.equal(result.snapshot.summary.processed_receipts, 1);
  assert.equal(result.snapshot.summary.rejected_receipts, 2);
  assert.equal(result.snapshot.summary.externalized_usdc, 80_000);
});

