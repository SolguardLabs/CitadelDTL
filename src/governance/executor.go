package governance

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

type OperationState string

const (
	StateQueued    OperationState = "queued"
	StateExecuted  OperationState = "executed"
	StateCancelled OperationState = "cancelled"
)

type Operation struct {
	Protocol     string `json:"protocol"`
	Network      string `json:"network"`
	Target       string `json:"target"`
	Method       string `json:"method"`
	PayloadHash  string `json:"payload_hash"`
	Predecessor  string `json:"predecessor,omitempty"`
	Salt         string `json:"salt"`
	ExecuteAfter uint64 `json:"execute_after"`
}

type Record struct {
	ID          string         `json:"id"`
	Operation   Operation      `json:"operation"`
	State       OperationState `json:"state"`
	Proposer    string         `json:"proposer"`
	Approvals   []string       `json:"approvals"`
	ExpiresAt   uint64         `json:"expires_at"`
	ExecutedAt  uint64         `json:"executed_at,omitempty"`
	CancelledBy string         `json:"cancelled_by,omitempty"`
}

type storedRecord struct {
	record    Record
	approvals map[string]struct{}
}

type Executor struct {
	mu        sync.RWMutex
	governors map[string]struct{}
	guardian  string
	quorum    int
	delay     uint64
	grace     uint64
	records   map[string]*storedRecord
}

func NewExecutor(governors []string, quorum int, delay uint64, grace uint64, guardian string) (*Executor, error) {
	if len(governors) == 0 {
		return nil, fmt.Errorf("governance: at least one governor is required")
	}
	if delay == 0 || grace == 0 {
		return nil, fmt.Errorf("governance: delay and grace period must be positive")
	}
	if strings.TrimSpace(guardian) == "" {
		return nil, fmt.Errorf("governance: guardian is required")
	}
	set := make(map[string]struct{}, len(governors))
	for _, governor := range governors {
		normalized := strings.TrimSpace(governor)
		if normalized == "" {
			return nil, fmt.Errorf("governance: governor is empty")
		}
		if _, exists := set[normalized]; exists {
			return nil, fmt.Errorf("governance: duplicate governor %q", normalized)
		}
		set[normalized] = struct{}{}
	}
	if quorum < 1 || quorum > len(set) {
		return nil, fmt.Errorf("governance: quorum must be within [1, %d]", len(set))
	}
	return &Executor{
		governors: set,
		guardian:  strings.TrimSpace(guardian),
		quorum:    quorum,
		delay:     delay,
		grace:     grace,
		records:   make(map[string]*storedRecord),
	}, nil
}

