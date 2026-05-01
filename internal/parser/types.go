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
	direction string
	regex     *regexp.Regexp
}
