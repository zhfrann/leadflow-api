package mail

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	netmail "net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

type Message struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

type Sender interface {
	Send(ctx context.Context, message Message) error
}

type SMTPSender struct {
	address     string
	host        string
	fromName    string
	fromAddress string
	timeout     time.Duration
}

func NewSMTPSender(
	address string,
	host string,
	fromName string,
	fromAddress string,
	timeout time.Duration,
) (*SMTPSender, error) {
	if strings.TrimSpace(address) == "" {
		return nil, fmt.Errorf("SMTP address must not be empty")
	}

	if strings.TrimSpace(host) == "" {
		return nil, fmt.Errorf("SMTP host must not be empty")
	}

	if _, err := netmail.ParseAddress(fromAddress); err != nil {
		return nil, fmt.Errorf("invalid sender address: %w", err)
	}

	if timeout <= 0 {
		return nil, fmt.Errorf("SMTP timeout must be greater than zero")
	}

	return &SMTPSender{
		address:     address,
		host:        host,
		fromName:    fromName,
		fromAddress: fromAddress,
		timeout:     timeout,
	}, nil
}

func (s *SMTPSender) Send(
	ctx context.Context,
	message Message,
) error {
	rawMessage, err := s.buildMessage(message)
	if err != nil {
		return err
	}

	dialer := net.Dialer{
		Timeout: s.timeout,
	}

	connection, err := dialer.DialContext(
		ctx,
		"tcp",
		s.address,
	)
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer connection.Close()

	deadline := time.Now().Add(s.timeout)

	if contextDeadline, ok := ctx.Deadline(); ok &&
		contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}

	if err := connection.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set SMTP deadline: %w", err)
	}

	stopCancellation := context.AfterFunc(ctx, func() {
		_ = connection.Close()
	})
	defer stopCancellation()

	client, err := smtp.NewClient(connection, s.host)
	if err != nil {
		return fmt.Errorf("create SMTP client: %w", err)
	}
	defer client.Close()

	if err := client.Mail(s.fromAddress); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}

	if err := client.Rcpt(message.To); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP message writer: %w", err)
	}

	if _, err := writer.Write(rawMessage); err != nil {
		_ = writer.Close()

		return fmt.Errorf("write SMTP message: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close SMTP message writer: %w", err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("finish SMTP session: %w", err)
	}

	return nil
}

func (s *SMTPSender) buildMessage(
	message Message,
) ([]byte, error) {
	if _, err := netmail.ParseAddress(message.To); err != nil {
		return nil, fmt.Errorf("invalid recipient address: %w", err)
	}

	if containsNewline(message.Subject) {
		return nil, fmt.Errorf("email subject contains invalid newline")
	}

	from := (&netmail.Address{
		Name:    s.fromName,
		Address: s.fromAddress,
	}).String()

	to := (&netmail.Address{
		Address: message.To,
	}).String()

	var body bytes.Buffer

	multipartWriter := multipart.NewWriter(&body)

	textHeader := textproto.MIMEHeader{}
	textHeader.Set(
		"Content-Type",
		`text/plain; charset="UTF-8"`,
	)
	textHeader.Set("Content-Transfer-Encoding", "8bit")

	textPart, err := multipartWriter.CreatePart(textHeader)
	if err != nil {
		return nil, fmt.Errorf("create text email part: %w", err)
	}

	if _, err := textPart.Write(
		[]byte(message.TextBody),
	); err != nil {
		return nil, fmt.Errorf("write text email part: %w", err)
	}

	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set(
		"Content-Type",
		`text/html; charset="UTF-8"`,
	)
	htmlHeader.Set("Content-Transfer-Encoding", "8bit")

	htmlPart, err := multipartWriter.CreatePart(htmlHeader)
	if err != nil {
		return nil, fmt.Errorf("create HTML email part: %w", err)
	}

	if _, err := htmlPart.Write(
		[]byte(message.HTMLBody),
	); err != nil {
		return nil, fmt.Errorf("write HTML email part: %w", err)
	}

	if err := multipartWriter.Close(); err != nil {
		return nil, fmt.Errorf("close multipart email: %w", err)
	}

	var result bytes.Buffer

	fmt.Fprintf(&result, "From: %s\r\n", from)
	fmt.Fprintf(&result, "To: %s\r\n", to)
	fmt.Fprintf(
		&result,
		"Subject: %s\r\n",
		mime.QEncoding.Encode("UTF-8", message.Subject),
	)
	fmt.Fprintf(
		&result,
		"Date: %s\r\n",
		time.Now().Format(time.RFC1123Z),
	)
	fmt.Fprint(&result, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(
		&result,
		"Content-Type: multipart/alternative; boundary=%q\r\n",
		multipartWriter.Boundary(),
	)
	fmt.Fprint(&result, "\r\n")

	result.Write(body.Bytes())

	return result.Bytes(), nil
}

func containsNewline(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}
