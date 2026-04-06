package mail

import (
	"bytes"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"
)

// Config holds SMTP configuration for sending emails.
type Config struct {
	Host     string // SMTP server host (e.g., "smtp.gmail.com")
	Port     int    // SMTP server port (e.g., 587)
	Username string // SMTP username
	Password string // SMTP password
}

// Message represents an email message.
type Message struct {
	From    string   // Sender email address
	To      []string // Recipient email addresses
	Subject string   // Email subject
	Body    string   // Email body
}

// Send sends a plain text email using the provided SMTP configuration.
// It validates addresses using net/mail and sends via net/smtp.
func Send(config Config, msg Message) error {
	return send(config, msg, "text/plain")
}

// SendHTML sends an HTML email using the provided SMTP configuration.
// It sets Content-Type to text/html and sends via net/smtp.
func SendHTML(config Config, msg Message) error {
	return send(config, msg, "text/html")
}

func send(config Config, msg Message, contentType string) error {
	if err := validate(msg); err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("From: %s\r\n", msg.From))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ",")))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	if contentType != "text/plain" {
		buf.WriteString(fmt.Sprintf("Content-Type: %s; charset=UTF-8\r\n", contentType))
	}
	buf.WriteString("\r\n")
	buf.WriteString(msg.Body)

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	if err := smtp.SendMail(addr, auth, msg.From, msg.To, buf.Bytes()); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func validate(msg Message) error {
	if _, err := mail.ParseAddress(msg.From); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}

	for _, to := range msg.To {
		if _, err := mail.ParseAddress(to); err != nil {
			return fmt.Errorf("invalid to address %q: %w", to, err)
		}
	}

	return nil
}
