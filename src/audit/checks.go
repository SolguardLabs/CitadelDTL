package audit

import (
	"fmt"
	"sort"

	"citadeldtl/src/domain"
	"citadeldtl/src/ledger"
	"citadeldtl/src/mandate"
)

type Checker struct {
	book     *ledger.Book
	registry *mandate.Registry
	receipts []domain.Receipt
}

func NewChecker(book *ledger.Book, registry *mandate.Registry, receipts []domain.Receipt) *Checker {
	copied := make([]domain.Receipt, len(receipts))
	copy(copied, receipts)
	return &Checker{book: book, registry: registry, receipts: copied}
}

func (c *Checker) Run() []domain.AuditIssue {
	issues := []domain.AuditIssue{}
	issues = append(issues, c.checkReserveCoverage()...)
	issues = append(issues, c.checkBlockedLineageWithdrawals()...)
	issues = append(issues, c.checkOperationalEscalation()...)
	issues = append(issues, c.checkMandateUsage()...)
	issues = append(issues, c.checkSuspendedAccounts()...)
	sort.SliceStable(issues, func(i, j int) bool {
		if severityRank(issues[i].Severity) == severityRank(issues[j].Severity) {
			if issues[i].Code == issues[j].Code {
				return issues[i].Account < issues[j].Account
			}
			return issues[i].Code < issues[j].Code
		}
		return severityRank(issues[i].Severity) > severityRank(issues[j].Severity)
	})
	return issues
}

func (c *Checker) Summary(issues []domain.AuditIssue) domain.SnapshotSummary {
	summary := domain.SnapshotSummary{
		TotalAccounts:     len(c.book.Accounts()),
		TotalMandates:     len(c.registry.Mandates()),
		ProcessedReceipts: c.countReceipts(domain.ReceiptProcessed),
		RejectedReceipts:  c.countReceipts(domain.ReceiptRejected),
	}
	for _, issue := range issues {
		switch issue.Severity {
		case domain.SeverityCritical:
			summary.CriticalIssues++
		case domain.SeverityHigh:
			summary.HighIssues++
		}
	}
	for _, line := range c.book.BalanceLines() {
		if line.Asset != domain.AssetUSDC {
			continue
		}
		switch line.Type {
		case domain.AccountReserve:
			summary.SegregatedUSDC += line.Segregated
		case domain.AccountExternal:
			summary.ExternalizedUSDC += line.Available
		case domain.AccountOperational:
			summary.OperationalUSDC += line.Available
		case domain.AccountDelegated:
			if account, ok := c.book.Account(line.Account); ok && account.WithdrawalBlocked() {
				summary.DelegatedLockedUSDC += line.Available
			}
		}
	}
	return summary
}

func (c *Checker) checkReserveCoverage() []domain.AuditIssue {
	issues := []domain.AuditIssue{}
	for _, reserve := range c.book.ReserveLines() {
		if reserve.SegregationID.Empty() {
			continue
		}
		if reserve.Delta < 0 {
			issues = append(issues, domain.AuditIssue{
				Code:           "reserve.under_collateralized",
				Severity:       domain.SeverityCritical,
				Asset:          reserve.Asset,
				Amount:         -reserve.Delta,
				Message:        fmt.Sprintf("segregated reserve %s is short by %d %s", reserve.SegregationID, -reserve.Delta, reserve.Asset),
				Recommendation: "block withdrawals until reserves are restored",
			})
			continue
		}
		if reserve.Delta > 0 {
			issues = append(issues, domain.AuditIssue{
				Code:           "reserve.excess",
				Severity:       domain.SeverityInfo,
				Asset:          reserve.Asset,
				Amount:         reserve.Delta,
				Message:        fmt.Sprintf("segregated reserve %s has excess %d %s", reserve.SegregationID, reserve.Delta, reserve.Asset),
				Recommendation: "reconcile reserve excess against custody liabilities",
			})
		}
	}
	return issues
}

func (c *Checker) checkBlockedLineageWithdrawals() []domain.AuditIssue {
	issues := []domain.AuditIssue{}
	for _, receipt := range c.receipts {
		if receipt.Status != domain.ReceiptProcessed || receipt.Kind != domain.MovementWithdrawal {
			continue
		}
		if c.registry.LineageHasBlockedWithdrawal(receipt.Account) {
			issues = append(issues, domain.AuditIssue{
				Code:           "mandate.blocked_lineage_withdrew",
				Severity:       domain.SeverityCritical,
				Account:        receipt.Account,
				MandateID:      receipt.MandateID,
				Asset:          receipt.Asset,
				Amount:         receipt.Amount,
				Message:        fmt.Sprintf("processed withdrawal %s from account %s under a blocked delegated lineage", receipt.ID, receipt.Account),
				Recommendation: "enforce inherited withdrawal restrictions before debit",
			})
		}
	}
	return issues
}

