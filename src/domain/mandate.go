package domain

type Mandate struct {
	ID                          MandateID     `json:"id"`
	ParentAccount               AccountID     `json:"parent_account"`
	DelegateAccount             AccountID     `json:"delegate_account"`
	Asset                       Asset         `json:"asset"`
	SegregationID               SegregationID `json:"segregation_id"`
	DailyLimit                  Amount        `json:"daily_limit"`
	WithdrawalLimit             Amount        `json:"withdrawal_limit"`
	SettlementLimit             Amount        `json:"settlement_limit"`
	UsedDaily                   Amount        `json:"used_daily"`
	UsedSettlement              Amount        `json:"used_settlement"`
	DirectWithdrawal            bool          `json:"direct_withdrawal"`
	AllowInternalTransfers      bool          `json:"allow_internal_transfers"`
	AllowOperationalSubaccounts bool          `json:"allow_operational_subaccounts"`
	Status                      MandateStatus `json:"status"`
	CreatedEpoch                Epoch         `json:"created_epoch"`
	ExpiresEpoch                Epoch         `json:"expires_epoch,omitempty"`
	Label                       string        `json:"label,omitempty"`
}

type MandateInput struct {
	ID                          MandateID
	ParentAccount               AccountID
	DelegateAccount             AccountID
	Asset                       Asset
	SegregationID               SegregationID
	DailyLimit                  Amount
	WithdrawalLimit             Amount
	SettlementLimit             Amount
	DirectWithdrawal            bool
	AllowInternalTransfers      bool
	AllowOperationalSubaccounts bool
	CreatedEpoch                Epoch
	ExpiresEpoch                Epoch
	Label                       string
}

func NewMandate(input MandateInput) (Mandate, error) {
	if err := input.ID.Validate("mandate id"); err != nil {
		return Mandate{}, err
	}
	if err := input.ParentAccount.Validate("parent account"); err != nil {
		return Mandate{}, err
	}
	if err := input.DelegateAccount.Validate("delegate account"); err != nil {
		return Mandate{}, err
	}
	asset := input.Asset.Normalize()
	if err := asset.Validate(); err != nil {
		return Mandate{}, err
	}
	if err := input.DailyLimit.ValidatePositive("daily limit"); err != nil {
		return Mandate{}, err
	}
	withdrawalLimit := input.WithdrawalLimit
	if withdrawalLimit == 0 {
		withdrawalLimit = input.DailyLimit
	}
	if err := withdrawalLimit.ValidatePositive("withdrawal limit"); err != nil {
		return Mandate{}, err
	}
	settlementLimit := input.SettlementLimit
	if settlementLimit == 0 {
		settlementLimit = input.DailyLimit
	}
	if err := settlementLimit.ValidatePositive("settlement limit"); err != nil {
		return Mandate{}, err
	}
	mandate := Mandate{
		ID:                          input.ID,
		ParentAccount:               input.ParentAccount,
		DelegateAccount:             input.DelegateAccount,
		Asset:                       asset,
		SegregationID:               input.SegregationID,
		DailyLimit:                  input.DailyLimit,
		WithdrawalLimit:             withdrawalLimit,
		SettlementLimit:             settlementLimit,
		DirectWithdrawal:            input.DirectWithdrawal,
		AllowInternalTransfers:      input.AllowInternalTransfers,
		AllowOperationalSubaccounts: input.AllowOperationalSubaccounts,
		Status:                      MandateActive,
		CreatedEpoch:                input.CreatedEpoch,
		ExpiresEpoch:                input.ExpiresEpoch,
		Label:                       NormalizeLabel(input.Label),
	}
	return mandate, mandate.Validate()
}

func (m Mandate) Validate() error {
	if err := m.ID.Validate("mandate id"); err != nil {
		return err
	}
	if err := m.ParentAccount.Validate("parent account"); err != nil {
		return err
	}
	if err := m.DelegateAccount.Validate("delegate account"); err != nil {
		return err
	}
	if err := m.Asset.Validate(); err != nil {
		return err
	}
	if err := m.DailyLimit.ValidatePositive("daily limit"); err != nil {
		return err
	}
	if err := m.WithdrawalLimit.ValidatePositive("withdrawal limit"); err != nil {
		return err
	}
	if err := m.SettlementLimit.ValidatePositive("settlement limit"); err != nil {
		return err
	}
	if err := m.UsedDaily.ValidateNonNegative("used daily"); err != nil {
		return err
	}
	if err := m.UsedSettlement.ValidateNonNegative("used settlement"); err != nil {
		return err
	}
	if err := m.Status.Validate(); err != nil {
		return err
	}
	if m.WithdrawalLimit > m.DailyLimit {
		return NewValidationError("mandate.withdrawal_limit", "withdrawal limit cannot exceed daily limit")
	}
	if m.SettlementLimit > m.DailyLimit {
		return NewValidationError("mandate.settlement_limit", "settlement limit cannot exceed daily limit")
	}
	if m.ExpiresEpoch != 0 && m.ExpiresEpoch <= m.CreatedEpoch {
		return NewValidationError("mandate.expiry", "expiry must be after creation epoch")
	}
	return nil
}

