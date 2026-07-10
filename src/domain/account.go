package domain

import "sort"

type Account struct {
	ID                     AccountID            `json:"id"`
	Owner                  string               `json:"owner"`
	Type                   AccountType          `json:"type"`
	Status                 AccountStatus        `json:"status"`
	ParentID               AccountID            `json:"parent_id,omitempty"`
	RootID                 AccountID            `json:"root_id,omitempty"`
	MandateID              MandateID            `json:"mandate_id,omitempty"`
	SegregationID          SegregationID        `json:"segregation_id,omitempty"`
	WithdrawalCapability   WithdrawalCapability `json:"withdrawal_capability"`
	AllowInternalTransfers bool                 `json:"allow_internal_transfers"`
	AllowSubaccounts       bool                 `json:"allow_subaccounts"`
	CreatedEpoch           Epoch                `json:"created_epoch"`
	Metadata               map[string]string    `json:"metadata,omitempty"`
}

type AccountInput struct {
	ID                     AccountID
	Owner                  string
	Type                   AccountType
	ParentID               AccountID
	RootID                 AccountID
	MandateID              MandateID
	SegregationID          SegregationID
	WithdrawalCapability   WithdrawalCapability
	AllowInternalTransfers bool
	AllowSubaccounts       bool
	CreatedEpoch           Epoch
	Metadata               map[string]string
}

func NewAccount(input AccountInput) (Account, error) {
	if err := input.ID.Validate("account id"); err != nil {
		return Account{}, err
	}
	if err := input.Type.Validate(); err != nil {
		return Account{}, err
	}
	if input.Owner == "" {
		return Account{}, NewValidationError("account.owner", "owner is required")
	}
	status := AccountOpen
	capability := input.WithdrawalCapability
	if capability == "" {
		capability = DefaultWithdrawalCapability(input.Type)
	}
	if err := capability.Validate(); err != nil {
		return Account{}, err
	}
	root := input.RootID
	if root.Empty() {
		if input.ParentID.Empty() {
			root = input.ID
		} else {
			root = input.ParentID
		}
	}
	account := Account{
		ID:                     input.ID,
		Owner:                  NormalizeLabel(input.Owner),
		Type:                   input.Type,
		Status:                 status,
		ParentID:               input.ParentID,
		RootID:                 root,
		MandateID:              input.MandateID,
		SegregationID:          input.SegregationID,
		WithdrawalCapability:   capability,
		AllowInternalTransfers: input.AllowInternalTransfers,
		AllowSubaccounts:       input.AllowSubaccounts,
		CreatedEpoch:           input.CreatedEpoch,
		Metadata:               copyStringMap(input.Metadata),
	}
	return account, account.Validate()
}

func (a Account) Validate() error {
	if err := a.ID.Validate("account id"); err != nil {
		return err
	}
	if a.Owner == "" {
		return NewValidationError("account.owner", "owner is required")
	}
	if err := a.Type.Validate(); err != nil {
		return err
	}
	if err := a.Status.Validate(); err != nil {
		return err
	}
	if err := a.WithdrawalCapability.Validate(); err != nil {
		return err
	}
	if !a.ParentID.Empty() && a.ParentID == a.ID {
		return NewValidationError("account.parent", "account cannot be its own parent")
	}
	if a.RootID.Empty() {
		return NewValidationError("account.root", "root id is required")
	}
	return nil
}

func (a Account) IsOpen() bool {
	return a.Status == AccountOpen
}

func (a Account) IsCustody() bool {
	return IsCustodyType(a.Type)
}

func (a Account) IsSystem() bool {
	return IsSystemType(a.Type)
}

func (a Account) IsDelegated() bool {
	return a.Type == AccountDelegated
}

func (a Account) IsOperational() bool {
	return a.Type == AccountOperational
}

func (a Account) IsInstitutional() bool {
	return a.Type == AccountInstitutional
}

func (a Account) HasMandate() bool {
	return !a.MandateID.Empty()
}

func (a Account) CanCreateSubaccounts() bool {
	return a.IsOpen() && a.AllowSubaccounts
}

func (a Account) CanTransferInternally() bool {
	return a.IsOpen() && a.AllowInternalTransfers
}

func (a Account) WithdrawalBlocked() bool {
	return a.WithdrawalCapability == WithdrawalBlocked
}

func (a Account) WithdrawalEnabled() bool {
	switch a.WithdrawalCapability {
	case WithdrawalDirect, WithdrawalOperational, WithdrawalSettlement:
		return true
	default:
		return false
	}
}

func (a Account) Clone() Account {
	a.Metadata = copyStringMap(a.Metadata)
	return a
}

func (a Account) WithStatus(status AccountStatus) (Account, error) {
	if err := status.Validate(); err != nil {
		return a, err
	}
	next := a.Clone()
	next.Status = status
	return next, next.Validate()
}

func (a Account) WithMandate(id MandateID) Account {
	next := a.Clone()
	next.MandateID = id
	return next
}

func (a Account) WithMetadata(key string, value string) Account {
	next := a.Clone()
	if next.Metadata == nil {
		next.Metadata = map[string]string{}
	}
	next.Metadata[key] = value
	return next
}

func (a Account) LineageRoot() AccountID {
	if !a.RootID.Empty() {
		return a.RootID
	}
	if !a.ParentID.Empty() {
		return a.ParentID
	}
	return a.ID
}

func (a Account) CapabilityRank() int {
	switch a.WithdrawalCapability {
	case WithdrawalBlocked:
		return 0
	case WithdrawalSettlement:
		return 1
	case WithdrawalOperational:
		return 2
	case WithdrawalDirect:
		return 3
	default:
		return 0
	}
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(input))
	for _, key := range keys {
		out[key] = input[key]
	}
	return out
}

type AccountLine struct {
	ID                     AccountID            `json:"id"`
	Owner                  string               `json:"owner"`
	Type                   AccountType          `json:"type"`
	Status                 AccountStatus        `json:"status"`
	ParentID               AccountID            `json:"parent_id,omitempty"`
	RootID                 AccountID            `json:"root_id,omitempty"`
	MandateID              MandateID            `json:"mandate_id,omitempty"`
	SegregationID          SegregationID        `json:"segregation_id,omitempty"`
	WithdrawalCapability   WithdrawalCapability `json:"withdrawal_capability"`
	AllowInternalTransfers bool                 `json:"allow_internal_transfers"`
	AllowSubaccounts       bool                 `json:"allow_subaccounts"`
}

func NewAccountLine(account Account) AccountLine {
	return AccountLine{
		ID:                     account.ID,
		Owner:                  account.Owner,
		Type:                   account.Type,
		Status:                 account.Status,
		ParentID:               account.ParentID,
		RootID:                 account.RootID,
		MandateID:              account.MandateID,
		SegregationID:          account.SegregationID,
		WithdrawalCapability:   account.WithdrawalCapability,
		AllowInternalTransfers: account.AllowInternalTransfers,
		AllowSubaccounts:       account.AllowSubaccounts,
	}
}
