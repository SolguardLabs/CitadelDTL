package domain

import (
	"fmt"
	"strings"
)

type Asset string
type AccountID string
type MandateID string
type EventID string
type ReceiptID string
type OperationID string
type SegregationID string
type Epoch int64

const (
	AssetUSDC Asset = "USDC"
	AssetEURC Asset = "EURC"
	AssetUSD  Asset = "USD"
)

type AccountType string

const (
	AccountInstitutional AccountType = "institutional"
	AccountDelegated     AccountType = "delegated"
	AccountOperational   AccountType = "operational"
	AccountReserve       AccountType = "reserve"
	AccountSettlement    AccountType = "settlement"
	AccountExternal      AccountType = "external"
)

type AccountStatus string

const (
	AccountOpen      AccountStatus = "open"
	AccountSuspended AccountStatus = "suspended"
	AccountClosed    AccountStatus = "closed"
)

type WithdrawalCapability string

const (
	WithdrawalBlocked     WithdrawalCapability = "blocked"
	WithdrawalDirect      WithdrawalCapability = "direct"
	WithdrawalOperational WithdrawalCapability = "operational"
	WithdrawalSettlement  WithdrawalCapability = "settlement_only"
)

type MandateStatus string

const (
	MandateDraft     MandateStatus = "draft"
	MandateActive    MandateStatus = "active"
	MandateSuspended MandateStatus = "suspended"
	MandateExpired   MandateStatus = "expired"
)

type MovementKind string

const (
	MovementDeposit             MovementKind = "deposit"
	MovementInternalTransfer    MovementKind = "internal_transfer"
	MovementMandateAllocation   MovementKind = "mandate_allocation"
	MovementWithdrawal          MovementKind = "withdrawal"
	MovementLiquidation         MovementKind = "liquidation"
	MovementSettlement          MovementKind = "settlement"
	MovementReserveAdjustment   MovementKind = "reserve_adjustment"
	MovementOperationalCreation MovementKind = "operational_creation"
)

type ResultStatus string

const (
	ResultAccepted ResultStatus = "accepted"
	ResultRejected ResultStatus = "rejected"
	ResultFailed   ResultStatus = "failed"
)

type ReceiptStatus string

const (
	ReceiptProcessed ReceiptStatus = "processed"
	ReceiptRejected  ReceiptStatus = "rejected"
	ReceiptQueued    ReceiptStatus = "queued"
)

type IssueSeverity string

const (
	SeverityInfo     IssueSeverity = "info"
	SeverityWarning  IssueSeverity = "warning"
	SeverityHigh     IssueSeverity = "high"
	SeverityCritical IssueSeverity = "critical"
)

func (a Asset) String() string {
	return string(a)
}

func (a Asset) Normalize() Asset {
	return Asset(strings.ToUpper(strings.TrimSpace(string(a))))
}

func (a Asset) Validate() error {
	switch a.Normalize() {
	case AssetUSDC, AssetEURC, AssetUSD:
		return nil
	default:
		return NewValidationError("asset.invalid", fmt.Sprintf("unsupported asset %q", a))
	}
}

func (id AccountID) String() string {
	return string(id)
}

func (id AccountID) Empty() bool {
	return strings.TrimSpace(string(id)) == ""
}

func (id AccountID) Validate(field string) error {
	if id.Empty() {
		return NewValidationError("account.empty", field+" is required")
	}
	if strings.ContainsAny(string(id), " \t\r\n") {
		return NewValidationError("account.invalid", field+" cannot contain whitespace")
	}
	return nil
}

func (id MandateID) String() string {
	return string(id)
}

func (id MandateID) Empty() bool {
	return strings.TrimSpace(string(id)) == ""
}

func (id MandateID) Validate(field string) error {
	if id.Empty() {
		return NewValidationError("mandate.empty", field+" is required")
	}
	if strings.ContainsAny(string(id), " \t\r\n") {
		return NewValidationError("mandate.invalid", field+" cannot contain whitespace")
	}
	return nil
}

func (id SegregationID) String() string {
	return string(id)
}

func (id SegregationID) Empty() bool {
	return strings.TrimSpace(string(id)) == ""
}

func (typ AccountType) Validate() error {
	switch typ {
	case AccountInstitutional, AccountDelegated, AccountOperational, AccountReserve, AccountSettlement, AccountExternal:
		return nil
	default:
		return NewValidationError("account_type.invalid", fmt.Sprintf("unsupported account type %q", typ))
	}
}

func (status AccountStatus) Validate() error {
	switch status {
	case AccountOpen, AccountSuspended, AccountClosed:
		return nil
	default:
		return NewValidationError("account_status.invalid", fmt.Sprintf("unsupported account status %q", status))
	}
}

func (cap WithdrawalCapability) Validate() error {
	switch cap {
	case WithdrawalBlocked, WithdrawalDirect, WithdrawalOperational, WithdrawalSettlement:
		return nil
	default:
		return NewValidationError("withdrawal_capability.invalid", fmt.Sprintf("unsupported withdrawal capability %q", cap))
	}
}

func (status MandateStatus) Validate() error {
	switch status {
	case MandateDraft, MandateActive, MandateSuspended, MandateExpired:
		return nil
	default:
		return NewValidationError("mandate_status.invalid", fmt.Sprintf("unsupported mandate status %q", status))
	}
}

func DefaultWithdrawalCapability(typ AccountType) WithdrawalCapability {
	switch typ {
	case AccountInstitutional:
		return WithdrawalDirect
	case AccountDelegated:
		return WithdrawalBlocked
	case AccountOperational:
		return WithdrawalOperational
	case AccountSettlement:
		return WithdrawalSettlement
	case AccountReserve:
		return WithdrawalBlocked
	case AccountExternal:
		return WithdrawalDirect
	default:
		return WithdrawalBlocked
	}
}

func IsCustodyType(typ AccountType) bool {
	switch typ {
	case AccountInstitutional, AccountDelegated, AccountOperational:
		return true
	default:
		return false
	}
}

func IsSystemType(typ AccountType) bool {
	switch typ {
	case AccountReserve, AccountSettlement:
		return true
	default:
		return false
	}
}

func NormalizeLabel(value string) string {
	return strings.TrimSpace(value)
}