func (m Mandate) ActiveAt(epoch Epoch) bool {
	if m.Status != MandateActive {
		return false
	}
	if m.ExpiresEpoch != 0 && epoch >= m.ExpiresEpoch {
		return false
	}
	return true
}

func (m Mandate) RemainingDaily() Amount {
	if m.UsedDaily >= m.DailyLimit {
		return 0
	}
	return m.DailyLimit - m.UsedDaily
}

func (m Mandate) RemainingWithdrawal() Amount {
	base := m.WithdrawalLimit
	if m.UsedDaily >= base {
		return 0
	}
	return base - m.UsedDaily
}

func (m Mandate) RemainingSettlement() Amount {
	if m.UsedSettlement >= m.SettlementLimit {
		return 0
	}
	bySettlement := m.SettlementLimit - m.UsedSettlement
	byDaily := m.RemainingDaily()
	return bySettlement.Min(byDaily)
}

func (m Mandate) CanConsumeWithdrawal(amount Amount, epoch Epoch) error {
	if !m.ActiveAt(epoch) {
		return NewUnauthorizedError("mandate.inactive", "mandate is not active")
	}
	if err := amount.ValidatePositive("withdrawal amount"); err != nil {
		return err
	}
	if amount > m.RemainingWithdrawal() {
		return NewLimitExceededError("mandate.withdrawal_limit", "withdrawal exceeds remaining mandate limit")
	}
	return nil
}

func (m Mandate) CanConsumeSettlement(amount Amount, epoch Epoch) error {
	if !m.ActiveAt(epoch) {
		return NewUnauthorizedError("mandate.inactive", "mandate is not active")
	}
	if err := amount.ValidatePositive("settlement amount"); err != nil {
		return err
	}
	if amount > m.RemainingSettlement() {
		return NewLimitExceededError("mandate.settlement_limit", "settlement exceeds remaining mandate limit")
	}
	return nil
}

func (m Mandate) ConsumeWithdrawal(amount Amount, epoch Epoch) (Mandate, error) {
	if err := m.CanConsumeWithdrawal(amount, epoch); err != nil {
		return m, err
	}
	next := m
	next.UsedDaily += amount
	return next, next.Validate()
}

func (m Mandate) ConsumeSettlement(amount Amount, epoch Epoch) (Mandate, error) {
	if err := m.CanConsumeSettlement(amount, epoch); err != nil {
		return m, err
	}
	next := m
	next.UsedDaily += amount
	next.UsedSettlement += amount
	return next, next.Validate()
}

func (m Mandate) ResetUsage() Mandate {
	next := m
	next.UsedDaily = 0
	next.UsedSettlement = 0
	return next
}

func (m Mandate) WithStatus(status MandateStatus) (Mandate, error) {
	if err := status.Validate(); err != nil {
		return m, err
	}
	next := m
	next.Status = status
	return next, next.Validate()
}

type MandateLine struct {
	ID                          MandateID     `json:"id"`
	ParentAccount               AccountID     `json:"parent_account"`
	DelegateAccount             AccountID     `json:"delegate_account"`
	Asset                       Asset         `json:"asset"`
	SegregationID               SegregationID `json:"segregation_id"`
	DailyLimit                  Amount        `json:"daily_limit"`
	WithdrawalLimit             Amount        `json:"withdrawal_limit"`
	SettlementLimit             Amount        `json:"settlement_limit"`
	UsedDaily                   Amount        `json:"used_daily"`
	UsedSettlement              Amount        `json:"used_settlement"`
	DirectWithdrawal            bool          `json:"direct_withdrawal"`
	AllowInternalTransfers      bool          `json:"allow_internal_transfers"`
	AllowOperationalSubaccounts bool          `json:"allow_operational_subaccounts"`
	Status                      MandateStatus `json:"status"`
	RemainingDaily              Amount        `json:"remaining_daily"`
	RemainingWithdrawal         Amount        `json:"remaining_withdrawal"`
	RemainingSettlement         Amount        `json:"remaining_settlement"`
}

func NewMandateLine(m Mandate) MandateLine {
	return MandateLine{
		ID:                          m.ID,
		ParentAccount:               m.ParentAccount,
		DelegateAccount:             m.DelegateAccount,
		Asset:                       m.Asset,
		SegregationID:               m.SegregationID,
		DailyLimit:                  m.DailyLimit,
		WithdrawalLimit:             m.WithdrawalLimit,
		SettlementLimit:             m.SettlementLimit,
		UsedDaily:                   m.UsedDaily,
		UsedSettlement:              m.UsedSettlement,
		DirectWithdrawal:            m.DirectWithdrawal,
		AllowInternalTransfers:      m.AllowInternalTransfers,
		AllowOperationalSubaccounts: m.AllowOperationalSubaccounts,
		Status:                      m.Status,
		RemainingDaily:              m.RemainingDaily(),
		RemainingWithdrawal:         m.RemainingWithdrawal(),
		RemainingSettlement:         m.RemainingSettlement(),
	}
}
