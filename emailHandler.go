package log15MandrillEmailer

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

type EmailHandler struct {
	Addresses        []string
	EmailsSent       int
	MaxEmailsPerHour int
	FromEmail        string
	FromName         string
	SubjectPrepend   string
	MandrillApiKey   string

	lastMessage string
	lock        *sync.Mutex
}

func (t EmailHandler) getPermissionToSendEmail() bool {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.EmailsSent > t.MaxEmailsPerHour {
		return false
	}
	t.EmailsSent++
	go func() {
		time.Sleep(time.Hour)
		t.lock.Lock()
		defer t.lock.Unlock()
		t.EmailsSent--
	}()
	return true
}

func (t EmailHandler) Log(r *log15.Record) error {
	if hasPermission := t.getPermissionToSendEmail(); !hasPermission {
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

	client := mandrill.ClientWithKey(t.MandrillApiKey)
	message := &mandrill.Message{
		FromEmail: t.FromEmail,
		FromName:  t.FromName,
		Subject:   strings.TrimSpace(t.SubjectPrepend + " " + r.Msg),
		Text:      strings.Join(textLst, "\n") + "\n\nStack:\n" + string(debug.Stack()),
	}

	for _, v := range t.Addresses {
		message.AddRecipient(v, "logging-recipient", "to")
	}

	if message.Text == t.lastMessage {
		return nil
	}
	t.lastMessage = message.Text
	_, err = client.MessagesSend(message)
	return err
}
