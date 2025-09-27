-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- Add car_id column to appointments table
ALTER TABLE appointments ADD COLUMN car_id UUID REFERENCES cars(id) ON DELETE CASCADE;

-- Update existing appointments to link to cars (this will be empty since we cleared data)
-- Future appointments will be created with car_id

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
ALTER TABLE appointments DROP COLUMN car_id;