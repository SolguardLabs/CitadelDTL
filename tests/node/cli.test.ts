import test from "node:test";
import assert from "node:assert/strict";
import { listScenarios } from "../helpers/runner.ts";

test("cli lists deterministic public scenarios", () => {
  assert.deepEqual(listScenarios(), [
    "deposits",
    "mandate_allocation",
    "withdrawal_controls",
    "settlement_audit",
    "mandate_bypass",
  ]);
});

