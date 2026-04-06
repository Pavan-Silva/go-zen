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
	Body    string   // Email body (plain text)
}

// Send sends an email using the provided SMTP configuration.
// It validates the message using net/mail and sends via net/smtp.
func Send(config Config, msg Message) error {
	// Validate From address
	if _, err := mail.ParseAddress(msg.From); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}

	// Validate To addresses
	for _, to := range msg.To {
		if _, err := mail.ParseAddress(to); err != nil {
			return fmt.Errorf("invalid to address %q: %w", to, err)
		}
	}

	// Build the email message
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("From: %s\r\n", msg.From))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ",")))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	buf.WriteString("\r\n") // Empty line separates headers from body
	buf.WriteString(msg.Body)

	// SMTP server address
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	// Authenticate
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	// Send the email
	err := smtp.SendMail(addr, auth, msg.From, msg.To, buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// SendHTML sends an HTML email.
// Similar to Send, but sets Content-Type to text/html.
func SendHTML(config Config, msg Message) error {
	// Validate addresses (same as Send)
	if _, err := mail.ParseAddress(msg.From); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}
	for _, to := range msg.To {
		if _, err := mail.ParseAddress(to); err != nil {
			return fmt.Errorf("invalid to address %q: %w", to, err)
		}
	}

	// Build the email message with HTML content type
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("From: %s\r\n", msg.From))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ",")))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(msg.Body)

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	err := smtp.SendMail(addr, auth, msg.From, msg.To, buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send HTML email: %w", err)
	}

	return nil
}