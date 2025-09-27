package utils

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"os"
	"strconv"
)

type EmailService struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

func NewEmailService() *EmailService {
	port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	return &EmailService{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     port,
		Username: os.Getenv("SMTP_EMAIL"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_EMAIL"),
	}
}

func (e *EmailService) SendEmail(to, subject, body string) error {
	if e.Host == "" || e.Username == "" || e.Password == "" {
		return fmt.Errorf("email configuration not properly set")
	}

	// For port 465, use implicit TLS (SSL)
	if e.Port == 465 {
		return e.sendEmailWithTLS(to, subject, body)
	}

	// For other ports (like 587), use standard SMTP
	auth := smtp.PlainAuth("", e.Username, e.Password, e.Host)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		e.From, to, subject, body)

	addr := fmt.Sprintf("%s:%d", e.Host, e.Port)
	return smtp.SendMail(addr, auth, e.From, []string{to}, []byte(msg))
}

func (e *EmailService) sendEmailWithTLS(to, subject, body string) error {
	// Create TLS connection for port 465 (implicit TLS)
	addr := fmt.Sprintf("%s:%d", e.Host, e.Port)

	// Create TLS config
	tlsConfig := &tls.Config{
		ServerName: e.Host,
		// For production, you might want to set InsecureSkipVerify to false
		// and ensure proper certificate validation
		InsecureSkipVerify: false,
	}

	// Establish TLS connection
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server with TLS: %v", err)
	}
	defer conn.Close()

	// Create SMTP client over TLS connection
	client, err := smtp.NewClient(conn, e.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %v", err)
	}
	defer client.Quit()

	// Authenticate
	auth := smtp.PlainAuth("", e.Username, e.Password, e.Host)
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %v", err)
	}

	// Set sender
	if err = client.Mail(e.From); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}

	// Set recipient
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %v", err)
	}

	// Send email body
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to open data writer: %v", err)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		e.From, to, subject, body)

	_, err = writer.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("failed to write email body: %v", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close email writer: %v", err)
	}

	return nil
}

func (e *EmailService) SendVerificationEmail(to, token string) error {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Ensure HTTPS for production domain
	if baseURL == "hartlepoolcarservices.com" {
		baseURL = "https://hartlepoolcarservices.com"
	}

	verificationURL := fmt.Sprintf("%s/verify-email?token=%s", baseURL, token)

	subject := "Verify Your Email - Hartlepool Car Services"
	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
				<h2 style="color: #2c3e50;">Welcome to Hartlepool Car Services!</h2>
				<p>Thank you for registering with us. To complete your registration, please verify your email address by clicking the button below:</p>

				<div style="text-align: center; margin: 30px 0;">
					<a href="%s" style="background-color: #3498db; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block;">Verify Email Address</a>
				</div>

				<p>If the button doesn't work, you can also copy and paste this link into your browser:</p>
				<p style="word-break: break-all; color: #7f8c8d;">%s</p>

				<p style="margin-top: 30px;">This verification link will expire in 24 hours.</p>

				<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
				<p style="font-size: 12px; color: #7f8c8d;">
					If you didn't create an account with us, please ignore this email.
				</p>
				<p style="font-size: 12px; color: #7f8c8d;">
					Hartlepool Car Services<br>
					Email: info@hartlepoolcarservices.com
				</p>
			</div>
		</body>
		</html>
	`, verificationURL, verificationURL)

	return e.SendEmail(to, subject, body)
}