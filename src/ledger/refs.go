package ledger

import "citadeldtl/src/domain"

const (
	DefaultInstitutionalAccount domain.AccountID     = "inst-cold"
	DefaultHotAccount           domain.AccountID     = "inst-hot"
	DefaultReserveAccount       domain.AccountID     = "reserve-usdc-main"
	DefaultSettlementAccount    domain.AccountID     = "settlement-usdc"
	DefaultExternalAccount      domain.AccountID     = "external-bank"
	DefaultSegregationID        domain.SegregationID = "seg-main"
)
