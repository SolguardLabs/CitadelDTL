package ledger

import (
	"fmt"
	"sort"

	"citadeldtl/src/domain"
)

type balanceKey struct {
	account domain.AccountID
	asset   domain.Asset
}

type Book struct {
	epoch       domain.Epoch
	accounts    map[domain.AccountID]domain.Account
	balances    map[balanceKey]domain.Balance
	events      []domain.LedgerEvent
	nextEventID int64
}

func NewBook() *Book {
	return &Book{
		epoch:       1,
		accounts:    map[domain.AccountID]domain.Account{},
		balances:    map[balanceKey]domain.Balance{},
		events:      []domain.LedgerEvent{},
		nextEventID: 1,
	}
}

func (b *Book) Epoch() domain.Epoch {
	return b.epoch
}

func (b *Book) AdvanceEpoch() domain.Epoch {
	b.epoch++
	b.emit(domain.LedgerEvent{
		Kind:    domain.MovementReserveAdjustment,
		Asset:   domain.AssetUSDC,
		Message: "epoch advanced",
	})
	return b.epoch
}

func (b *Book) RegisterAccount(account domain.Account) error {
	if err := account.Validate(); err != nil {
		return err
	}
	if _, exists := b.accounts[account.ID]; exists {
		return domain.NewConflictError("account.exists", fmt.Sprintf("account %s already exists", account.ID))
	}
	if !account.ParentID.Empty() {
		parent, ok := b.accounts[account.ParentID]
		if !ok {
			return domain.NewNotFoundError("account.parent", fmt.Sprintf("parent account %s not found", account.ParentID))
		}
		if account.RootID.Empty() || account.RootID == account.ParentID {
			account.RootID = parent.LineageRoot()
		}
		if account.SegregationID.Empty() {
			account.SegregationID = parent.SegregationID
		}
	}
	b.accounts[account.ID] = account.Clone()
	b.emit(domain.LedgerEvent{
		Kind:          domain.MovementOperationalCreation,
		Account:       account.ID,
		Asset:         domain.AssetUSDC,
		MandateID:     account.MandateID,
		SegregationID: account.SegregationID,
		Message:       "account registered",
	})
	return nil
}

func (b *Book) ReplaceAccount(account domain.Account) error {
	if err := account.Validate(); err != nil {
		return err
	}
	if _, exists := b.accounts[account.ID]; !exists {
		return domain.NewNotFoundError("account.missing", fmt.Sprintf("account %s not found", account.ID))
	}
	b.accounts[account.ID] = account.Clone()
	return nil
}

func (b *Book) Account(id domain.AccountID) (domain.Account, bool) {
	account, ok := b.accounts[id]
	if !ok {
		return domain.Account{}, false
	}
	return account.Clone(), true
}

func (b *Book) MustAccount(id domain.AccountID) (domain.Account, error) {
	account, ok := b.Account(id)
	if !ok {
		return domain.Account{}, domain.NewNotFoundError("account.missing", fmt.Sprintf("account %s not found", id))
	}
	return account, nil
}

