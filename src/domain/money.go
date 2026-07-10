package domain

import "fmt"

type Amount int64

const Zero Amount = 0

func NewAmount(value int64) (Amount, error) {
	amount := Amount(value)
	if err := amount.ValidateNonNegative("amount"); err != nil {
		return 0, err
	}
	return amount, nil
}

func MustAmount(value int64) Amount {
	amount, err := NewAmount(value)
	if err != nil {
		panic(err)
	}
	return amount
}

func (a Amount) Int64() int64 {
	return int64(a)
}

func (a Amount) IsZero() bool {
	return a == 0
}

func (a Amount) Positive() bool {
	return a > 0
}

func (a Amount) Negative() bool {
	return a < 0
}

func (a Amount) ValidatePositive(field string) error {
	if a <= 0 {
		return NewValidationError("amount.positive", field+" must be positive")
	}
	return nil
}

func (a Amount) ValidateNonNegative(field string) error {
	if a < 0 {
		return NewValidationError("amount.non_negative", field+" must be non-negative")
	}
	return nil
}

func (a Amount) Add(other Amount) (Amount, error) {
	if other > 0 && a > Amount(^uint64(0)>>1)-other {
		return 0, NewInvariantError("amount.overflow", "amount addition overflow")
	}
	if other < 0 && a < -Amount(^uint64(0)>>1)-1-other {
		return 0, NewInvariantError("amount.overflow", "amount addition underflow")
	}
	return a + other, nil
}

func (a Amount) Sub(other Amount) (Amount, error) {
	return a.Add(-other)
}

func (a Amount) Min(other Amount) Amount {
	if a < other {
		return a
	}
	return other
}

func (a Amount) Max(other Amount) Amount {
	if a > other {
		return a
	}
	return other
}

func (a Amount) String() string {
	return fmt.Sprintf("%d", a)
}

type Balance struct {
	Available  Amount `json:"available"`
	Reserved   Amount `json:"reserved"`
	Segregated Amount `json:"segregated"`
}

func NewBalance() Balance {
	return Balance{}
}

func (b Balance) Total() Amount {
	return b.Available + b.Reserved
}

func (b Balance) Spendable() Amount {
	return b.Available
}

func (b Balance) Validate() error {
	if err := b.Available.ValidateNonNegative("available"); err != nil {
		return err
	}
	if err := b.Reserved.ValidateNonNegative("reserved"); err != nil {
		return err
	}
	if err := b.Segregated.ValidateNonNegative("segregated"); err != nil {
		return err
	}
	if b.Segregated > b.Total() {
		return NewInvariantError("balance.segregation", "segregated balance cannot exceed total")
	}
	return nil
}

func (b Balance) Credit(amount Amount, segregated bool) (Balance, error) {
	if err := amount.ValidatePositive("credit amount"); err != nil {
		return b, err
	}
	next := b
	next.Available += amount
	if segregated {
		next.Segregated += amount
	}
	return next, next.Validate()
}

func (b Balance) Debit(amount Amount, segregated bool) (Balance, error) {
	if err := amount.ValidatePositive("debit amount"); err != nil {
		return b, err
	}
	if b.Available < amount {
		return b, NewInsufficientFundsError("balance.available", "available balance is lower than debit amount")
	}
	next := b
	next.Available -= amount
	if segregated {
		if next.Segregated < amount {
			return b, NewInvariantError("balance.segregation", "segregated balance would become negative")
		}
		next.Segregated -= amount
	}
	return next, next.Validate()
}

func (b Balance) Reserve(amount Amount) (Balance, error) {
	if err := amount.ValidatePositive("reserve amount"); err != nil {
		return b, err
	}
	if b.Available < amount {
		return b, NewInsufficientFundsError("balance.available", "available balance is lower than reserve amount")
	}
	next := b
	next.Available -= amount
	next.Reserved += amount
	return next, next.Validate()
}

func (b Balance) Release(amount Amount) (Balance, error) {
	if err := amount.ValidatePositive("release amount"); err != nil {
		return b, err
	}
	if b.Reserved < amount {
		return b, NewInsufficientFundsError("balance.reserved", "reserved balance is lower than release amount")
	}
	next := b
	next.Reserved -= amount
	next.Available += amount
	return next, next.Validate()
}

func (b Balance) MoveReservedOut(amount Amount, segregated bool) (Balance, error) {
	if err := amount.ValidatePositive("reserved debit amount"); err != nil {
		return b, err
	}
	if b.Reserved < amount {
		return b, NewInsufficientFundsError("balance.reserved", "reserved balance is lower than debit amount")
	}
	next := b
	next.Reserved -= amount
	if segregated {
		if next.Segregated < amount {
			return b, NewInvariantError("balance.segregation", "segregated reserve would become negative")
		}
		next.Segregated -= amount
	}
	return next, next.Validate()
}
