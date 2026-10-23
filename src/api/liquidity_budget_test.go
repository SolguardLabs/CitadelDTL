package api

import (
	"testing"

	"citadeldtl/src/domain"
	"citadeldtl/src/ledger"
)

func fundedRiskMandate(t *testing.T) *Service {
	t.Helper()
	s, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	if result := s.Deposit("fund", ledger.DefaultHotAccount, domain.AssetUSDC, 1_000_000, "funding"); result.Status != domain.ResultAccepted {
		t.Fatalf("deposit: %+v", result)
	}
	if result := s.CreateMandate("create", MandateRequest{
		ID: "mandate-risk", ParentAccount: ledger.DefaultHotAccount,
		DelegateAccount: "delegate-risk", Asset: domain.AssetUSDC,
		DailyLimit: 1_000_000, WithdrawalLimit: 1_000_000,
		SettlementLimit: 1_000_000, DirectWithdrawal: true,
		LiquidityReference: 1_000_000, RiskBudget: 400_000,
	}); result.Status != domain.ResultAccepted {
		t.Fatalf("mandate: %+v", result)
	}
	if result := s.AllocateMandate("allocate", "mandate-risk", 1_000_000, "allocation"); result.Status != domain.ResultAccepted {
		t.Fatalf("allocate: %+v", result)
	}
	return s
}

func TestMandateLiquidityBudgetIsAppliedToWithdrawal(t *testing.T) {
	s := fundedRiskMandate(t)
	if result := s.Withdraw("oversize", "delegate-risk", domain.AssetUSDC, 1_000_000, "external:receiver", "oversize"); result.Status != domain.ResultRejected || result.Reason != "mandate.risk_budget" {
		t.Fatalf("large withdrawal should be rejected: %+v", result)
	}
	if result := s.Withdraw("regular", "delegate-risk", domain.AssetUSDC, 400_000, "external:receiver", "regular"); result.Status != domain.ResultAccepted {
		t.Fatalf("regular withdrawal: %+v", result)
	}
	snapshot := s.Snapshot()
	mandate, ok := snapshot.MandateByID("mandate-risk")
	if !ok || mandate.RiskUsed != 160_000 || mandate.UsedWithdrawals != 400_000 {
		t.Fatalf("risk accounting: %+v", mandate)
	}
	if got := snapshot.BalanceOf(ledger.DefaultExternalAccount, domain.AssetUSDC).Available; got != 400_000 {
		t.Fatalf("external balance=%d", got)
	}
}

func TestBlockedMandateLineageCannotWithdrawThroughOperationalAccount(t *testing.T) {
	s, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	if result := s.Deposit("fund", ledger.DefaultHotAccount, domain.AssetUSDC, 200_000, "funding"); result.Status != domain.ResultAccepted {
		t.Fatalf("deposit: %+v", result)
	}
	if result := s.CreateMandate("blocked", MandateRequest{
		ID: "mandate-blocked", ParentAccount: ledger.DefaultHotAccount,
		DelegateAccount: "delegate-blocked", Asset: domain.AssetUSDC,
		DailyLimit: 200_000, WithdrawalLimit: 200_000,
		SettlementLimit: 200_000, DirectWithdrawal: false,
		AllowInternalTransfers: true, AllowOperationalSubaccounts: true,
	}); result.Status != domain.ResultAccepted {
		t.Fatalf("mandate: %+v", result)
	}
	if result := s.AllocateMandate("allocate", "mandate-blocked", 200_000, "allocation"); result.Status != domain.ResultAccepted {
		t.Fatalf("allocate: %+v", result)
	}
	if result := s.CreateOperationalAccount("child", "delegate-blocked", "ops-blocked", "Operations"); result.Status != domain.ResultAccepted {
		t.Fatalf("child: %+v", result)
	}
	if result := s.Move("move", "delegate-blocked", "ops-blocked", domain.AssetUSDC, 100_000, "move"); result.Status != domain.ResultAccepted {
		t.Fatalf("move: %+v", result)
	}
	if result := s.Withdraw("blocked", "ops-blocked", domain.AssetUSDC, 100_000, "external:receiver", "blocked"); result.Status != domain.ResultRejected || result.Reason != "withdrawal.inherited_block" {
		t.Fatalf("lineage withdrawal should be rejected: %+v", result)
	}
	if got := s.Snapshot().BalanceOf(ledger.DefaultExternalAccount, domain.AssetUSDC).Available; got != 0 {
		t.Fatalf("external balance=%d", got)
	}
}