func (b *Book) Accounts() []domain.Account {
	ids := make([]string, 0, len(b.accounts))
	for id := range b.accounts {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	out := make([]domain.Account, 0, len(ids))
	for _, id := range ids {
		out = append(out, b.accounts[domain.AccountID(id)].Clone())
	}
	return out
}

func (b *Book) ParentOf(id domain.AccountID) (domain.Account, bool) {
	account, ok := b.Account(id)
	if !ok || account.ParentID.Empty() {
		return domain.Account{}, false
	}
	return b.Account(account.ParentID)
}

func (b *Book) Ancestors(id domain.AccountID) []domain.Account {
	out := []domain.Account{}
	seen := map[domain.AccountID]bool{}
	current, ok := b.Account(id)
	for ok && !current.ParentID.Empty() {
		if seen[current.ParentID] {
			break
		}
		seen[current.ParentID] = true
		parent, parentOK := b.Account(current.ParentID)
		if !parentOK {
			break
		}
		out = append(out, parent)
		current = parent
	}
	return out
}

func (b *Book) Descendants(id domain.AccountID) []domain.Account {
	out := []domain.Account{}
	for _, account := range b.Accounts() {
		if account.ParentID == id {
			out = append(out, account)
			out = append(out, b.Descendants(account.ID)...)
		}
	}
	return out
}

func (b *Book) BalanceOf(account domain.AccountID, asset domain.Asset) domain.Balance {
	key := balanceKey{account: account, asset: asset.Normalize()}
	return b.balances[key]
}

func (b *Book) setBalance(account domain.AccountID, asset domain.Asset, balance domain.Balance) error {
	if err := balance.Validate(); err != nil {
		return err
	}
	key := balanceKey{account: account, asset: asset.Normalize()}
	if balance.Total() == 0 && balance.Segregated == 0 {
		delete(b.balances, key)
		return nil
	}
	b.balances[key] = balance
	return nil
}

func (b *Book) Credit(accountID domain.AccountID, asset domain.Asset, amount domain.Amount, reference string, segregated bool) error {
	account, err := b.MustAccount(accountID)
	if err != nil {
		return err
	}
	asset = asset.Normalize()
	if err := asset.Validate(); err != nil {
		return err
	}
	current := b.BalanceOf(accountID, asset)
	next, err := current.Credit(amount, segregated)
	if err != nil {
		return err
	}
	if err := b.setBalance(accountID, asset, next); err != nil {
		return err
	}
	b.emit(domain.LedgerEvent{
		Kind:          domain.MovementDeposit,
		Account:       accountID,
		To:            accountID,
		Asset:         asset,
		Amount:        amount,
		MandateID:     account.MandateID,
		SegregationID: account.SegregationID,
		Reference:     reference,
		Message:       "credit applied",
	})
	return nil
}

func (b *Book) Debit(accountID domain.AccountID, asset domain.Asset, amount domain.Amount, reference string, segregated bool, kind domain.MovementKind) error {
	account, err := b.MustAccount(accountID)
	if err != nil {
		return err
	}
	asset = asset.Normalize()
	if err := asset.Validate(); err != nil {
		return err
	}
	current := b.BalanceOf(accountID, asset)
	next, err := current.Debit(amount, segregated)
	if err != nil {
		return err
	}
	if err := b.setBalance(accountID, asset, next); err != nil {
		return err
	}
	b.emit(domain.LedgerEvent{
		Kind:          kind,
		Account:       accountID,
		From:          accountID,
		Asset:         asset,
		Amount:        amount,
		MandateID:     account.MandateID,
		SegregationID: account.SegregationID,
		Reference:     reference,
		Message:       "debit applied",
	})
	return nil
}

func (b *Book) Transfer(from domain.AccountID, to domain.AccountID, asset domain.Asset, amount domain.Amount, reference string, segregated bool, kind domain.MovementKind) error {
	fromAccount, err := b.MustAccount(from)
	if err != nil {
		return err
	}
	toAccount, err := b.MustAccount(to)
	if err != nil {
		return err
	}
	asset = asset.Normalize()
	if err := asset.Validate(); err != nil {
		return err
	}
	fromBalance := b.BalanceOf(from, asset)
	nextFrom, err := fromBalance.Debit(amount, segregated)
	if err != nil {
		return err
	}
	toBalance := b.BalanceOf(to, asset)
	nextTo, err := toBalance.Credit(amount, segregated)
	if err != nil {
		return err
	}
	if err := b.setBalance(from, asset, nextFrom); err != nil {
		return err
	}
	if err := b.setBalance(to, asset, nextTo); err != nil {
		return err
	}
	mandateID := fromAccount.MandateID
	if mandateID.Empty() {
		mandateID = toAccount.MandateID
	}
	segID := fromAccount.SegregationID
	if segID.Empty() {
		segID = toAccount.SegregationID
	}
	b.emit(domain.LedgerEvent{
		Kind:          kind,
		From:          from,
		To:            to,
		Asset:         asset,
		Amount:        amount,
		MandateID:     mandateID,
		SegregationID: segID,
		Reference:     reference,
		Message:       "transfer applied",
	})
	return nil
}

func (b *Book) Reserve(accountID domain.AccountID, asset domain.Asset, amount domain.Amount, reference string) error {
	account, err := b.MustAccount(accountID)
	if err != nil {
		return err
	}
	current := b.BalanceOf(accountID, asset)
	next, err := current.Reserve(amount)
	if err != nil {
		return err
	}
	if err := b.setBalance(accountID, asset, next); err != nil {
		return err
	}
	b.emit(domain.LedgerEvent{
		Kind:          domain.MovementReserveAdjustment,
		Account:       accountID,
		Asset:         asset.Normalize(),
		Amount:        amount,
		MandateID:     account.MandateID,
		SegregationID: account.SegregationID,
		Reference:     reference,
		Message:       "balance reserved",
	})
	return nil
}

func (b *Book) Release(accountID domain.AccountID, asset domain.Asset, amount domain.Amount, reference string) error {
	account, err := b.MustAccount(accountID)
	if err != nil {
		return err
	}
	current := b.BalanceOf(accountID, asset)
	next, err := current.Release(amount)
	if err != nil {
		return err
	}
	if err := b.setBalance(accountID, asset, next); err != nil {
		return err
	}
	b.emit(domain.LedgerEvent{
		Kind:          domain.MovementReserveAdjustment,
		Account:       accountID,
		Asset:         asset.Normalize(),
		Amount:        amount,
		MandateID:     account.MandateID,
		SegregationID: account.SegregationID,
		Reference:     reference,
		Message:       "balance released",
	})
	return nil
}

func (b *Book) FindReserveAccount(segregationID domain.SegregationID, asset domain.Asset) (domain.Account, bool) {
	for _, account := range b.Accounts() {
		if account.Type == domain.AccountReserve && account.SegregationID == segregationID {
			if _, ok := b.balances[balanceKey{account: account.ID, asset: asset.Normalize()}]; ok {
				return account, true
			}
			return account, true
		}
	}
	return domain.Account{}, false
}

func (b *Book) Events() []domain.LedgerEvent {
	out := make([]domain.LedgerEvent, len(b.events))
	copy(out, b.events)
	return out
}

func (b *Book) BalanceLines() []domain.BalanceLine {
	keys := make([]balanceKey, 0, len(b.balances))
	for key := range b.balances {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].account == keys[j].account {
			return keys[i].asset < keys[j].asset
		}
		return keys[i].account < keys[j].account
	})
	lines := make([]domain.BalanceLine, 0, len(keys))
	for _, key := range keys {
		account := b.accounts[key.account]
		balance := b.balances[key]
		lines = append(lines, domain.BalanceLine{
			Account:       key.account,
			Owner:         account.Owner,
			Type:          account.Type,
			Asset:         key.asset,
			Available:     balance.Available,
			Reserved:      balance.Reserved,
			Segregated:    balance.Segregated,
			MandateID:     account.MandateID,
			SegregationID: account.SegregationID,
		})
	}
	return lines
}

