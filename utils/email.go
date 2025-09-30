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

func (e *EmailService) SendPasswordResetEmail(to, token string) error {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Ensure HTTPS for production domain
	if baseURL == "hartlepoolcarservices.com" {
		baseURL = "https://hartlepoolcarservices.com"
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", baseURL, token)

	subject := "Password Reset Request - Hartlepool Car Services"
	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
				<h2 style="color: #2c3e50;">Password Reset Request</h2>
				<p>You have requested to reset your password for Hartlepool Car Services. Click the button below to reset your password:</p>

				<div style="text-align: center; margin: 30px 0;">
					<a href="%s" style="background-color: #e74c3c; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block;">Reset Password</a>
				</div>

				<p>If the button doesn't work, you can also copy and paste this link into your browser:</p>
				<p style="word-break: break-all; color: #7f8c8d;">%s</p>

				<p style="margin-top: 30px;">This password reset link will expire in 1 hour.</p>

				<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
				<p style="font-size: 12px; color: #7f8c8d;">
					If you didn't request a password reset, please ignore this email. Your password will remain unchanged.
				</p>
				<p style="font-size: 12px; color: #7f8c8d;">
					Hartlepool Car Services<br>
					Email: info@hartlepoolcarservices.com
				</p>
			</div>
		</body>
		</html>
	`, resetURL, resetURL)

	return e.SendEmail(to, subject, body)
}

func (e *EmailService) SendAppointmentNotification(userName, userEmail, carRegistration, carMake, appointmentTitle, appointmentDescription, appointmentDateTime string) error {
	subject := "New Appointment Created - Hartlepool Car Services"
	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
				<h2 style="color: #2c3e50;">New Appointment Notification</h2>
				<p>A new appointment has been created by a customer.</p>

				<div style="background-color: #f8f9fa; border-left: 4px solid #3498db; padding: 15px; margin: 20px 0;">
					<h3 style="margin-top: 0; color: #2c3e50;">Appointment Details</h3>
					<p style="margin: 8px 0;"><strong>Customer Name:</strong> %s</p>
					<p style="margin: 8px 0;"><strong>Customer Email:</strong> %s</p>
					<p style="margin: 8px 0;"><strong>Vehicle:</strong> %s - %s</p>
					<p style="margin: 8px 0;"><strong>Service:</strong> %s</p>
					<p style="margin: 8px 0;"><strong>Description:</strong> %s</p>
					<p style="margin: 8px 0;"><strong>Date & Time:</strong> %s</p>
				</div>

				<p style="margin-top: 20px;">Please log in to the admin dashboard to review and confirm this appointment.</p>

				<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
				<p style="font-size: 12px; color: #7f8c8d;">
					This is an automated notification from Hartlepool Car Services.<br>
					Email: info@hartlepoolcarservices.com
				</p>
			</div>
		</body>
		</html>
	`, userName, userEmail, carRegistration, carMake, appointmentTitle, appointmentDescription, appointmentDateTime)

	return e.SendEmail("info@hartlepoolcarservices.com", subject, body)
}

func (e *EmailService) SendAppointmentConfirmedEmail(userName, userEmail, carRegistration, carMake, appointmentTitle, appointmentDateTime string) error {
	subject := "Appointment Confirmed - Hartlepool Car Services"
	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
				<h2 style="color: #27ae60;">Appointment Confirmed!</h2>
				<p>Dear %s,</p>
				<p>Great news! Your appointment has been confirmed by our team.</p>

				<div style="background-color: #d5f4e6; border-left: 4px solid #27ae60; padding: 15px; margin: 20px 0;">
					<h3 style="margin-top: 0; color: #27ae60;">Appointment Details</h3>
					<p style="margin: 8px 0;"><strong>Service:</strong> %s</p>
					<p style="margin: 8px 0;"><strong>Vehicle:</strong> %s - %s</p>
					<p style="margin: 8px 0;"><strong>Date & Time:</strong> %s</p>
					<p style="margin: 8px 0;"><strong>Status:</strong> <span style="color: #27ae60; font-weight: bold;">CONFIRMED</span></p>
				</div>

				<p>We look forward to seeing you! If you need to make any changes, please contact us as soon as possible.</p>

				<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
				<p style="font-size: 12px; color: #7f8c8d;">
					Hartlepool Car Services<br>
					Email: info@hartlepoolcarservices.com
				</p>
			</div>
		</body>
		</html>
	`, userName, appointmentTitle, carRegistration, carMake, appointmentDateTime)

	return e.SendEmail(userEmail, subject, body)
}

func (e *EmailService) SendAppointmentCancelledEmail(userName, userEmail, carRegistration, carMake, appointmentTitle, appointmentDateTime string) error {
	subject := "Appointment Cancelled - Hartlepool Car Services"
	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
				<h2 style="color: #e74c3c;">Appointment Cancelled</h2>
				<p>Dear %s,</p>
				<p>Your appointment has been cancelled.</p>

				<div style="background-color: #fadbd8; border-left: 4px solid #e74c3c; padding: 15px; margin: 20px 0;">
					<h3 style="margin-top: 0; color: #e74c3c;">Cancelled Appointment Details</h3>
					<p style="margin: 8px 0;"><strong>Service:</strong> %s</p>
					<p style="margin: 8px 0;"><strong>Vehicle:</strong> %s - %s</p>
					<p style="margin: 8px 0;"><strong>Originally Scheduled:</strong> %s</p>
					<p style="margin: 8px 0;"><strong>Status:</strong> <span style="color: #e74c3c; font-weight: bold;">CANCELLED</span></p>
				</div>

				<p>If you would like to reschedule or have any questions, please feel free to contact us or book a new appointment through your dashboard.</p>

				<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
				<p style="font-size: 12px; color: #7f8c8d;">
					Hartlepool Car Services<br>
					Email: info@hartlepoolcarservices.com
				</p>
			</div>
		</body>
		</html>
	`, userName, appointmentTitle, carRegistration, carMake, appointmentDateTime)

	return e.SendEmail(userEmail, subject, body)
}

func (e *EmailService) SendAppointmentCompletedEmail(userName, userEmail, carRegistration, carMake, appointmentTitle, appointmentDateTime string) error {
	subject := "Service Completed - Hartlepool Car Services"
	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
				<h2 style="color: #3498db;">Service Completed</h2>
				<p>Dear %s,</p>
				<p>Thank you for choosing Hartlepool Car Services! Your service has been completed.</p>

				<div style="background-color: #d6eaf8; border-left: 4px solid #3498db; padding: 15px; margin: 20px 0;">
					<h3 style="margin-top: 0; color: #3498db;">Completed Service Details</h3>
					<p style="margin: 8px 0;"><strong>Service:</strong> %s</p>
					<p style="margin: 8px 0;"><strong>Vehicle:</strong> %s - %s</p>
					<p style="margin: 8px 0;"><strong>Service Date:</strong> %s</p>
					<p style="margin: 8px 0;"><strong>Status:</strong> <span style="color: #3498db; font-weight: bold;">COMPLETED</span></p>
				</div>

				<p>We hope you're satisfied with our service! If you have any questions or concerns, please don't hesitate to contact us.</p>
				<p>We look forward to serving you again in the future.</p>

				<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
				<p style="font-size: 12px; color: #7f8c8d;">
					Hartlepool Car Services<br>
					Email: info@hartlepoolcarservices.com
				</p>
			</div>
		</body>
		</html>
	`, userName, appointmentTitle, carRegistration, carMake, appointmentDateTime)

	return e.SendEmail(userEmail, subject, body)
}
