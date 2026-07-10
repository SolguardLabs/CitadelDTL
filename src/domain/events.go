package domain

type LedgerEvent struct {
	ID            EventID       `json:"id"`
	Epoch         Epoch         `json:"epoch"`
	Kind          MovementKind  `json:"kind"`
	From          AccountID     `json:"from,omitempty"`
	To            AccountID     `json:"to,omitempty"`
	Account       AccountID     `json:"account,omitempty"`
	Asset         Asset         `json:"asset"`
	Amount        Amount        `json:"amount"`
	MandateID     MandateID     `json:"mandate_id,omitempty"`
	SegregationID SegregationID `json:"segregation_id,omitempty"`
	Reference     string        `json:"reference,omitempty"`
	Message       string        `json:"message,omitempty"`
}

type Receipt struct {
	ID            ReceiptID     `json:"id"`
	Epoch         Epoch         `json:"epoch"`
	Kind          MovementKind  `json:"kind"`
	Account       AccountID     `json:"account"`
	Destination   string        `json:"destination,omitempty"`
	Asset         Asset         `json:"asset"`
	Amount        Amount        `json:"amount"`
	Fee           Amount        `json:"fee"`
	MandateID     MandateID     `json:"mandate_id,omitempty"`
	SegregationID SegregationID `json:"segregation_id,omitempty"`
	Status        ReceiptStatus `json:"status"`
	Reason        string        `json:"reason,omitempty"`
}

type AuditIssue struct {
	Code           string        `json:"code"`
	Severity       IssueSeverity `json:"severity"`
	Account        AccountID     `json:"account,omitempty"`
	MandateID      MandateID     `json:"mandate_id,omitempty"`
	Asset          Asset         `json:"asset,omitempty"`
	Amount         Amount        `json:"amount,omitempty"`
	Message        string        `json:"message"`
	Recommendation string        `json:"recommendation,omitempty"`
}

type ActionResult struct {
	Label     string                 `json:"label"`
	Type      string                 `json:"type"`
	Status    ResultStatus           `json:"status"`
	Account   AccountID              `json:"account,omitempty"`
	MandateID MandateID              `json:"mandate_id,omitempty"`
	ReceiptID ReceiptID              `json:"receipt_id,omitempty"`
	Reason    string                 `json:"reason,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

func Accepted(label string, typ string) ActionResult {
	return ActionResult{Label: label, Type: typ, Status: ResultAccepted}
}

func Rejected(label string, typ string, err error) ActionResult {
	return ActionResult{Label: label, Type: typ, Status: ResultRejected, Reason: ErrorReason(err), Message: errorMessage(err)}
}

func Failed(label string, typ string, err error) ActionResult {
	return ActionResult{Label: label, Type: typ, Status: ResultFailed, Reason: ErrorReason(err), Message: errorMessage(err)}
}

func (r ActionResult) WithAccount(account AccountID) ActionResult {
	r.Account = account
	return r
}

func (r ActionResult) WithMandate(id MandateID) ActionResult {
	r.MandateID = id
	return r
}

func (r ActionResult) WithReceipt(id ReceiptID) ActionResult {
	r.ReceiptID = id
	return r
}

func (r ActionResult) WithDetail(key string, value interface{}) ActionResult {
	if r.Details == nil {
		r.Details = map[string]interface{}{}
	}
	r.Details[key] = value
	return r
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	if app, ok := err.(*AppError); ok {
		return app.Message
	}
	return err.Error()
}
