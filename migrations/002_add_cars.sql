-- +goose Up
-- SQL in this section is executed when the migration is applied.

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

-- Index for efficient user lookups
CREATE INDEX idx_cars_user_id ON cars(user_id);

-- Index for registration number lookups
CREATE INDEX idx_cars_registration_number ON cars(registration_number);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
DROP TABLE IF EXISTS cars;