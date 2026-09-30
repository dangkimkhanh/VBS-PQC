package utils

import (
	"encoding/base64"
	"fmt"
	"mime"
	"net/smtp"
	"strings"
	"time"
)

type EmailSender interface {
	SendEmail(to, subject, body string) error
}

type smtpSender struct {
	from     string
	password string
	host     string
	port     string
}

func NewSMTPSender(from, password, host, port string) EmailSender {
	return &smtpSender{from: from, password: password, host: host, port: port}
}

func (s *smtpSender) SendEmail(to, subject, body string) error {
	if strings.ContainsAny(to, "\r\n") || strings.ContainsAny(subject, "\r\n") {
		return fmt.Errorf("invalid email header")
	}

	// Vietnamese text needs explicit UTF-8 headers; the subject is RFC 2047 encoded
	// and the body base64 encoded so no mail server rewrites the characters.
	encodedBody := base64.StdEncoding.EncodeToString([]byte(strings.ReplaceAll(body, "\n", "\r\n")))
	var wrapped strings.Builder
	for len(encodedBody) > 76 {
		wrapped.WriteString(encodedBody[:76] + "\r\n")
		encodedBody = encodedBody[76:]
	}
	wrapped.WriteString(encodedBody)

	headers := []string{
		"From: " + mime.QEncoding.Encode("utf-8", "Văn bằng số PQC") + " <" + s.from + ">",
		"To: " + to,
		"Subject: " + mime.BEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: base64",
	}
	msg := []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + wrapped.String() + "\r\n")

	auth := smtp.PlainAuth("", s.from, s.password, s.host)
	return smtp.SendMail(s.host+":"+s.port, auth, s.from, []string{to}, msg)
}