func OperationID(operation Operation) (string, error) {
	if err := validateOperation(operation); err != nil {
		return "", err
	}
	hasher := sha256.New()
	fields := []string{
		operation.Protocol,
		operation.Network,
		operation.Target,
		operation.Method,
		strings.ToLower(operation.PayloadHash),
		strings.ToLower(operation.Predecessor),
		operation.Salt,
	}
	for _, field := range fields {
		length := make([]byte, 8)
		binary.BigEndian.PutUint64(length, uint64(len(field)))
		_, _ = hasher.Write(length)
		_, _ = hasher.Write([]byte(field))
	}
	epoch := make([]byte, 8)
	binary.BigEndian.PutUint64(epoch, operation.ExecuteAfter)
	_, _ = hasher.Write(epoch)
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (e *Executor) Queue(operation Operation, proposer string, now uint64) (Record, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	proposer = strings.TrimSpace(proposer)
	if !e.isGovernor(proposer) {
		return Record{}, fmt.Errorf("governance: proposer is not an active governor")
	}
	minimum, err := safeAdd(now, e.delay)
	if err != nil {
		return Record{}, err
	}
	if operation.ExecuteAfter < minimum {
		return Record{}, fmt.Errorf("governance: execution epoch must be at least %d", minimum)
	}
	expiresAt, err := safeAdd(operation.ExecuteAfter, e.grace)
	if err != nil {
		return Record{}, err
	}
	id, err := OperationID(operation)
	if err != nil {
		return Record{}, err
	}
	if _, exists := e.records[id]; exists {
		return Record{}, fmt.Errorf("governance: operation %s already exists", id)
	}
	stored := &storedRecord{
		record:    Record{ID: id, Operation: operation, State: StateQueued, Proposer: proposer, ExpiresAt: expiresAt},
		approvals: map[string]struct{}{proposer: {}},
	}
	e.records[id] = stored
	return copyRecord(stored), nil
}

func (e *Executor) Approve(id string, governor string, now uint64) (Record, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	governor = strings.TrimSpace(governor)
	if !e.isGovernor(governor) {
		return Record{}, fmt.Errorf("governance: approver is not an active governor")
	}
	stored, err := e.mutableQueued(id, now)
	if err != nil {
		return Record{}, err
	}
	if _, exists := stored.approvals[governor]; exists {
		return Record{}, fmt.Errorf("governance: governor %s already approved operation", governor)
	}
	stored.approvals[governor] = struct{}{}
	return copyRecord(stored), nil
}

func (e *Executor) Execute(id string, caller string, now uint64) (Record, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	caller = strings.TrimSpace(caller)
	if !e.isGovernor(caller) {
		return Record{}, fmt.Errorf("governance: executor is not an active governor")
	}
	stored, err := e.mutableQueued(id, now)
	if err != nil {
		return Record{}, err
	}
	if now < stored.record.Operation.ExecuteAfter {
		return Record{}, fmt.Errorf("governance: operation is timelocked until %d", stored.record.Operation.ExecuteAfter)
	}
	if len(stored.approvals) < e.quorum {
		return Record{}, fmt.Errorf("governance: operation has %d approvals and requires %d", len(stored.approvals), e.quorum)
	}
	if predecessor := stored.record.Operation.Predecessor; predecessor != "" {
		parent, exists := e.records[strings.ToLower(predecessor)]
		if !exists || parent.record.State != StateExecuted {
			return Record{}, fmt.Errorf("governance: predecessor %s is not executed", predecessor)
		}
	}
	stored.record.State = StateExecuted
	stored.record.ExecutedAt = now
	return copyRecord(stored), nil
}

func (e *Executor) Cancel(id string, guardian string, now uint64) (Record, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	guardian = strings.TrimSpace(guardian)
	if guardian != e.guardian {
		return Record{}, fmt.Errorf("governance: cancellation requires the configured guardian")
	}
	stored, err := e.mutableQueued(id, now)
	if err != nil {
		return Record{}, err
	}
	stored.record.State = StateCancelled
	stored.record.CancelledBy = guardian
	return copyRecord(stored), nil
}

func (e *Executor) Record(id string) (Record, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	stored, exists := e.records[strings.ToLower(strings.TrimSpace(id))]
	if !exists {
		return Record{}, false
	}
	return copyRecord(stored), true
}

func (e *Executor) Records() []Record {
	e.mu.RLock()
	defer e.mu.RUnlock()
	records := make([]Record, 0, len(e.records))
	for _, stored := range e.records {
		records = append(records, copyRecord(stored))
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records
}

func (e *Executor) mutableQueued(id string, now uint64) (*storedRecord, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	stored, exists := e.records[id]
	if !exists {
		return nil, fmt.Errorf("governance: operation %s does not exist", id)
	}
	if stored.record.State != StateQueued {
		return nil, fmt.Errorf("governance: operation %s is %s", id, stored.record.State)
	}
	if now > stored.record.ExpiresAt {
		return nil, fmt.Errorf("governance: operation %s expired at %d", id, stored.record.ExpiresAt)
	}
	return stored, nil
}

func (e *Executor) isGovernor(account string) bool {
	_, exists := e.governors[account]
	return exists
}

func validateOperation(operation Operation) error {
	values := map[string]string{
		"protocol": operation.Protocol,
		"network":  operation.Network,
		"target":   operation.Target,
		"method":   operation.Method,
		"salt":     operation.Salt,
	}
	for field, value := range values {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return fmt.Errorf("governance: %s must be non-empty and normalized", field)
		}
	}
	if !isHash(operation.PayloadHash) {
		return fmt.Errorf("governance: payload hash must be 32-byte hexadecimal")
	}
	if operation.Predecessor != "" && !isHash(operation.Predecessor) {
		return fmt.Errorf("governance: predecessor must be a 32-byte hexadecimal operation id")
	}
	return nil
}

func isHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func copyRecord(stored *storedRecord) Record {
	record := stored.record
	record.Approvals = make([]string, 0, len(stored.approvals))
	for governor := range stored.approvals {
		record.Approvals = append(record.Approvals, governor)
	}
	sort.Strings(record.Approvals)
	return record
}

func safeAdd(a uint64, b uint64) (uint64, error) {
	if math.MaxUint64-a < b {
		return 0, fmt.Errorf("governance: epoch arithmetic exceeds uint64")
	}
	return a + b, nil
}
