package ledger

import "citadeldtl/src/domain"

func NewBootstrappedBook() (*Book, error) {
	book := NewBook()
	accounts := []domain.AccountInput{
		{
			ID:                     DefaultInstitutionalAccount,
			Owner:                  "Citadel Institutional Cold",
			Type:                   domain.AccountInstitutional,
			SegregationID:          DefaultSegregationID,
			WithdrawalCapability:   domain.WithdrawalBlocked,
			AllowInternalTransfers: true,
			AllowSubaccounts:       true,
			CreatedEpoch:           book.Epoch(),
			Metadata:               map[string]string{"tier": "cold", "policy": "segregated"},
		},
		{
			ID:                     DefaultHotAccount,
			Owner:                  "Citadel Institutional Hot",
			Type:                   domain.AccountInstitutional,
			SegregationID:          DefaultSegregationID,
			WithdrawalCapability:   domain.WithdrawalDirect,
			AllowInternalTransfers: true,
			AllowSubaccounts:       true,
			CreatedEpoch:           book.Epoch(),
			Metadata:               map[string]string{"tier": "hot", "policy": "operator"},
		},
		{
			ID:                     DefaultReserveAccount,
			Owner:                  "Citadel Segregated Reserve",
			Type:                   domain.AccountReserve,
			SegregationID:          DefaultSegregationID,
			WithdrawalCapability:   domain.WithdrawalBlocked,
			AllowInternalTransfers: false,
			AllowSubaccounts:       false,
			CreatedEpoch:           book.Epoch(),
			Metadata:               map[string]string{"asset": "USDC"},
		},
		{
			ID:                     DefaultSettlementAccount,
			Owner:                  "Citadel Settlement Omnibus",
			Type:                   domain.AccountSettlement,
			SegregationID:          DefaultSegregationID,
			WithdrawalCapability:   domain.WithdrawalSettlement,
			AllowInternalTransfers: true,
			AllowSubaccounts:       false,
			CreatedEpoch:           book.Epoch(),
			Metadata:               map[string]string{"rail": "stablecoin"},
		},
		{
			ID:                     DefaultExternalAccount,
			Owner:                  "External Banking Rail",
			Type:                   domain.AccountExternal,
			SegregationID:          "",
			WithdrawalCapability:   domain.WithdrawalDirect,
			AllowInternalTransfers: true,
			AllowSubaccounts:       false,
			CreatedEpoch:           book.Epoch(),
			Metadata:               map[string]string{"rail": "wire"},
		},
	}
	for _, input := range accounts {
		account, err := domain.NewAccount(input)
		if err != nil {
			return nil, err
		}
		if err := book.RegisterAccount(account); err != nil {
			return nil, err
		}
	}
	return book, nil
}
