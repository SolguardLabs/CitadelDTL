package governance

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func fixtureOperation(executeAfter uint64) Operation {
	payload := sha256.Sum256([]byte(`{"limit":"250000","asset":"USDC"}`))
	return Operation{
		Protocol: "CitadelDTL", Network: "institutional-1", Target: "mandate-registry",
		Method: "setDailyLimit", PayloadHash: hex.EncodeToString(payload[:]), Salt: "change-2026-08", ExecuteAfter: executeAfter,
	}
}

func TestOperationIDBindsEveryField(t *testing.T) {
	base := fixtureOperation(120)
	first, err := OperationID(base)
	if err != nil {
		t.Fatal(err)
	}
	second, err := OperationID(base)
	if err != nil || first != second {
		t.Fatal("operation id must be deterministic")
	}
	changed := base
	changed.Network = "institutional-2"
	third, err := OperationID(changed)
	if err != nil {
		t.Fatal(err)
	}
	if first == third {
		t.Fatal("network must be bound into operation id")
	}
}

func TestExecutorRequiresQuorumDelayAndPredecessor(t *testing.T) {
	executor, err := NewExecutor([]string{"gov-a", "gov-b", "gov-c"}, 2, 20, 40, "guardian")
	if err != nil {
		t.Fatal(err)
	}
	first, err := executor.Queue(fixtureOperation(120), "gov-a", 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Execute(first.ID, "gov-a", 119); err == nil {
		t.Fatal("execution before timelock should fail")
	}
	if _, err := executor.Execute(first.ID, "gov-a", 120); err == nil {
		t.Fatal("execution without quorum should fail")
	}
	if _, err := executor.Approve(first.ID, "gov-b", 110); err != nil {
		t.Fatal(err)
	}
	executed, err := executor.Execute(first.ID, "gov-c", 120)
	if err != nil || executed.State != StateExecuted {
		t.Fatalf("expected execution: record=%#v err=%v", executed, err)
	}

	secondOperation := fixtureOperation(150)
	secondOperation.Salt = "dependent-change"
	secondOperation.Predecessor = first.ID
	second, err := executor.Queue(secondOperation, "gov-a", 125)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Approve(second.ID, "gov-b", 130); err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Execute(second.ID, "gov-c", 150); err != nil {
		t.Fatal(err)
	}
}

func TestExecutorRejectsUnknownPredecessorAndExpiredOperation(t *testing.T) {
	executor, err := NewExecutor([]string{"gov-a", "gov-b"}, 2, 10, 5, "guardian")
	if err != nil {
		t.Fatal(err)
	}
	operation := fixtureOperation(20)
	operation.Predecessor = hex.EncodeToString(make([]byte, 32))
	record, err := executor.Queue(operation, "gov-a", 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Approve(record.ID, "gov-b", 15); err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Execute(record.ID, "gov-a", 20); err == nil {
		t.Fatal("unknown predecessor should block execution")
	}
	if _, err := executor.Execute(record.ID, "gov-a", 26); err == nil {
		t.Fatal("expired operation should fail")
	}
}

func TestGuardianCanCancelQueuedOperation(t *testing.T) {
	executor, err := NewExecutor([]string{"gov-a"}, 1, 10, 10, "guardian")
	if err != nil {
		t.Fatal(err)
	}
	record, err := executor.Queue(fixtureOperation(20), "gov-a", 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Cancel(record.ID, "other", 11); err == nil {
		t.Fatal("unexpected guardian authorization")
	}
	cancelled, err := executor.Cancel(record.ID, "guardian", 11)
	if err != nil || cancelled.State != StateCancelled {
		t.Fatalf("expected cancellation: record=%#v err=%v", cancelled, err)
	}
	if _, err := executor.Execute(record.ID, "gov-a", 20); err == nil {
		t.Fatal("cancelled operation should not execute")
	}
}

func TestExecutorReturnsDefensiveCopies(t *testing.T) {
	executor, err := NewExecutor([]string{"gov-a"}, 1, 10, 10, "guardian")
	if err != nil {
		t.Fatal(err)
	}
	record, err := executor.Queue(fixtureOperation(20), "gov-a", 10)
	if err != nil {
		t.Fatal(err)
	}
	record.Approvals[0] = "mutated"
	stored, ok := executor.Record(record.ID)
	if !ok || stored.Approvals[0] != "gov-a" {
		t.Fatal("stored approvals were mutated through caller copy")
	}
}
