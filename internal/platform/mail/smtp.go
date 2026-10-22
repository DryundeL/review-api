package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

var ErrUnavailable = errors.New("email delivery unavailable")

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

type Sender struct {
	cfg Config
}

func New(cfg Config) *Sender { return &Sender{cfg: cfg} }

func (s *Sender) SendCode(ctx context.Context, to, code, purpose string) error {
	if s == nil || s.cfg.Host == "" || s.cfg.Port <= 0 || s.cfg.From == "" ||
		strings.ContainsAny(s.cfg.From+to, "\r\n") {
		return ErrUnavailable
	}
	subject := "Код входа в Review Hub"
	action := "войти в аккаунт"
	if purpose == "bind" {
		subject = "Подтвердите почту в Review Hub"
		action = "привязать эту почту к аккаунту"
	}
	message, err := renderMessage(s.cfg.From, to, subject, action, code)
	if err != nil {
		return err
	}
	address := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	dialer := net.Dialer{Timeout: 8 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("connect smtp: %w", err)
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return errors.New("smtp server does not support STARTTLS")
	}
	if err := client.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("start smtp tls: %w", err)
	}
	if s.cfg.User != "" {
		if err := client.Auth(smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("authenticate smtp: %w", err)
		}
	}
	if err := client.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("smtp from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		return fmt.Errorf("write smtp message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close smtp message: %w", err)
	}
	return client.Quit()
}

func renderMessage(from, to, subject, action, code string) ([]byte, error) {
	var html bytes.Buffer
	if err := codeTemplate.Execute(&html, struct {
		Action string
		Code   string
	}{action, code}); err != nil {
		return nil, fmt.Errorf("render email: %w", err)
	}
	boundary := "review-hub-code"
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject),
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative; boundary=" + boundary,
	}
	plain := "Код Review Hub: " + code + "\r\n\r\nИспользуйте его, чтобы " + action + ". Код действует 10 минут."
	body := strings.Join(headers, "\r\n") + "\r\n\r\n--" + boundary +
		"\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + plain +
		"\r\n--" + boundary + "\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + html.String() +
		"\r\n--" + boundary + "--\r\n"
	return []byte(body), nil
}

var codeTemplate = template.Must(template.New("code").Parse(`<!doctype html>
<html lang="ru"><body style="margin:0;background:#0a0c0b;color:#f4f6f4;font-family:Manrope,Arial,sans-serif">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#0a0c0b;padding:32px 12px"><tr><td align="center">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:480px;background:#161a18;border:1px solid #2a322e;border-radius:16px;padding:32px">
<tr><td style="font-size:24px;font-weight:700;padding-bottom:16px">Review Hub</td></tr>
<tr><td style="color:#8b968f;font-size:16px;line-height:1.5;padding-bottom:24px">Используйте этот код, чтобы {{.Action}}:</td></tr>
<tr><td style="color:#b7e4c7;font-size:36px;font-weight:700;letter-spacing:8px;padding-bottom:24px">{{.Code}}</td></tr>
<tr><td style="color:#8b968f;font-size:14px;line-height:1.5">Код действует 10 минут. Если запрос сделали не вы, проигнорируйте письмо.</td></tr>
</table></td></tr></table></body></html>`))
