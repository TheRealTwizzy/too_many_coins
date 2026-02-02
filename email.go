package main

import (
	"errors"
	"fmt"
	"net/smtp"
	"os"
	"strconv"
	"strings"
)

func sendPasswordResetEmail(to string, token string, baseURL string) error {
	host := os.Getenv("SMTP_HOST")
	portStr := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")
	if host == "" || user == "" || pass == "" || from == "" {
		return errors.New("EMAIL_NOT_CONFIGURED")
	}
	if portStr == "" {
		portStr = "587"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return errors.New("EMAIL_NOT_CONFIGURED")
	}

	baseURL = strings.TrimRight(baseURL, "/")
	resetLink := fmt.Sprintf("%s/#/reset?token=%s", baseURL, token)

	subject := "Too Many Coins password reset"
	body := fmt.Sprintf("Use this link to reset your password:\n\n%s\n\nIf you did not request a reset, you can ignore this email.", resetLink)

	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")

	addr := fmt.Sprintf("%s:%d", host, port)
	auth := smtp.PlainAuth("", user, pass, host)
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}
