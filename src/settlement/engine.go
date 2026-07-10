package settlement

import (
	"fmt"
	"sort"

	"citadeldtl/src/domain"
	"citadeldtl/src/ledger"
	"citadeldtl/src/mandate"
)

type Engine struct {
	book          *ledger.Book
	registry      *mandate.Registry
	receipts      []domain.Receipt
	nextReceiptID int64
}

func NewEngine(book *ledger.Book, registry *mandate.Registry) *Engine {
	return &Engine{
		book:          book,
		registry:      registry,
		receipts:      []domain.Receipt{},
		nextReceiptID: 1,
	}
}

func (e *Engine) Deposit(accountID domain.AccountID, asset domain.Asset, amount domain.Amount, reference string) error {
	account, err := e.book.MustAccount(accountID)
	if err != nil {
		return err
	}
	asset = asset.Normalize()
	if err := asset.Validate(); err != nil {
		return err
	}
	if err := amount.ValidatePositive("deposit amount"); err != nil {
		return err
	}
	segregated := account.IsCustody()
	if err := e.book.Credit(accountID, asset, amount, reference, segregated); err != nil {
		return err
	}
	if segregated {
		reserve, ok := e.book.FindReserveAccount(account.SegregationID, asset)
		if !ok {
			return domain.NewInvariantError("reserve.missing", "segregated deposit has no reserve account")
		}
		if err := e.book.Credit(reserve.ID, asset, amount, reference+":reserve", true); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) AllocateMandate(mandateID domain.MandateID, amount domain.Amount, reference string) error {
	return e.registry.AllocateToDelegate(mandateID, amount, reference)
}

func (e *Engine) Move(from domain.AccountID, to domain.AccountID, asset domain.Asset, amount domain.Amount, reference string) error {
	if err := e.registry.AuthorizeInternalTransfer(from, to, asset.Normalize(), amount); err != nil {
		return err
	}
	return e.book.Transfer(from, to, asset.Normalize(), amount, reference, true, domain.MovementInternalTransfer)
}

func (e *Engine) Withdraw(accountID domain.AccountID, asset domain.Asset, amount domain.Amount, destination string, reference string) (domain.Receipt, error) {
	account, err := e.book.MustAccount(accountID)
	if err != nil {
		receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementWithdrawal, err)
		return receipt, err
	}
	asset = asset.Normalize()
	if err := e.ensureSpendable(accountID, asset, amount); err != nil {
		receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementWithdrawal, err)
		return receipt, err
	}
	if account.IsCustody() {
		if err := e.ensureReserve(account.SegregationID, asset, amount); err != nil {
			receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementWithdrawal, err)
			return receipt, err
		}
	}
	auth, err := e.registry.AuthorizeWithdrawal(accountID, asset, amount)
	if err != nil {
		receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementWithdrawal, err)
		return receipt, err
	}
	if err := e.book.Debit(accountID, asset, amount, reference, account.IsCustody(), domain.MovementWithdrawal); err != nil {
		receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementWithdrawal, err)
		return receipt, err
	}
	if account.IsCustody() {
		reserve, _ := e.book.FindReserveAccount(account.SegregationID, asset)
		if err := e.book.Debit(reserve.ID, asset, amount, reference+":reserve", true, domain.MovementReserveAdjustment); err != nil {
			receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementWithdrawal, err)
			return receipt, err
		}
	}
	if destination == "" {
		destination = string(ledger.DefaultExternalAccount)
	}
	_ = e.book.Credit(ledger.DefaultExternalAccount, asset, amount, reference+":external", false)
	receipt := e.processedReceipt(accountID, destination, asset, amount, domain.MovementWithdrawal, auth)
	e.receipts = append(e.receipts, receipt)
	return receipt, nil
}

func (e *Engine) Liquidate(accountID domain.AccountID, asset domain.Asset, amount domain.Amount, destination string, reference string) (domain.Receipt, error) {
	account, err := e.book.MustAccount(accountID)
	if err != nil {
		receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementLiquidation, err)
		return receipt, err
	}
	asset = asset.Normalize()
	if err := e.ensureSpendable(accountID, asset, amount); err != nil {
		receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementLiquidation, err)
		return receipt, err
	}
	auth, err := e.registry.AuthorizeSettlement(accountID, asset, amount)
	if err != nil {
		receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementLiquidation, err)
		return receipt, err
	}
	target := ledger.DefaultSettlementAccount
	if destination != "" {
		target = domain.AccountID(destination)
	}
	if _, ok := e.book.Account(target); !ok {
		target = ledger.DefaultSettlementAccount
	}
	if err := e.book.Transfer(accountID, target, asset, amount, reference, account.IsCustody(), domain.MovementLiquidation); err != nil {
		receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementLiquidation, err)
		return receipt, err
	}
	receipt := e.processedReceipt(accountID, string(target), asset, amount, domain.MovementLiquidation, auth)
	e.receipts = append(e.receipts, receipt)
	return receipt, nil
}

