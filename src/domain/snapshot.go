package domain

type BalanceLine struct {
	Account       AccountID     `json:"account"`
	Owner         string        `json:"owner"`
	Type          AccountType   `json:"type"`
	Asset         Asset         `json:"asset"`
	Available     Amount        `json:"available"`
	Reserved      Amount        `json:"reserved"`
	Segregated    Amount        `json:"segregated"`
	MandateID     MandateID     `json:"mandate_id,omitempty"`
	SegregationID SegregationID `json:"segregation_id,omitempty"`
}

type ReserveLine struct {
	SegregationID SegregationID `json:"segregation_id"`
	Asset         Asset         `json:"asset"`
	CustodyTotal  Amount        `json:"custody_total"`
	ReserveTotal  Amount        `json:"reserve_total"`
	Delta         Amount        `json:"delta"`
}

type SnapshotSummary struct {
	TotalAccounts       int    `json:"total_accounts"`
	TotalMandates       int    `json:"total_mandates"`
	ProcessedReceipts   int    `json:"processed_receipts"`
	RejectedReceipts    int    `json:"rejected_receipts"`
	CriticalIssues      int    `json:"critical_issues"`
	HighIssues          int    `json:"high_issues"`
	SegregatedUSDC      Amount `json:"segregated_usdc"`
	ExternalizedUSDC    Amount `json:"externalized_usdc"`
	OperationalUSDC     Amount `json:"operational_usdc"`
	DelegatedLockedUSDC Amount `json:"delegated_locked_usdc"`
}

type Snapshot struct {
	Epoch       Epoch           `json:"epoch"`
	Accounts    []AccountLine   `json:"accounts"`
	Balances    []BalanceLine   `json:"balances"`
	Mandates    []MandateLine   `json:"mandates"`
	Receipts    []Receipt       `json:"receipts"`
	Events      []LedgerEvent   `json:"events"`
	Reserves    []ReserveLine   `json:"reserves"`
	AuditIssues []AuditIssue    `json:"audit_issues"`
	Summary     SnapshotSummary `json:"summary"`
}

func (s Snapshot) BalanceOf(account AccountID, asset Asset) BalanceLine {
	for _, line := range s.Balances {
		if line.Account == account && line.Asset == asset {
			return line
		}
	}
	return BalanceLine{Account: account, Asset: asset}
}

func (s Snapshot) MandateByID(id MandateID) (MandateLine, bool) {
	for _, line := range s.Mandates {
		if line.ID == id {
			return line, true
		}
	}
	return MandateLine{}, false
}

func (s Snapshot) AccountByID(id AccountID) (AccountLine, bool) {
	for _, line := range s.Accounts {
		if line.ID == id {
			return line, true
		}
	}
	return AccountLine{}, false
}

func (s Snapshot) CriticalIssueCount() int {
	total := 0
	for _, issue := range s.AuditIssues {
		if issue.Severity == SeverityCritical {
			total++
		}
	}
	return total
}
