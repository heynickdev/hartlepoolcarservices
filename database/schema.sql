-- This file defines the database schema for sqlc to use for code generation.
-- It should be kept in sync with your migration files.

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    phone TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'user',
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    email_verification_token TEXT,
    email_verification_expires TIMESTAMPTZ,
    password_reset_token TEXT,
    password_reset_expires TIMESTAMPTZ,
    pending_email TEXT,
    email_change_token TEXT,
    email_change_expires TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE appointments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    car_id UUID REFERENCES cars(id) ON DELETE CASCADE,
    datetime TIMESTAMPTZ NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'pending', -- e.g., pending, confirmed, completed, cancelled
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE cars (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Core identification
    registration_number TEXT NOT NULL UNIQUE,

    -- Basic vehicle info
    make TEXT,
    colour TEXT,
    fuel_type TEXT,
    engine_capacity INTEGER,
    year_of_manufacture INTEGER,
    month_of_first_registration TEXT,
    month_of_first_dvla_registration TEXT,

    -- Tax and MOT status
    tax_status TEXT,
    tax_due_date DATE,
    mot_status TEXT,
    mot_expiry_date DATE,

    -- Technical specifications
    co2_emissions INTEGER,
    euro_status TEXT,
    real_driving_emissions TEXT,
    revenue_weight INTEGER,
    type_approval TEXT,
    wheelplan TEXT,
    automated_vehicle BOOLEAN,

    -- Registration details
    marked_for_export BOOLEAN,
    date_of_last_v5c_issued DATE,
    art_end_date DATE,

    -- Metadata
    dvla_data_fetched_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Token blacklist table for JWT session management
CREATE TABLE token_blacklist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    reason VARCHAR(100) NOT NULL,
    blacklisted_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_appointments_user_id ON appointments(user_id);
CREATE INDEX idx_appointments_car_id ON appointments(car_id);
CREATE INDEX idx_appointments_date ON appointments(datetime);
CREATE INDEX idx_token_blacklist_hash ON token_blacklist(token_hash);
CREATE INDEX idx_token_blacklist_expires ON token_blacklist(expires_at);
