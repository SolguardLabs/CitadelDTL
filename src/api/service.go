package api

import (
	"fmt"

	"citadeldtl/src/audit"
	"citadeldtl/src/domain"
	"citadeldtl/src/ledger"
	"citadeldtl/src/mandate"
	"citadeldtl/src/settlement"
)

type Service struct {
	book     *ledger.Book
	registry *mandate.Registry
	engine   *settlement.Engine
}

type MandateRequest struct {
	ID                          domain.MandateID
	ParentAccount               domain.AccountID
	DelegateAccount             domain.AccountID
	Asset                       domain.Asset
	DailyLimit                  domain.Amount
	WithdrawalLimit             domain.Amount
	SettlementLimit             domain.Amount
	DirectWithdrawal            bool
	AllowInternalTransfers      bool
	AllowOperationalSubaccounts bool
	Label                       string
}

func NewService() (*Service, error) {
	book, err := ledger.NewBootstrappedBook()
	if err != nil {
		return nil, err
	}
	registry := mandate.NewRegistry(book)
	engine := settlement.NewEngine(book, registry)
	return &Service{book: book, registry: registry, engine: engine}, nil
}

func (s *Service) Deposit(label string, account domain.AccountID, asset domain.Asset, amount domain.Amount, reference string) domain.ActionResult {
	if reference == "" {
		reference = label
	}
	err := s.engine.Deposit(account, asset, amount, reference)
	if err != nil {
		return domain.Rejected(label, "deposit", err).WithAccount(account)
	}
	balance := s.book.BalanceOf(account, asset.Normalize())
	return domain.Accepted(label, "deposit").
		WithAccount(account).
		WithDetail("asset", asset.Normalize()).
		WithDetail("amount", amount).
		WithDetail("available", balance.Available).
		WithDetail("segregated", balance.Segregated)
}

func (s *Service) CreateMandate(label string, req MandateRequest) domain.ActionResult {
	if req.ParentAccount.Empty() {
		req.ParentAccount = ledger.DefaultInstitutionalAccount
	}
	if req.Asset == "" {
		req.Asset = domain.AssetUSDC
	}
	if req.Label == "" {
		req.Label = string(req.DelegateAccount)
	}
	mandate, err := s.registry.CreateMandate(domain.MandateInput{
		ID:                          req.ID,
		ParentAccount:               req.ParentAccount,
		DelegateAccount:             req.DelegateAccount,
		Asset:                       req.Asset,
		DailyLimit:                  req.DailyLimit,
		WithdrawalLimit:             req.WithdrawalLimit,
		SettlementLimit:             req.SettlementLimit,
		DirectWithdrawal:            req.DirectWithdrawal,
		AllowInternalTransfers:      req.AllowInternalTransfers,
		AllowOperationalSubaccounts: req.AllowOperationalSubaccounts,
		Label:                       req.Label,
	})
	if err != nil {
		return domain.Rejected(label, "create_mandate", err).WithMandate(req.ID)
	}
	return domain.Accepted(label, "create_mandate").
		WithMandate(mandate.ID).
		WithAccount(mandate.DelegateAccount).
		WithDetail("parent_account", mandate.ParentAccount).
		WithDetail("direct_withdrawal", mandate.DirectWithdrawal).
		WithDetail("daily_limit", mandate.DailyLimit).
		WithDetail("withdrawal_limit", mandate.WithdrawalLimit).
		WithDetail("settlement_limit", mandate.SettlementLimit)
}

func (s *Service) AllocateMandate(label string, mandateID domain.MandateID, amount domain.Amount, reference string) domain.ActionResult {
	if reference == "" {
		reference = label
	}
	err := s.engine.AllocateMandate(mandateID, amount, reference)
	if err != nil {
		return domain.Rejected(label, "allocate_mandate", err).WithMandate(mandateID)
	}
	mandate, _ := s.registry.Mandate(mandateID)
	balance := s.book.BalanceOf(mandate.DelegateAccount, mandate.Asset)
	return domain.Accepted(label, "allocate_mandate").
		WithMandate(mandateID).
		WithAccount(mandate.DelegateAccount).
		WithDetail("amount", amount).
		WithDetail("delegate_available", balance.Available)
}

func (s *Service) CreateOperationalAccount(label string, parent domain.AccountID, child domain.AccountID, owner string) domain.ActionResult {
	account, err := s.registry.CreateOperationalSubaccount(parent, child, owner, label)
	if err != nil {
		return domain.Rejected(label, "create_operational_account", err).WithAccount(child)
	}
	return domain.Accepted(label, "create_operational_account").
		WithAccount(account.ID).
		WithMandate(account.MandateID).
		WithDetail("parent", account.ParentID).
		WithDetail("withdrawal_capability", account.WithdrawalCapability).
		WithDetail("segregation_id", account.SegregationID)
}

