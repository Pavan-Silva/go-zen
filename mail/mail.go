package mail

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"mime/quotedprintable"
	"net/mail"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// Config holds SMTP configuration for sending emails.
type Config struct {
	Host     string // SMTP server host (e.g., "smtp.gmail.com")
	Port     int    // SMTP server port (e.g., 587)
	Username string // SMTP username
	Password string // SMTP password
	UseTLS   bool   // Use implicit TLS on connect (port 465). Defaults to STARTTLS (port 587).
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
	BCC     []string // Blind carbon copy recipients (omitted from headers, included in envelope)
	ReplyTo string   // Reply-to address (optional)
	Subject string   // Email subject
	Body    string   // Email body (plain text or HTML depending on send method)
}

// allRecipients returns the full SMTP envelope recipients (To + CC + BCC).
// BCC addresses are included in the envelope but intentionally omitted from headers.
func (m Message) allRecipients() []string {
	all := make([]string, 0, len(m.To)+len(m.CC)+len(m.BCC))
	all = append(all, m.To...)
	all = append(all, m.CC...)
	all = append(all, m.BCC...)
	return all
}

// BulkResult holds the outcome of a single message in a bulk send operation.
type BulkResult struct {
	Message Message // The original message
	Err     error   // Non-nil if sending failed
}

// BulkOption configures bulk send behaviour.
type BulkOption func(*bulkOptions)

type bulkOptions struct {
	workers     int  // number of concurrent senders (default: 5)
	stopOnError bool // abort remaining sends on first error (default: false)
}

// WithWorkers sets the number of concurrent goroutines used during bulk send.
// Defaults to 5. Higher values increase throughput but may trigger SMTP rate limits.
func WithWorkers(n int) BulkOption {
	return func(o *bulkOptions) {
		if n > 0 {
			o.workers = n
		}
	}
}

// WithStopOnError configures bulk send to abort after the first failed message.
func WithStopOnError() BulkOption {
	return func(o *bulkOptions) {
		o.stopOnError = true
	}
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
	return d.sendOne(msg, "text/plain")
}

// SendHTML sends an HTML email using the Dialer's SMTP configuration.
func (d *Dialer) SendHTML(msg Message) error {
	return d.sendOne(msg, "text/html")
}

// SendBulk sends multiple plain-text emails concurrently.
// It returns a slice of BulkResult, one per message, in the same order.
// All results are always returned unless WithStopOnError is set.
func (d *Dialer) SendBulk(messages []Message, opts ...BulkOption) []BulkResult {
	return d.sendBulk(messages, "text/plain", opts...)
}

// SendBulkHTML sends multiple HTML emails concurrently.
// It returns a slice of BulkResult, one per message, in the same order.
// All results are always returned unless WithStopOnError is set.
func (d *Dialer) SendBulkHTML(messages []Message, opts ...BulkOption) []BulkResult {
	return d.sendBulk(messages, "text/html", opts...)
}

func (d *Dialer) sendBulk(messages []Message, contentType string, opts ...BulkOption) []BulkResult {
	if len(messages) == 0 {
		return nil
	}

	options := &bulkOptions{workers: 5}
	for _, o := range opts {
		o(options)
	}

	results := make([]BulkResult, len(messages))
	queue := make(chan int, len(messages))

	stop := make(chan struct{})
	var stopOnce sync.Once

	for i := range messages {
		queue <- i
	}
	close(queue)

	var wg sync.WaitGroup
	workers := min(options.workers, len(messages))

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Each worker holds its own persistent SMTP connection.
			client, err := d.dial()
			if err != nil {
				// If we can't connect, fail all messages this worker would have handled.
				for i := range queue {
					results[i] = BulkResult{Message: messages[i], Err: fmt.Errorf("mail: dial failed: %w", err)}
				}
				return
			}
			defer client.Quit()

			for i := range queue {
				select {
				case <-stop:
					results[i] = BulkResult{Message: messages[i], Err: fmt.Errorf("mail: aborted")}
					continue
				default:
				}

				err := d.sendWithClient(client, messages[i], contentType)
				results[i] = BulkResult{Message: messages[i], Err: err}

				if err != nil && options.stopOnError {
					stopOnce.Do(func() { close(stop) })
				}
			}
		}()
	}

	wg.Wait()
	return results
}

// dial opens a persistent SMTP connection, handling both STARTTLS (587) and
// implicit TLS (465) depending on Config.UseTLS.
func (d *Dialer) dial() (*smtp.Client, error) {
	tlsConfig := &tls.Config{ServerName: d.config.Host}

	if d.config.UseTLS {
		conn, err := tls.Dial("tcp", d.config.addr(), tlsConfig)
		if err != nil {
			return nil, err
		}
		return smtp.NewClient(conn, d.config.Host)
	}

	client, err := smtp.Dial(d.config.addr())
	if err != nil {
		return nil, err
	}

	if err := client.StartTLS(tlsConfig); err != nil {
		client.Close()
		return nil, err
	}

	if err := client.Auth(d.auth); err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}

// sendWithClient sends a single message over an existing SMTP client connection,
// resetting the envelope between sends so the connection can be reused.
func (d *Dialer) sendWithClient(client *smtp.Client, msg Message, contentType string) error {
	if err := validate(msg); err != nil {
		return err
	}

	raw, err := buildMessage(msg, contentType)
	if err != nil {
		return err
	}

	if err := client.Reset(); err != nil {
		return fmt.Errorf("mail: reset failed: %w", err)
	}
	if err := client.Mail(msg.From); err != nil {
		return fmt.Errorf("mail: MAIL FROM failed: %w", err)
	}
	for _, addr := range msg.allRecipients() {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("mail: RCPT TO failed for %q: %w", addr, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mail: DATA failed: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("mail: write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: close data writer failed: %w", err)
	}

	return nil
}

// sendOne opens a fresh connection, sends a single message, and closes.
func (d *Dialer) sendOne(msg Message, contentType string) error {
	if err := validate(msg); err != nil {
		return err
	}

	raw, err := buildMessage(msg, contentType)
	if err != nil {
		return err
	}

	if err := smtp.SendMail(d.config.addr(), d.auth, msg.From, msg.allRecipients(), raw); err != nil {
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

	// Properly encode body as quoted-printable.
	qw := quotedprintable.NewWriter(&buf)
	if _, err := qw.Write([]byte(msg.Body)); err != nil {
		return nil, fmt.Errorf("mail: failed to encode body: %w", err)
	}
	if err := qw.Close(); err != nil {
		return nil, fmt.Errorf("mail: failed to flush body: %w", err)
	}

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