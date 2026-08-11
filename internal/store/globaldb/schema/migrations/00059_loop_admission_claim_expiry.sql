-- +goose Up
-- Migration 47 fell back to claimed_at for timestamps serialized by the Go
-- SQLite driver. Repair only those rows, preserving any fractional seconds.
UPDATE loop_admission_claims
SET expires_at =
  strftime('%Y-%m-%dT%H:%M:%S', substr(claimed_at, 1, 19), '+7 days') ||
  CASE
    WHEN substr(claimed_at, 20, 1) = '.'
      THEN CASE
        WHEN instr(substr(claimed_at, 20), ' ') > 0
          THEN substr(claimed_at, 20, instr(substr(claimed_at, 20), ' ') - 1)
        WHEN instr(substr(claimed_at, 20), 'Z') > 0
          THEN substr(claimed_at, 20, instr(substr(claimed_at, 20), 'Z') - 1)
        ELSE substr(claimed_at, 20)
      END
    ELSE '.000000000'
  END ||
  'Z'
WHERE expires_at = claimed_at;

-- Writers before migration 59 serialized time.Time values using the driver's
-- space-separated format. Canonicalize those values so lexical comparisons
-- against new timestamps preserve the actual expiry horizon.
UPDATE loop_admission_claims
SET claimed_at = CASE
    WHEN substr(claimed_at, 11, 1) = ' ' THEN
      strftime('%Y-%m-%dT%H:%M:%S', substr(claimed_at, 1, 19)) ||
      CASE
        WHEN substr(claimed_at, 20, 1) = '.'
          THEN substr(claimed_at, 20, instr(substr(claimed_at, 20), ' ') - 1)
        ELSE '.000000000'
      END ||
      'Z'
    ELSE claimed_at
  END,
  expires_at = CASE
    WHEN substr(expires_at, 11, 1) = ' ' THEN
      strftime('%Y-%m-%dT%H:%M:%S', substr(expires_at, 1, 19)) ||
      CASE
        WHEN substr(expires_at, 20, 1) = '.'
          THEN substr(expires_at, 20, instr(substr(expires_at, 20), ' ') - 1)
        ELSE '.000000000'
      END ||
      'Z'
    ELSE expires_at
  END,
  last_suppressed_at = CASE
    WHEN last_suppressed_at IS NULL THEN NULL
    WHEN substr(last_suppressed_at, 11, 1) = ' ' THEN
      strftime('%Y-%m-%dT%H:%M:%S', substr(last_suppressed_at, 1, 19)) ||
      CASE
        WHEN substr(last_suppressed_at, 20, 1) = '.'
          THEN substr(last_suppressed_at, 20, instr(substr(last_suppressed_at, 20), ' ') - 1)
        ELSE '.000000000'
      END ||
      'Z'
    ELSE last_suppressed_at
  END
WHERE substr(claimed_at, 11, 1) = ' '
  OR substr(expires_at, 11, 1) = ' '
  OR substr(last_suppressed_at, 11, 1) = ' ';
