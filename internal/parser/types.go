package parser

import "regexp"

type SMSPayload struct {
	Sender     string `json:"sender"`
	Message    string `json:"message"`
	ReceivedAt string `json:"received_at"`
	DeviceID   string `json:"device_id"`
}

type ParsedTransaction struct {
	Sender      string
	ReceivedAt  string
	DeviceID    string
	Description string
	AccountMask string
	Amount      string
	Direction   string
	BankName    string
}

type Direction string

const (
	DirectionCredit Direction = "credit"
	DirectionDebit  Direction = "debit"
)

func (d Direction) Valid() bool {
	switch d {
	case DirectionCredit, DirectionDebit:
		return true
	default:
		return false
	}
}

type BankMatcher struct {
	name        string
	senderRegex []*regexp.Regexp
	messages    []messageMatcher
}

func (m BankMatcher) Name() string {
	return m.name
}

type messageMatcher struct {
	name      string
	direction Direction
	regex     *regexp.Regexp
}