func (e *Engine) Settle(accountID domain.AccountID, asset domain.Asset, amount domain.Amount, destination string, reference string) (domain.Receipt, error) {
	account, err := e.book.MustAccount(accountID)
	if err != nil {
		receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementSettlement, err)
		return receipt, err
	}
	if account.Type != domain.AccountSettlement {
		err := domain.NewUnauthorizedError("settlement.account", "settlement must start from settlement account")
		receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementSettlement, err)
		return receipt, err
	}
	asset = asset.Normalize()
	if err := e.ensureSpendable(accountID, asset, amount); err != nil {
		receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementSettlement, err)
		return receipt, err
	}
	if err := e.book.Debit(accountID, asset, amount, reference, true, domain.MovementSettlement); err != nil {
		receipt := e.rejectedReceipt(accountID, destination, asset, amount, domain.MovementSettlement, err)
		return receipt, err
	}
	if destination == "" {
		destination = "clearing-house"
	}
	receipt := e.processedReceipt(accountID, destination, asset, amount, domain.MovementSettlement, mandate.Authorization{Account: account, Asset: asset, Amount: amount})
	e.receipts = append(e.receipts, receipt)
	return receipt, nil
}

func (e *Engine) Receipts() []domain.Receipt {
	out := make([]domain.Receipt, len(e.receipts))
	copy(out, e.receipts)
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (e *Engine) ensureSpendable(accountID domain.AccountID, asset domain.Asset, amount domain.Amount) error {
	if err := amount.ValidatePositive("amount"); err != nil {
		return err
	}
	balance := e.book.BalanceOf(accountID, asset.Normalize())
	if balance.Available < amount {
		return domain.NewInsufficientFundsError("balance.available", "account has insufficient available balance")
	}
	return nil
}

func (e *Engine) ensureReserve(segregationID domain.SegregationID, asset domain.Asset, amount domain.Amount) error {
	reserve, ok := e.book.FindReserveAccount(segregationID, asset.Normalize())
	if !ok {
		return domain.NewInvariantError("reserve.missing", "reserve account not found")
	}
	balance := e.book.BalanceOf(reserve.ID, asset.Normalize())
	if balance.Available < amount || balance.Segregated < amount {
		return domain.NewInvariantError("reserve.insufficient", "segregated reserve cannot cover withdrawal")
	}
	return nil
}

func (e *Engine) processedReceipt(accountID domain.AccountID, destination string, asset domain.Asset, amount domain.Amount, kind domain.MovementKind, auth mandate.Authorization) domain.Receipt {
	receipt := domain.Receipt{
		ID:            e.nextReceipt(),
		Epoch:         e.book.Epoch(),
		Kind:          kind,
		Account:       accountID,
		Destination:   destination,
		Asset:         asset.Normalize(),
		Amount:        amount,
		Fee:           feeFor(kind, amount),
		MandateID:     auth.Account.MandateID,
		SegregationID: auth.Account.SegregationID,
		Status:        domain.ReceiptProcessed,
	}
	if receipt.MandateID.Empty() {
		receipt.MandateID = auth.Mandate.ID
	}
	if receipt.SegregationID.Empty() {
		receipt.SegregationID = auth.Mandate.SegregationID
	}
	return receipt
}

func (e *Engine) rejectedReceipt(accountID domain.AccountID, destination string, asset domain.Asset, amount domain.Amount, kind domain.MovementKind, err error) domain.Receipt {
	receipt := domain.Receipt{
		ID:          e.nextReceipt(),
		Epoch:       e.book.Epoch(),
		Kind:        kind,
		Account:     accountID,
		Destination: destination,
		Asset:       asset.Normalize(),
		Amount:      amount,
		Status:      domain.ReceiptRejected,
		Reason:      domain.ErrorReason(err),
	}
	if account, ok := e.book.Account(accountID); ok {
		receipt.MandateID = account.MandateID
		receipt.SegregationID = account.SegregationID
	}
	e.receipts = append(e.receipts, receipt)
	return receipt
}

func (e *Engine) nextReceipt() domain.ReceiptID {
	id := domain.ReceiptID(fmt.Sprintf("rcpt-%06d", e.nextReceiptID))
	e.nextReceiptID++
	return id
}

func feeFor(kind domain.MovementKind, amount domain.Amount) domain.Amount {
	switch kind {
	case domain.MovementWithdrawal:
		fee := amount / 1000
		if fee < 25 {
			return 25
		}
		return fee
	case domain.MovementLiquidation:
		fee := amount / 2000
		if fee < 10 {
			return 10
		}
		return fee
	default:
		return 0
	}
}