func (b *Book) AccountLines() []domain.AccountLine {
	accounts := b.Accounts()
	lines := make([]domain.AccountLine, 0, len(accounts))
	for _, account := range accounts {
		lines = append(lines, domain.NewAccountLine(account))
	}
	return lines
}

func (b *Book) ReserveLines() []domain.ReserveLine {
	type reserveKey struct {
		segregation domain.SegregationID
		asset       domain.Asset
	}
	custody := map[reserveKey]domain.Amount{}
	reserve := map[reserveKey]domain.Amount{}
	for _, line := range b.BalanceLines() {
		key := reserveKey{segregation: line.SegregationID, asset: line.Asset}
		switch line.Type {
		case domain.AccountInstitutional, domain.AccountDelegated, domain.AccountOperational:
			custody[key] += line.Segregated
		case domain.AccountReserve:
			reserve[key] += line.Segregated
		}
	}
	keys := map[reserveKey]bool{}
	for key := range custody {
		keys[key] = true
	}
	for key := range reserve {
		keys[key] = true
	}
	ordered := make([]reserveKey, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].segregation == ordered[j].segregation {
			return ordered[i].asset < ordered[j].asset
		}
		return ordered[i].segregation < ordered[j].segregation
	})
	lines := make([]domain.ReserveLine, 0, len(ordered))
	for _, key := range ordered {
		lines = append(lines, domain.ReserveLine{
			SegregationID: key.segregation,
			Asset:         key.asset,
			CustodyTotal:  custody[key],
			ReserveTotal:  reserve[key],
			Delta:         reserve[key] - custody[key],
		})
	}
	return lines
}

func (b *Book) TotalByType(asset domain.Asset, typ domain.AccountType) domain.Amount {
	total := domain.Amount(0)
	for _, line := range b.BalanceLines() {
		if line.Asset == asset.Normalize() && line.Type == typ {
			total += line.Available + line.Reserved
		}
	}
	return total
}

func (b *Book) SegregatedTotal(asset domain.Asset, typ domain.AccountType) domain.Amount {
	total := domain.Amount(0)
	for _, line := range b.BalanceLines() {
		if line.Asset == asset.Normalize() && line.Type == typ {
			total += line.Segregated
		}
	}
	return total
}

func (b *Book) emit(event domain.LedgerEvent) domain.LedgerEvent {
	event.ID = domain.EventID(fmt.Sprintf("evt-%06d", b.nextEventID))
	event.Epoch = b.epoch
	b.nextEventID++
	b.events = append(b.events, event)
	return event
}