func (s *Service) Move(label string, from domain.AccountID, to domain.AccountID, asset domain.Asset, amount domain.Amount, reference string) domain.ActionResult {
	if reference == "" {
		reference = label
	}
	err := s.engine.Move(from, to, asset, amount, reference)
	if err != nil {
		return domain.Rejected(label, "move", err).WithAccount(from)
	}
	fromBalance := s.book.BalanceOf(from, asset.Normalize())
	toBalance := s.book.BalanceOf(to, asset.Normalize())
	return domain.Accepted(label, "move").
		WithAccount(from).
		WithDetail("to", to).
		WithDetail("asset", asset.Normalize()).
		WithDetail("amount", amount).
		WithDetail("from_available", fromBalance.Available).
		WithDetail("to_available", toBalance.Available)
}

func (s *Service) Withdraw(label string, account domain.AccountID, asset domain.Asset, amount domain.Amount, destination string, reference string) domain.ActionResult {
	if reference == "" {
		reference = label
	}
	receipt, err := s.engine.Withdraw(account, asset, amount, destination, reference)
	if err != nil {
		return domain.Rejected(label, "withdraw", err).
			WithAccount(account).
			WithReceipt(receipt.ID).
			WithDetail("receipt_status", receipt.Status)
	}
	return domain.Accepted(label, "withdraw").
		WithAccount(account).
		WithMandate(receipt.MandateID).
		WithReceipt(receipt.ID).
		WithDetail("amount", amount).
		WithDetail("destination", receipt.Destination).
		WithDetail("fee", receipt.Fee)
}

func (s *Service) Liquidate(label string, account domain.AccountID, asset domain.Asset, amount domain.Amount, destination string, reference string) domain.ActionResult {
	if reference == "" {
		reference = label
	}
	receipt, err := s.engine.Liquidate(account, asset, amount, destination, reference)
	if err != nil {
		return domain.Rejected(label, "liquidate", err).
			WithAccount(account).
			WithReceipt(receipt.ID).
			WithDetail("receipt_status", receipt.Status)
	}
	return domain.Accepted(label, "liquidate").
		WithAccount(account).
		WithMandate(receipt.MandateID).
		WithReceipt(receipt.ID).
		WithDetail("amount", amount).
		WithDetail("destination", receipt.Destination).
		WithDetail("fee", receipt.Fee)
}

func (s *Service) Settle(label string, account domain.AccountID, asset domain.Asset, amount domain.Amount, destination string, reference string) domain.ActionResult {
	if reference == "" {
		reference = label
	}
	receipt, err := s.engine.Settle(account, asset, amount, destination, reference)
	if err != nil {
		return domain.Rejected(label, "settle", err).
			WithAccount(account).
			WithReceipt(receipt.ID).
			WithDetail("receipt_status", receipt.Status)
	}
	return domain.Accepted(label, "settle").
		WithAccount(account).
		WithReceipt(receipt.ID).
		WithDetail("amount", amount).
		WithDetail("destination", receipt.Destination).
		WithDetail("fee", receipt.Fee)
}

func (s *Service) AdvanceEpoch(label string) domain.ActionResult {
	epoch := s.book.AdvanceEpoch()
	s.registry.ResetUsage()
	return domain.Accepted(label, "advance_epoch").WithDetail("epoch", epoch)
}

func (s *Service) SuspendMandate(label string, mandateID domain.MandateID) domain.ActionResult {
	err := s.registry.Suspend(mandateID)
	if err != nil {
		return domain.Rejected(label, "suspend_mandate", err).WithMandate(mandateID)
	}
	return domain.Accepted(label, "suspend_mandate").WithMandate(mandateID)
}

func (s *Service) ActivateMandate(label string, mandateID domain.MandateID) domain.ActionResult {
	err := s.registry.Activate(mandateID)
	if err != nil {
		return domain.Rejected(label, "activate_mandate", err).WithMandate(mandateID)
	}
	return domain.Accepted(label, "activate_mandate").WithMandate(mandateID)
}

func (s *Service) Audit(label string) domain.ActionResult {
	snapshot := s.Snapshot()
	result := domain.Accepted(label, "audit").
		WithDetail("issues", len(snapshot.AuditIssues)).
		WithDetail("critical", snapshot.Summary.CriticalIssues).
		WithDetail("high", snapshot.Summary.HighIssues)
	if snapshot.Summary.CriticalIssues > 0 {
		result.Status = domain.ResultRejected
		result.Reason = "audit.critical"
		result.Message = fmt.Sprintf("audit found %d critical issue(s)", snapshot.Summary.CriticalIssues)
	}
	return result
}

func (s *Service) Snapshot() domain.Snapshot {
	receipts := s.engine.Receipts()
	checker := audit.NewChecker(s.book, s.registry, receipts)
	issues := checker.Run()
	summary := checker.Summary(issues)
	return domain.Snapshot{
		Epoch:       s.book.Epoch(),
		Accounts:    s.book.AccountLines(),
		Balances:    s.book.BalanceLines(),
		Mandates:    s.registry.MandateLines(),
		Receipts:    receipts,
		Events:      s.book.Events(),
		Reserves:    s.book.ReserveLines(),
		AuditIssues: issues,
		Summary:     summary,
	}
}

func (s *Service) Book() *ledger.Book {
	return s.book
}

func (s *Service) Registry() *mandate.Registry {
	return s.registry
}

func (s *Service) Engine() *settlement.Engine {
	return s.engine
}
