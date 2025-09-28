-- name: CreateUser :one
INSERT INTO users (name, email, password_hash, phone, is_admin, email_verification_token, email_verification_expires)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetAllUsers :many
SELECT * FROM users ORDER BY name;

-- name: CreateAppointment :one
INSERT INTO appointments (user_id, car_id, datetime, title, description)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UserCancelAppointment :exec
UPDATE appointments SET status = 'cancelled' WHERE id = $1 AND user_id = $2;

-- name: AdminDeleteAppointment :exec
DELETE FROM appointments WHERE id = $1;

-- name: GetAppointmentsForUser :many
SELECT
  a.*,
  c.registration_number AS car_registration,
  c.make AS car_make,
  c.colour AS car_colour
FROM appointments a
LEFT JOIN cars c ON a.car_id = c.id
WHERE a.user_id = $1
ORDER BY a.datetime DESC;

-- name: GetAppointmentsForCar :many
SELECT * FROM appointments WHERE car_id = $1 ORDER BY datetime DESC;

-- name: GetNextAppointmentForCar :one
SELECT * FROM appointments
WHERE car_id = $1 AND datetime > NOW() AND status != 'cancelled'
ORDER BY datetime ASC
LIMIT 1;

-- name: UpdateAppointmentStatus :exec
UPDATE appointments SET status = $2 WHERE id = $1;

-- name: GetAllAppointments :many
SELECT
  a.id,
  a.user_id,
  a.car_id,
  a.datetime,
  a.title,
  a.description,
  a.status,
  a.created_at,
  u.name AS user_name,
  u.email AS user_email,
  u.phone AS user_phone,
  c.registration_number AS car_registration,
  c.make AS car_make,
  c.colour AS car_colour
FROM appointments a
JOIN users u ON a.user_id = u.id
LEFT JOIN cars c ON a.car_id = c.id
ORDER BY a.created_at DESC;

-- name: GetAppointmentsByID :one
SELECT * FROM appointments WHERE id = $1;

-- name: GetAllAppointmentsByMonth :many
SELECT
  a.*,
  c.registration_number AS car_registration,
  c.make AS car_make
FROM appointments a
LEFT JOIN cars c ON a.car_id = c.id
WHERE datetime >= $1 AND datetime < $2
ORDER BY datetime;

-- Car queries
-- name: CreateCar :one
INSERT INTO cars (
    user_id,
    registration_number,
    make,
    colour,
    fuel_type,
    engine_capacity,
    year_of_manufacture,
    month_of_first_registration,
    month_of_first_dvla_registration,
    tax_status,
    tax_due_date,
    mot_status,
    mot_expiry_date,
    co2_emissions,
    euro_status,
    real_driving_emissions,
    revenue_weight,
    type_approval,
    wheelplan,
    automated_vehicle,
    marked_for_export,
    date_of_last_v5c_issued,
    art_end_date,
    dvla_data_fetched_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24
) RETURNING *;

-- name: GetCarsByUserID :many
SELECT * FROM cars WHERE user_id = $1 ORDER BY created_at DESC;

-- name: GetCarByID :one
SELECT * FROM cars WHERE id = $1;

-- name: GetCarByRegistration :one
SELECT * FROM cars WHERE registration_number = $1;

-- name: UpdateCarDVLAData :one
UPDATE cars SET
    make = $2,
    colour = $3,
    fuel_type = $4,
    engine_capacity = $5,
    year_of_manufacture = $6,
    month_of_first_registration = $7,
    month_of_first_dvla_registration = $8,
    tax_status = $9,
    tax_due_date = $10,
    mot_status = $11,
    mot_expiry_date = $12,
    co2_emissions = $13,
    euro_status = $14,
    real_driving_emissions = $15,
    revenue_weight = $16,
    type_approval = $17,
    wheelplan = $18,
    automated_vehicle = $19,
    marked_for_export = $20,
    date_of_last_v5c_issued = $21,
    art_end_date = $22,
    dvla_data_fetched_at = $23,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteCar :exec
DELETE FROM cars WHERE id = $1 AND user_id = $2;

-- name: AdminDeleteCar :exec
DELETE FROM cars WHERE id = $1;

-- name: GetAllCarsWithUsers :many
SELECT
    c.*,
    u.name AS user_name,
    u.email AS user_email
FROM cars c
JOIN users u ON c.user_id = u.id
ORDER BY c.created_at DESC;

-- name: UpdateUserEmail :exec
UPDATE users SET email = $2 WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: GetUserByVerificationToken :one
SELECT * FROM users WHERE email_verification_token = $1 AND email_verification_expires > NOW();

-- name: VerifyUserEmail :exec
UPDATE users SET email_verified = TRUE, email_verification_token = NULL, email_verification_expires = NULL WHERE id = $1;

-- name: SetPasswordResetToken :exec
UPDATE users SET password_reset_token = $2, password_reset_expires = $3 WHERE email = $1;

-- name: GetActivePasswordResetToken :one
SELECT password_reset_token, password_reset_expires FROM users WHERE email = $1 AND password_reset_token IS NOT NULL AND password_reset_expires > NOW();

-- name: GetUserByPasswordResetToken :one
SELECT * FROM users WHERE password_reset_token = $1 AND password_reset_expires > NOW();

-- name: ResetPassword :exec
UPDATE users SET password_hash = $2, password_reset_token = NULL, password_reset_expires = NULL WHERE id = $1;

-- name: SetEmailChangeRequest :exec
UPDATE users SET pending_email = $2, email_change_token = $3, email_change_expires = $4 WHERE id = $1;

-- name: GetUserByEmailChangeToken :one
SELECT * FROM users WHERE email_change_token = $1 AND email_change_expires > NOW();

-- name: ConfirmEmailChange :exec
UPDATE users SET email = pending_email, pending_email = NULL, email_change_token = NULL, email_change_expires = NULL, email_verified = TRUE WHERE id = $1;

-- name: BlacklistToken :exec
INSERT INTO token_blacklist (token_hash, user_id, reason, expires_at) VALUES ($1, $2, $3, $4);

-- name: IsTokenBlacklisted :one
SELECT EXISTS(SELECT 1 FROM token_blacklist WHERE token_hash = $1 AND expires_at > NOW());

-- name: BlacklistAllUserTokens :exec
INSERT INTO token_blacklist (token_hash, user_id, reason, expires_at)
SELECT
    'user_' || $1::text || '_' || extract(epoch from NOW())::text, -- Generate a placeholder hash for user-wide blacklist
    $1,
    $2,
    $3
WHERE NOT EXISTS (
    SELECT 1 FROM token_blacklist
    WHERE user_id = $1 AND reason = $2 AND blacklisted_at > NOW() - INTERVAL '1 minute'
);

-- name: CleanupExpiredTokens :exec
DELETE FROM token_blacklist WHERE expires_at <= NOW();

-- name: ListAllUsers :many
SELECT id, name, email, is_admin, email_verified, created_at FROM users ORDER BY created_at DESC;

-- name: SetEmailVerificationToken :exec
UPDATE users SET email_verification_token = $2, email_verification_expires = $3 WHERE id = $1;

-- name: MakeUserAdmin :exec
UPDATE users SET is_admin = TRUE WHERE email = $1;

-- name: RemoveUserAdmin :exec
UPDATE users SET is_admin = FALSE WHERE email = $1;

-- name: ActivateUserByEmail :exec
UPDATE users SET email_verified = TRUE WHERE email = $1;

-- name: DeleteUserByEmail :exec
DELETE FROM users WHERE email = $1;


