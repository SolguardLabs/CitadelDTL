package scenario

import "citadeldtl/src/domain"

type Fixture struct {
	Name    string   `json:"name"`
	Actions []Action `json:"actions"`
}

type Action struct {
	Label                       string            `json:"label"`
	Type                        string            `json:"type"`
	Account                     domain.AccountID  `json:"account"`
	Parent                      domain.AccountID  `json:"parent"`
	Child                       domain.AccountID  `json:"child"`
	From                        domain.AccountID  `json:"from"`
	To                          domain.AccountID  `json:"to"`
	Owner                       string            `json:"owner"`
	Asset                       domain.Asset      `json:"asset"`
	Amount                      domain.Amount     `json:"amount"`
	Destination                 string            `json:"destination"`
	Reference                   string            `json:"reference"`
	MandateID                   domain.MandateID  `json:"mandate_id"`
	Delegate                    domain.AccountID  `json:"delegate"`
	DailyLimit                  domain.Amount     `json:"daily_limit"`
	WithdrawalLimit             domain.Amount     `json:"withdrawal_limit"`
	SettlementLimit             domain.Amount     `json:"settlement_limit"`
	DirectWithdrawal            bool              `json:"direct_withdrawal"`
	AllowInternalTransfers      bool              `json:"allow_internal_transfers"`
	AllowOperationalSubaccounts bool              `json:"allow_operational_subaccounts"`
	Metadata                    map[string]string `json:"metadata"`
}

type Result struct {
	Name     string                `json:"name"`
	Results  []domain.ActionResult `json:"results"`
	Snapshot domain.Snapshot       `json:"snapshot"`
}

func BuiltinScenarios() []string {
	return []string{
		"deposits",
		"mandate_allocation",
		"withdrawal_controls",
		"settlement_audit",
	}
}
