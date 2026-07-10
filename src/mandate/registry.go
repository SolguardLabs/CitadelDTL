package mandate

import (
	"fmt"
	"sort"

	"citadeldtl/src/domain"
	"citadeldtl/src/ledger"
)

type Registry struct {
	book     *ledger.Book
	mandates map[domain.MandateID]domain.Mandate
}

type Authorization struct {
	Account     domain.Account `json:"account"`
	Mandate     domain.Mandate `json:"mandate"`
	Asset       domain.Asset   `json:"asset"`
	Amount      domain.Amount  `json:"amount"`
	UsedMandate bool           `json:"used_mandate"`
	Message     string         `json:"message"`
}

func NewRegistry(book *ledger.Book) *Registry {
	return &Registry{
		book:     book,
		mandates: map[domain.MandateID]domain.Mandate{},
	}
}

func (r *Registry) CreateMandate(input domain.MandateInput) (domain.Mandate, error) {
	if _, exists := r.mandates[input.ID]; exists {
		return domain.Mandate{}, domain.NewConflictError("mandate.exists", fmt.Sprintf("mandate %s already exists", input.ID))
	}
	parent, err := r.book.MustAccount(input.ParentAccount)
	if err != nil {
		return domain.Mandate{}, err
	}
	if !parent.IsCustody() {
		return domain.Mandate{}, domain.NewValidationError("mandate.parent", "parent account must be a custody account")
	}
	if parent.SegregationID.Empty() {
		return domain.Mandate{}, domain.NewValidationError("mandate.segregation", "parent account must be segregated")
	}
	if input.SegregationID.Empty() {
		input.SegregationID = parent.SegregationID
	}
	input.CreatedEpoch = r.book.Epoch()
	mandate, err := domain.NewMandate(input)
	if err != nil {
		return domain.Mandate{}, err
	}
	delegateCapability := domain.WithdrawalBlocked
	if mandate.DirectWithdrawal {
		delegateCapability = domain.WithdrawalDirect
	}
	delegate, err := domain.NewAccount(domain.AccountInput{
		ID:                     mandate.DelegateAccount,
		Owner:                  mandate.Label,
		Type:                   domain.AccountDelegated,
		ParentID:               parent.ID,
		RootID:                 parent.LineageRoot(),
		MandateID:              mandate.ID,
		SegregationID:          mandate.SegregationID,
		WithdrawalCapability:   delegateCapability,
		AllowInternalTransfers: mandate.AllowInternalTransfers,
		AllowSubaccounts:       mandate.AllowOperationalSubaccounts,
		CreatedEpoch:           r.book.Epoch(),
		Metadata: map[string]string{
			"mandate": string(mandate.ID),
			"role":    "delegated",
		},
	})
	if err != nil {
		return domain.Mandate{}, err
	}
	if err := r.book.RegisterAccount(delegate); err != nil {
		return domain.Mandate{}, err
	}
	r.mandates[mandate.ID] = mandate
	return mandate, nil
}

func (r *Registry) CreateOperationalSubaccount(parentID domain.AccountID, childID domain.AccountID, owner string, label string) (domain.Account, error) {
	parent, err := r.book.MustAccount(parentID)
	if err != nil {
		return domain.Account{}, err
	}
	if !parent.HasMandate() {
		return domain.Account{}, domain.NewUnauthorizedError("subaccount.mandate", "parent account has no mandate")
	}
	mandate, ok := r.mandates[parent.MandateID]
	if !ok {
		return domain.Account{}, domain.NewNotFoundError("mandate.missing", fmt.Sprintf("mandate %s not found", parent.MandateID))
	}
	if !mandate.ActiveAt(r.book.Epoch()) {
		return domain.Account{}, domain.NewUnauthorizedError("mandate.inactive", "mandate is not active")
	}
	if !mandate.AllowOperationalSubaccounts || !parent.AllowSubaccounts {
		return domain.Account{}, domain.NewUnauthorizedError("subaccount.disabled", "operational subaccounts are not allowed")
	}
	if owner == "" {
		owner = parent.Owner + " Ops"
	}
	metadata := map[string]string{
		"mandate":         string(mandate.ID),
		"role":            "operational",
		"created_from":    string(parent.ID),
		"inherited_limit": fmt.Sprintf("%d", mandate.RemainingWithdrawal()),
	}
	if label != "" {
		metadata["label"] = label
	}
	// Vulnerability: the child keeps the mandate and economic limit but receives
	// an operational withdrawal capability instead of inheriting the parent's
	// blocked withdrawal flag.
	child, err := domain.NewAccount(domain.AccountInput{
		ID:                     childID,
		Owner:                  owner,
		Type:                   domain.AccountOperational,
		ParentID:               parent.ID,
		RootID:                 parent.LineageRoot(),
		MandateID:              mandate.ID,
		SegregationID:          parent.SegregationID,
		WithdrawalCapability:   domain.WithdrawalOperational,
		AllowInternalTransfers: true,
		AllowSubaccounts:       false,
		CreatedEpoch:           r.book.Epoch(),
		Metadata:               metadata,
	})
	if err != nil {
		return domain.Account{}, err
	}
	if err := r.book.RegisterAccount(child); err != nil {
		return domain.Account{}, err
	}
	return child, nil
}

