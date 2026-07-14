package notification

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	texttemplate "text/template"

	mailx "github.com/zhfrann/leadflow-api/internal/platform/mail"
)

//go:embed templates/*.tmpl
var templateFiles embed.FS

type UserRegisteredPayload struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
}

type Templates struct {
	text *texttemplate.Template
	html *htmltemplate.Template
}

func NewTemplates() (*Templates, error) {
	textTemplate, err := texttemplate.ParseFS(
		templateFiles,
		"templates/welcome.txt.tmpl",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"parse welcome text template: %w",
			err,
		)
	}

	htmlTemplate, err := htmltemplate.ParseFS(
		templateFiles,
		"templates/welcome.html.tmpl",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"parse welcome HTML template: %w",
			err,
		)
	}

	return &Templates{
		text: textTemplate,
		html: htmlTemplate,
	}, nil
}

func (t *Templates) WelcomeMessage(
	recipient string,
	payload UserRegisteredPayload,
) (mailx.Message, error) {
	var textBody bytes.Buffer

	if err := t.text.ExecuteTemplate(
		&textBody,
		"welcome.txt.tmpl",
		payload,
	); err != nil {
		return mailx.Message{}, fmt.Errorf(
			"render welcome text template: %w",
			err,
		)
	}

	var htmlBody bytes.Buffer

	if err := t.html.ExecuteTemplate(
		&htmlBody,
		"welcome.html.tmpl",
		payload,
	); err != nil {
		return mailx.Message{}, fmt.Errorf(
			"render welcome HTML template: %w",
			err,
		)
	}

	return mailx.Message{
		To:       recipient,
		Subject:  "Welcome to LeadFlow",
		TextBody: textBody.String(),
		HTMLBody: htmlBody.String(),
	}, nil
}
