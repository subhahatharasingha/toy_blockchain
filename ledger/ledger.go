package ledger

type Ledger struct {
	Balances map[string]float64
}

func NewLedger() *Ledger {
	return &Ledger{
		Balances: make(map[string]float64),
	}
}

// ApplyTransaction updates balances
func (l *Ledger) ApplyTransaction(sender, receiver string, amount float64) {

	if sender != "" {
		l.Balances[sender] -= amount
	}

	if receiver != "" {
		l.Balances[receiver] += amount
	}
}

// GetBalance returns user balance
func (l *Ledger) GetBalance(user string) float64 {
	return l.Balances[user]
}