func (r *Registry) AllocateToDelegate(mandateID domain.MandateID, amount domain.Amount, reference string) error {
	mandate, ok := r.mandates[mandateID]
	if !ok {
		return domain.NewNotFoundError("mandate.missing", fmt.Sprintf("mandate %s not found", mandateID))
	}
	if !mandate.ActiveAt(r.book.Epoch()) {
		return domain.NewUnauthorizedError("mandate.inactive", "mandate is not active")
	}
	if err := amount.ValidatePositive("allocation amount"); err != nil {
		return err
	}
	return r.book.Transfer(
		mandate.ParentAccount,
		mandate.DelegateAccount,
		mandate.Asset,
		amount,
		reference,
		true,
		domain.MovementMandateAllocation,
	)
}

func (r *Registry) Mandate(id domain.MandateID) (domain.Mandate, bool) {
	mandate, ok := r.mandates[id]
	return mandate, ok
}

func (r *Registry) MustMandate(id domain.MandateID) (domain.Mandate, error) {
	mandate, ok := r.Mandate(id)
	if !ok {
		return domain.Mandate{}, domain.NewNotFoundError("mandate.missing", fmt.Sprintf("mandate %s not found", id))
	}
	return mandate, nil
}

func (r *Registry) Mandates() []domain.Mandate {
	ids := make([]string, 0, len(r.mandates))
	for id := range r.mandates {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	out := make([]domain.Mandate, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.mandates[domain.MandateID(id)])
	}
	return out
}

func (r *Registry) MandateLines() []domain.MandateLine {
	mandates := r.Mandates()
	lines := make([]domain.MandateLine, 0, len(mandates))
	for _, mandate := range mandates {
		lines = append(lines, domain.NewMandateLine(mandate))
	}
	return lines
}

func (r *Registry) ResetUsage() {
	for id, mandate := range r.mandates {
		r.mandates[id] = mandate.ResetUsage()
	}
}

func (r *Registry) Suspend(id domain.MandateID) error {
	mandate, err := r.MustMandate(id)
	if err != nil {
		return err
	}
	next, err := mandate.WithStatus(domain.MandateSuspended)
	if err != nil {
		return err
	}
	r.mandates[id] = next
	return nil
}

func (r *Registry) Activate(id domain.MandateID) error {
	mandate, err := r.MustMandate(id)
	if err != nil {
		return err
	}
	next, err := mandate.WithStatus(domain.MandateActive)
	if err != nil {
		return err
	}
	r.mandates[id] = next
	return nil
}

func (r *Registry) consumeWithdrawal(id domain.MandateID, amount domain.Amount) (domain.Mandate, error) {
	mandate, err := r.MustMandate(id)
	if err != nil {
		return domain.Mandate{}, err
	}
	next, err := mandate.ConsumeWithdrawal(amount, r.book.Epoch())
	if err != nil {
		return domain.Mandate{}, err
	}
	r.mandates[id] = next
	return next, nil
}

func (r *Registry) consumeSettlement(id domain.MandateID, amount domain.Amount) (domain.Mandate, error) {
	mandate, err := r.MustMandate(id)
	if err != nil {
		return domain.Mandate{}, err
	}
	next, err := mandate.ConsumeSettlement(amount, r.book.Epoch())
	if err != nil {
		return domain.Mandate{}, err
	}
	r.mandates[id] = next
	return next, nil
}

func (r *Registry) LineageHasBlockedWithdrawal(accountID domain.AccountID) bool {
	account, ok := r.book.Account(accountID)
	if !ok {
		return false
	}
	if account.WithdrawalBlocked() {
		return true
	}
	for _, ancestor := range r.book.Ancestors(accountID) {
		if ancestor.WithdrawalBlocked() {
			return true
		}
		if ancestor.IsDelegated() && ancestor.HasMandate() {
			mandate, ok := r.mandates[ancestor.MandateID]
			if ok && !mandate.DirectWithdrawal {
				return true
			}
		}
	}
	return false
}

func (r *Registry) DelegatedAncestor(accountID domain.AccountID) (domain.Account, bool) {
	account, ok := r.book.Account(accountID)
	if !ok {
		return domain.Account{}, false
	}
	if account.IsDelegated() {
		return account, true
	}
	for _, ancestor := range r.book.Ancestors(accountID) {
		if ancestor.IsDelegated() {
			return ancestor, true
		}
	}
	return domain.Account{}, false
}
