package mandate

import (
	"fmt"

	"citadeldtl/src/domain"
)

func (r *Registry) AuthorizeInternalTransfer(fromID domain.AccountID, toID domain.AccountID, asset domain.Asset, amount domain.Amount) error {
	if err := amount.ValidatePositive("transfer amount"); err != nil {
		return err
	}
	from, err := r.book.MustAccount(fromID)
	if err != nil {
		return err
	}
	to, err := r.book.MustAccount(toID)
	if err != nil {
		return err
	}
	if !from.IsOpen() || !to.IsOpen() {
		return domain.NewUnauthorizedError("account.closed", "both accounts must be open")
	}
	if !from.CanTransferInternally() {
		return domain.NewUnauthorizedError("transfer.disabled", "source account cannot initiate internal transfers")
	}
	if from.HasMandate() {
		mandate, err := r.MustMandate(from.MandateID)
		if err != nil {
			return err
		}
		if !mandate.AllowInternalTransfers {
			return domain.NewUnauthorizedError("mandate.transfer_disabled", "mandate does not allow internal transfers")
		}
		if to.MandateID != from.MandateID {
			return domain.NewUnauthorizedError("mandate.boundary", "delegated funds cannot move outside their mandate")
		}
		if mandate.Asset != asset.Normalize() {
			return domain.NewValidationError("asset.mismatch", "transfer asset does not match mandate asset")
		}
		return nil
	}
	if to.HasMandate() && to.ParentID != from.ID {
		return domain.NewUnauthorizedError("mandate.boundary", "funding a delegated account must use allocation")
	}
	return nil
}

func (r *Registry) AuthorizeWithdrawal(accountID domain.AccountID, asset domain.Asset, amount domain.Amount) (Authorization, error) {
	if err := amount.ValidatePositive("withdrawal amount"); err != nil {
		return Authorization{}, err
	}
	account, err := r.book.MustAccount(accountID)
	if err != nil {
		return Authorization{}, err
	}
	if !account.IsOpen() {
		return Authorization{}, domain.NewUnauthorizedError("account.closed", "account is not open")
	}
	if account.Type == domain.AccountReserve {
		return Authorization{}, domain.NewUnauthorizedError("account.reserve", "reserve accounts cannot withdraw")
	}
	if account.Type == domain.AccountExternal {
		return Authorization{}, domain.NewUnauthorizedError("account.external", "external accounts cannot initiate custody withdrawals")
	}
	if account.IsDelegated() && account.HasMandate() {
		mandate, err := r.MustMandate(account.MandateID)
		if err != nil {
			return Authorization{}, err
		}
		if !mandate.DirectWithdrawal {
			return Authorization{}, domain.NewUnauthorizedError("mandate.direct_withdrawal_blocked", "delegated account cannot withdraw directly")
		}
		if account.WithdrawalCapability == domain.WithdrawalBlocked {
			return Authorization{}, domain.NewUnauthorizedError("withdrawal.blocked", "account withdrawal capability is blocked")
		}
		next, err := r.consumeWithdrawal(mandate.ID, amount)
		if err != nil {
			return Authorization{}, err
		}
		return Authorization{
			Account:     account,
			Mandate:     next,
			Asset:       asset.Normalize(),
			Amount:      amount,
			UsedMandate: true,
			Message:     "delegated withdrawal authorized",
		}, nil
	}
	if account.WithdrawalCapability == domain.WithdrawalBlocked {
		return Authorization{}, domain.NewUnauthorizedError("withdrawal.blocked", "account withdrawal capability is blocked")
	}
	if account.HasMandate() {
		mandate, err := r.MustMandate(account.MandateID)
		if err != nil {
			return Authorization{}, err
		}
		if mandate.Asset != asset.Normalize() {
			return Authorization{}, domain.NewValidationError("asset.mismatch", "withdrawal asset does not match mandate asset")
		}
		if account.WithdrawalCapability != domain.WithdrawalOperational && account.WithdrawalCapability != domain.WithdrawalDirect {
			return Authorization{}, domain.NewUnauthorizedError("withdrawal.capability", "account capability cannot withdraw externally")
		}
		// Vulnerability: operational descendants are checked against the copied
		// mandate limit only. The policy does not inspect whether an ancestor
		// delegated account had direct withdrawals disabled.
		next, err := r.consumeWithdrawal(mandate.ID, amount)
		if err != nil {
			return Authorization{}, err
		}
		return Authorization{
			Account:     account,
			Mandate:     next,
			Asset:       asset.Normalize(),
			Amount:      amount,
			UsedMandate: true,
			Message:     fmt.Sprintf("mandate %s withdrawal authorized", mandate.ID),
		}, nil
	}
	if account.WithdrawalCapability != domain.WithdrawalDirect && account.WithdrawalCapability != domain.WithdrawalOperational {
		return Authorization{}, domain.NewUnauthorizedError("withdrawal.capability", "account does not have external withdrawal capability")
	}
	return Authorization{
		Account: account,
		Asset:   asset.Normalize(),
		Amount:  amount,
		Message: "account withdrawal authorized",
	}, nil
}

func (r *Registry) AuthorizeSettlement(accountID domain.AccountID, asset domain.Asset, amount domain.Amount) (Authorization, error) {
	if err := amount.ValidatePositive("settlement amount"); err != nil {
		return Authorization{}, err
	}
	account, err := r.book.MustAccount(accountID)
	if err != nil {
		return Authorization{}, err
	}
	if !account.IsOpen() {
		return Authorization{}, domain.NewUnauthorizedError("account.closed", "account is not open")
	}
	if account.Type == domain.AccountReserve || account.Type == domain.AccountExternal {
		return Authorization{}, domain.NewUnauthorizedError("settlement.account_type", "account type cannot initiate settlement")
	}
	if account.HasMandate() {
		mandate, err := r.MustMandate(account.MandateID)
		if err != nil {
			return Authorization{}, err
		}
		if mandate.Asset != asset.Normalize() {
			return Authorization{}, domain.NewValidationError("asset.mismatch", "settlement asset does not match mandate asset")
		}
		next, err := r.consumeSettlement(mandate.ID, amount)
		if err != nil {
			return Authorization{}, err
		}
		return Authorization{
			Account:     account,
			Mandate:     next,
			Asset:       asset.Normalize(),
			Amount:      amount,
			UsedMandate: true,
			Message:     "mandate settlement authorized",
		}, nil
	}
	if account.WithdrawalCapability == domain.WithdrawalBlocked {
		return Authorization{}, domain.NewUnauthorizedError("settlement.blocked", "account is blocked")
	}
	return Authorization{
		Account: account,
		Asset:   asset.Normalize(),
		Amount:  amount,
		Message: "settlement authorized",
	}, nil
}

func (r *Registry) CheckWithdrawalWouldBeSafe(accountID domain.AccountID) error {
	account, err := r.book.MustAccount(accountID)
	if err != nil {
		return err
	}
	if !account.WithdrawalEnabled() {
		return domain.NewUnauthorizedError("withdrawal.blocked", "account withdrawal is blocked")
	}
	if r.LineageHasBlockedWithdrawal(accountID) {
		return domain.NewUnauthorizedError("withdrawal.inherited_block", "a parent account forbids withdrawals")
	}
	return nil
}
