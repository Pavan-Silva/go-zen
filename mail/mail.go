package mail

import (
	"bytes"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Config holds SMTP configuration for sending emails.
type Config struct {
	Host     string // SMTP server host (e.g., "smtp.gmail.com")
	Port     int    // SMTP server port (e.g., 587)
	Username string // SMTP username
	Password string // SMTP password
}

// addr returns the host:port string for the SMTP server.
func (c Config) addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Message represents an email message.
type Message struct {
	From    string   // Sender email address
	To      []string // Primary recipients
	CC      []string // Carbon copy recipients
	BCC     []string // Blind carbon copy recipients
	ReplyTo string   // Reply-to address (optional)
	Subject string   // Email subject
	Body    string   // Email body (plain text or HTML depending on send method)
}

// allRecipients returns the full list of envelope recipients (To + CC + BCC).
func (m Message) allRecipients() []string {
	all := make([]string, 0, len(m.To)+len(m.CC)+len(m.BCC))
	return append(all, m.To...)
}

// Dialer holds a reusable SMTP configuration for sending multiple emails.
type Dialer struct {
	config Config
	auth   smtp.Auth
}

// NewDialer creates a new Dialer with the given SMTP configuration.
func NewDialer(config Config) *Dialer {
	return &Dialer{
		config: config,
		auth:   smtp.PlainAuth("", config.Username, config.Password, config.Host),
	}
}

// Send sends a plain-text email using the Dialer's SMTP configuration.
func (d *Dialer) Send(msg Message) error {
	return d.send(msg, "text/plain")
}

// SendHTML sends an HTML email using the Dialer's SMTP configuration.
func (d *Dialer) SendHTML(msg Message) error {
	return d.send(msg, "text/html")
}

func (d *Dialer) send(msg Message, contentType string) error {
	if err := validate(msg); err != nil {
		return err
	}

	raw, err := buildMessage(msg, contentType)
	if err != nil {
		return err
	}

	recipients := msg.allRecipients()
	if err := smtp.SendMail(d.config.addr(), d.auth, msg.From, recipients, raw); err != nil {
		return fmt.Errorf("mail: failed to send: %w", err)
	}

	return nil
}

// Send sends a plain-text email using a one-shot SMTP configuration.
// For sending multiple emails, prefer NewDialer to reuse the connection config.
func Send(config Config, msg Message) error {
	return NewDialer(config).Send(msg)
}

// SendHTML sends an HTML email using a one-shot SMTP configuration.
// For sending multiple emails, prefer NewDialer to reuse the connection config.
func SendHTML(config Config, msg Message) error {
	return NewDialer(config).SendHTML(msg)
}

func buildMessage(msg Message, contentType string) ([]byte, error) {
	var buf bytes.Buffer

	write := func(format string, args ...any) {
		fmt.Fprintf(&buf, format+"\r\n", args...)
	}

	fromAddr, err := mail.ParseAddress(msg.From)
	if err != nil {
		return nil, fmt.Errorf("mail: invalid From address: %w", err)
	}

	write("From: %s", fromAddr.String())
	write("To: %s", strings.Join(msg.To, ", "))

	if len(msg.CC) > 0 {
		write("CC: %s", strings.Join(msg.CC, ", "))
	}

	if msg.ReplyTo != "" {
		replyTo, err := mail.ParseAddress(msg.ReplyTo)
		if err != nil {
			return nil, fmt.Errorf("mail: invalid Reply-To address: %w", err)
		}
		write("Reply-To: %s", replyTo.String())
	}

	write("Subject: %s", msg.Subject)
	write("Date: %s", time.Now().UTC().Format(time.RFC1123Z))
	write("MIME-Version: 1.0")
	write("Content-Type: %s; charset=UTF-8", contentType)
	write("Content-Transfer-Encoding: quoted-printable")

	buf.WriteString("\r\n")
	buf.WriteString(msg.Body)

	return buf.Bytes(), nil
}

func validate(msg Message) error {
	if msg.From == "" {
		return fmt.Errorf("mail: From address is required")
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("mail: at least one To address is required")
	}

	if _, err := mail.ParseAddress(msg.From); err != nil {
		return fmt.Errorf("mail: invalid From address %q: %w", msg.From, err)
	}

	for _, addr := range msg.To {
		if _, err := mail.ParseAddress(addr); err != nil {
			return fmt.Errorf("mail: invalid To address %q: %w", addr, err)
		}
	}

	for _, addr := range msg.CC {
		if _, err := mail.ParseAddress(addr); err != nil {
			return fmt.Errorf("mail: invalid CC address %q: %w", addr, err)
		}
	}

	if msg.ReplyTo != "" {
		if _, err := mail.ParseAddress(msg.ReplyTo); err != nil {
			return fmt.Errorf("mail: invalid Reply-To address %q: %w", msg.ReplyTo, err)
		}
	}

	return nil
}