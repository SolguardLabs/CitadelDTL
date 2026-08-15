import test from "node:test";
import assert from "node:assert/strict";
import {
  accountById,
  balanceOf,
  mandateById,
  resultByLabel,
  runFixture,
} from "../helpers/runner.ts";

test("mandate allocation creates a blocked delegated account with inherited reserve segregation", () => {
  const result = runFixture("mandate_allocation");

  assert.equal(
    resultByLabel(result, "create-alpha-mandate").status,
    "accepted",
  );
  assert.equal(resultByLabel(result, "allocate-alpha").status, "accepted");
  assert.equal(resultByLabel(result, "audit-allocation").status, "accepted");

  const delegate = accountById(result, "delegate-alpha");
  assert.equal(delegate.type, "delegated");
  assert.equal(delegate.withdrawal_capability, "blocked");
  assert.equal(delegate.allow_subaccounts, true);

  const mandate = mandateById(result, "mandate-alpha");
  assert.equal(mandate.direct_withdrawal, false);
  assert.equal(mandate.remaining_withdrawal, 250_000);
  assert.equal(balanceOf(result, "delegate-alpha").available, 250_000);
  assert.equal(balanceOf(result, "inst-cold").available, 750_000);
});
