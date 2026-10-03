package email

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"github.com/paytrack/backend/internal/config"
)

// FetchedEmail represents a downloaded email payload ready for analysis
type FetchedEmail struct {
	UID       uint32
	MessageID string
	Sender    string
	Subject   string
	Date      time.Time
	Body      string
}

// IMAPClient handles secure connection and fetching from IMAP servers
type IMAPClient struct {
	cfg *config.Config
}

func NewIMAPClient(cfg *config.Config) *IMAPClient {
	return &IMAPClient{cfg: cfg}
}

// FetchEmailsForCredentials connects to IMAP with specific credentials and retrieves candidate payment emails
func (ic *IMAPClient) FetchEmailsForCredentials(server, username, password, folder string, batchSize, lookbackDays int) ([]FetchedEmail, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("IMAP credentials cannot be empty")
	}
	if folder == "" {
		folder = "INBOX"
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	if lookbackDays <= 0 {
		lookbackDays = 7
	}

	log.Printf("[IMAP] Connecting to %s for %s ...\n", server, username)

	serverHost := strings.Split(server, ":")[0]
	tlsConfig := &tls.Config{
		ServerName: serverHost,
	}

	c, err := client.DialTLS(server, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP server (%s): %w", server, err)
	}
	defer c.Logout()

	if err := c.Login(username, password); err != nil {
		return nil, fmt.Errorf("IMAP login failed for %s: %w", username, err)
	}
	log.Printf("[IMAP] Logged in successfully as %s\n", username)

	mbox, err := c.Select(folder, true) // Read-only mode
	if err != nil {
		return nil, fmt.Errorf("failed to select folder %s: %w", folder, err)
	}

	if mbox.Messages == 0 {
		log.Printf("[IMAP] Mailbox %s is empty for %s.\n", folder, username)
		return []FetchedEmail{}, nil
	}

	criteria := imap.NewSearchCriteria()
	criteria.Since = time.Now().AddDate(0, 0, -lookbackDays)

	uids, err := c.UidSearch(criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}

	if len(uids) == 0 {
		return []FetchedEmail{}, nil
	}

	if len(uids) > batchSize {
		uids = uids[len(uids)-batchSize:]
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uids...)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{
		imap.FetchEnvelope,
		imap.FetchUid,
		imap.FetchInternalDate,
		section.FetchItem(),
	}

	messages := make(chan *imap.Message, len(uids))
	done := make(chan error, 1)

	go func() {
		done <- c.UidFetch(seqSet, items, messages)
	}()

	var fetchedList []FetchedEmail

	for msg := range messages {
		if msg == nil {
			continue
		}

		email := ic.parseMessage(msg, section)
		if IsLikelyPaymentEmail(email.Subject, email.Sender, email.Body) {
			fetchedList = append(fetchedList, email)
		}
	}

	if err := <-done; err != nil {
		return fetchedList, fmt.Errorf("error during fetch: %w", err)
	}

	log.Printf("[IMAP] Account %s: Processed %d messages, filtered %d payment candidates\n", username, len(uids), len(fetchedList))
	return fetchedList, nil
}

// FetchRecentEmails fetches using default config credentials (fallback for single-user mode)
func (ic *IMAPClient) FetchRecentEmails() ([]FetchedEmail, error) {
	return ic.FetchEmailsForCredentials(
		ic.cfg.IMAPServer,
		ic.cfg.IMAPUsername,
		ic.cfg.IMAPPassword,
		ic.cfg.IMAPFolder,
		ic.cfg.FetchBatchSize,
		ic.cfg.LookbackDays,
	)
}

func (ic *IMAPClient) parseMessage(msg *imap.Message, section *imap.BodySectionName) FetchedEmail {
	email := FetchedEmail{
		UID:  msg.Uid,
		Date: msg.InternalDate,
	}

	if msg.Envelope != nil {
		email.Subject = msg.Envelope.Subject
		email.MessageID = msg.Envelope.MessageId
		if len(msg.Envelope.From) > 0 {
			from := msg.Envelope.From[0]
			if from.PersonalName != "" {
				email.Sender = fmt.Sprintf("%s <%s@%s>", from.PersonalName, from.MailboxName, from.HostName)
			} else {
				email.Sender = fmt.Sprintf("%s@%s", from.MailboxName, from.HostName)
			}
		}
		if !msg.Envelope.Date.IsZero() {
			email.Date = msg.Envelope.Date
		}
	}

	r := msg.GetBody(section)
	if r == nil {
		return email
	}

	mr, err := mail.CreateReader(r)
	if err != nil {
		raw := ReadAllString(r)
		email.Body = ExtractTextFromHTML(raw)
		return email
	}

	var textParts []string
	var htmlParts []string

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			break
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			contentType := h.Get("Content-Type")
			bodyBytes, _ := io.ReadAll(p.Body)
			bodyStr := string(bodyBytes)

			if strings.HasPrefix(contentType, "text/plain") {
				textParts = append(textParts, bodyStr)
			} else if strings.HasPrefix(contentType, "text/html") {
				htmlParts = append(htmlParts, bodyStr)
			}
		}
	}

	if len(textParts) > 0 {
		email.Body = cleanWhitespace(strings.Join(textParts, " \n "))
	} else if len(htmlParts) > 0 {
		email.Body = ExtractTextFromHTML(strings.Join(htmlParts, " \n "))
	} else {
		email.Body = cleanWhitespace(ReadAllString(r))
	}

	return email
}
