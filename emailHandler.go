package log15Emailer

import (
	"encoding/json"
	"fmt"
	"github.com/keighl/mandrill"
	"gopkg.in/inconshreveable/log15.v2"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Creates a new EmailHandler
func NewEmailHandler(addresses []string, fromEmailAddress, fromName, subjectPrepend, mandrillApiKey string) *EmailHandler {
	return &EmailHandler{
		MaxEmailsPerHour: 100,

		addresses:      addresses,
		fromEmail:      fromEmailAddress,
		fromName:       fromName,
		subjectPrepend: subjectPrepend,
		mandrillApiKey: mandrillApiKey,

		lock: &sync.Mutex{},
	}
}

// The handler which should be passed in as a log15 handler.
// This struct should be created via NewEmailHandler
type EmailHandler struct {
	// The maximum number of emails that can be sent in an hour.
	// NewEmailHandler will set this to 100 by default.
	MaxEmailsPerHour int

	addresses      []string
	fromEmail      string
	fromName       string
	subjectPrepend string
	mandrillApiKey string

	emailsSent  int
	lastMessage string
	lock        *sync.Mutex
}

func (handler *EmailHandler) getPermissionToSendEmail() bool {
	handler.lock.Lock()
	defer handler.lock.Unlock()
	if handler.emailsSent > handler.MaxEmailsPerHour {
		return false
	}
	handler.emailsSent++
	go func() {
		time.Sleep(time.Hour)
		handler.lock.Lock()
		defer handler.lock.Unlock()
		handler.emailsSent--
	}()
	return true
}

// The Log function fulfills the Log15 logger interface.
func (handler *EmailHandler) Log(r *log15.Record) error {
	if hasPermission := handler.getPermissionToSendEmail(); !hasPermission {
		return nil
	}
	var textLst = make([]string, 0)
	textLst = append(textLst, "Message: "+r.Msg)
	textLst = append(textLst, fmt.Sprint("LogLevel: ", r.Lvl))
	textLst = append(textLst, "Time (UTC): "+r.Time.UTC().Format(time.RFC3339))
	textLst = append(textLst, "\nCTX:")
	for i := 1; i < len(r.Ctx); i += 2 {
		textLst = append(textLst, fmt.Sprint(r.Ctx[i-1], ": ", r.Ctx[i]))
	}

	rawText, err := json.Marshal(r)
	if err != nil {
		return err
	}
	textLst = append(textLst, ("\nRaw:\n" + string(rawText)))

	client := mandrill.ClientWithKey(handler.mandrillApiKey)
	message := &mandrill.Message{
		FromEmail: handler.fromEmail,
		FromName:  handler.fromName,
		Subject:   strings.TrimSpace(handler.subjectPrepend + " " + r.Msg),
		Text:      strings.Join(textLst, "\n") + "\n\nStack:\n" + string(debug.Stack()),
	}

	for _, v := range handler.addresses {
		message.AddRecipient(v, "logging-recipient", "to")
	}

	if message.Text == handler.lastMessage {
		return nil
	}
	handler.lastMessage = message.Text
	_, err = client.MessagesSend(message)
	return err
}