func (c *Checker) checkOperationalEscalation() []domain.AuditIssue {
	issues := []domain.AuditIssue{}
	for _, account := range c.book.Accounts() {
		if !account.IsOperational() || !account.WithdrawalEnabled() || !account.HasMandate() {
			continue
		}
		ancestor, ok := c.registry.DelegatedAncestor(account.ID)
		if !ok {
			continue
		}
		mandateLineage, ok := c.registry.Mandate(ancestor.MandateID)
		if !ok {
			continue
		}
		if !mandateLineage.DirectWithdrawal {
			issues = append(issues, domain.AuditIssue{
				Code:           "mandate.operational_escalation",
				Severity:       domain.SeverityHigh,
				Account:        account.ID,
				MandateID:      account.MandateID,
				Message:        fmt.Sprintf("operational account %s can withdraw although delegated ancestor %s is blocked", account.ID, ancestor.ID),
				Recommendation: "make withdrawal restriction monotonic across subaccounts",
			})
		}
	}
	return issues
}

func (c *Checker) checkMandateUsage() []domain.AuditIssue {
	issues := []domain.AuditIssue{}
	for _, mandate := range c.registry.Mandates() {
		if mandate.UsedDaily > mandate.DailyLimit {
			issues = append(issues, domain.AuditIssue{
				Code:           "mandate.daily_limit_exceeded",
				Severity:       domain.SeverityCritical,
				MandateID:      mandate.ID,
				Asset:          mandate.Asset,
				Amount:         mandate.UsedDaily - mandate.DailyLimit,
				Message:        fmt.Sprintf("mandate %s exceeded daily limit", mandate.ID),
				Recommendation: "make usage accounting atomic with authorization",
			})
		}
		if mandate.UsedSettlement > mandate.SettlementLimit {
			issues = append(issues, domain.AuditIssue{
				Code:           "mandate.settlement_limit_exceeded",
				Severity:       domain.SeverityCritical,
				MandateID:      mandate.ID,
				Asset:          mandate.Asset,
				Amount:         mandate.UsedSettlement - mandate.SettlementLimit,
				Message:        fmt.Sprintf("mandate %s exceeded settlement limit", mandate.ID),
				Recommendation: "reconcile settlement usage with daily cap",
			})
		}
		if mandate.Status == domain.MandateSuspended && mandate.UsedDaily > 0 {
			issues = append(issues, domain.AuditIssue{
				Code:           "mandate.suspended_used",
				Severity:       domain.SeverityHigh,
				MandateID:      mandate.ID,
				Asset:          mandate.Asset,
				Amount:         mandate.UsedDaily,
				Message:        fmt.Sprintf("suspended mandate %s has usage", mandate.ID),
				Recommendation: "freeze all descendant accounts on suspension",
			})
		}
	}
	return issues
}

func (c *Checker) checkSuspendedAccounts() []domain.AuditIssue {
	issues := []domain.AuditIssue{}
	for _, account := range c.book.Accounts() {
		if account.Status != domain.AccountSuspended {
			continue
		}
		for _, line := range c.book.BalanceLines() {
			if line.Account != account.ID {
				continue
			}
			if line.Available > 0 {
				issues = append(issues, domain.AuditIssue{
					Code:           "account.suspended_with_balance",
					Severity:       domain.SeverityWarning,
					Account:        account.ID,
					Asset:          line.Asset,
					Amount:         line.Available,
					Message:        fmt.Sprintf("suspended account %s still has spendable balance", account.ID),
					Recommendation: "reserve or migrate spendable balance before suspension",
				})
			}
		}
	}
	return issues
}

func (c *Checker) countReceipts(status domain.ReceiptStatus) int {
	count := 0
	for _, receipt := range c.receipts {
		if receipt.Status == status {
			count++
		}
	}
	return count
}

func severityRank(severity domain.IssueSeverity) int {
	switch severity {
	case domain.SeverityCritical:
		return 4
	case domain.SeverityHigh:
		return 3
	case domain.SeverityWarning:
		return 2
	case domain.SeverityInfo:
		return 1
	default:
		return 0
	}
}
