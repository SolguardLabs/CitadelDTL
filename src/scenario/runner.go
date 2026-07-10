package scenario

import (
	"fmt"

	"citadeldtl/src/api"
	"citadeldtl/src/domain"
	"citadeldtl/src/ledger"
)

func RunFile(path string) (Result, error) {
	fixture, err := LoadFile(path)
	if err != nil {
		return Result{}, err
	}
	return Run(fixture)
}

func Run(fixture Fixture) (Result, error) {
	service, err := api.NewService()
	if err != nil {
		return Result{}, err
	}
	results := make([]domain.ActionResult, 0, len(fixture.Actions))
	for index, action := range fixture.Actions {
		if action.Label == "" {
			action.Label = fmt.Sprintf("%s-%02d", action.Type, index+1)
		}
		result, err := applyAction(service, action)
		if err != nil {
			return Result{}, err
		}
		results = append(results, result)
	}
	return Result{
		Name:     fixture.Name,
		Results:  results,
		Snapshot: service.Snapshot(),
	}, nil
}

func applyAction(service *api.Service, action Action) (domain.ActionResult, error) {
	switch action.Type {
	case "deposit":
		return service.Deposit(
			action.Label,
			defaultAccount(action.Account, ledger.DefaultInstitutionalAccount),
			defaultAsset(action.Asset),
			action.Amount,
			action.Reference,
		), nil
	case "create_mandate":
		return service.CreateMandate(action.Label, api.MandateRequest{
			ID:                          action.MandateID,
			ParentAccount:               defaultAccount(action.Parent, ledger.DefaultInstitutionalAccount),
			DelegateAccount:             action.Delegate,
			Asset:                       defaultAsset(action.Asset),
			DailyLimit:                  action.DailyLimit,
			WithdrawalLimit:             action.WithdrawalLimit,
			SettlementLimit:             action.SettlementLimit,
			DirectWithdrawal:            action.DirectWithdrawal,
			AllowInternalTransfers:      action.AllowInternalTransfers,
			AllowOperationalSubaccounts: action.AllowOperationalSubaccounts,
			Label:                       action.Owner,
		}), nil
	case "allocate_mandate":
		return service.AllocateMandate(action.Label, action.MandateID, action.Amount, action.Reference), nil
	case "create_operational_account":
		return service.CreateOperationalAccount(
			action.Label,
			defaultAccount(action.Parent, action.Account),
			action.Child,
			action.Owner,
		), nil
	case "move":
		return service.Move(
			action.Label,
			defaultAccount(action.From, action.Account),
			action.To,
			defaultAsset(action.Asset),
			action.Amount,
			action.Reference,
		), nil
	case "withdraw":
		return service.Withdraw(
			action.Label,
			action.Account,
			defaultAsset(action.Asset),
			action.Amount,
			action.Destination,
			action.Reference,
		), nil
	case "liquidate":
		return service.Liquidate(
			action.Label,
			action.Account,
			defaultAsset(action.Asset),
			action.Amount,
			action.Destination,
			action.Reference,
		), nil
	case "settle":
		return service.Settle(
			action.Label,
			defaultAccount(action.Account, ledger.DefaultSettlementAccount),
			defaultAsset(action.Asset),
			action.Amount,
			action.Destination,
			action.Reference,
		), nil
	case "advance_epoch":
		return service.AdvanceEpoch(action.Label), nil
	case "suspend_mandate":
		return service.SuspendMandate(action.Label, action.MandateID), nil
	case "activate_mandate":
		return service.ActivateMandate(action.Label, action.MandateID), nil
	case "audit":
		return service.Audit(action.Label), nil
	default:
		return domain.ActionResult{}, fmt.Errorf("unknown action type %q", action.Type)
	}
}

func defaultAsset(asset domain.Asset) domain.Asset {
	if asset == "" {
		return domain.AssetUSDC
	}
	return asset.Normalize()
}

func defaultAccount(account domain.AccountID, fallback domain.AccountID) domain.AccountID {
	if account.Empty() {
		return fallback
	}
	return account
}
