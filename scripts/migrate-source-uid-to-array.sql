-- Migration to change source_uid from string to source_uids array
-- This migration handles both:
-- 1. Database schema: source_uid column → source_uids column
-- 2. JSON data: raw_json field source_id → source_ids

BEGIN;

-- Step 1: Add the new source_uids column as JSONB array
ALTER TABLE activities ADD COLUMN source_uids JSONB DEFAULT '[]'::jsonb;

-- Step 2: Migrate existing source_uid data to source_uids array
-- Convert each source_uid string to a single-element JSON array
UPDATE activities 
SET source_uids = jsonb_build_array(source_uid)
WHERE source_uid IS NOT NULL;

-- Step 3: Migrate raw_json field from source_id to source_ids
-- This updates the JSON stored in raw_json to use the new array format
UPDATE activities
SET raw_json = jsonb_set(
    raw_json::jsonb #- '{source_id}',
    '{source_ids}',
    jsonb_build_array(raw_json::jsonb->'source_id')
)::text
WHERE raw_json::jsonb ? 'source_id'
  AND NOT (raw_json::jsonb ? 'source_ids');

-- Step 4: Drop the old source_uid column
ALTER TABLE activities DROP COLUMN source_uid;

-- Step 5: Create a GIN index on source_uids for efficient JSON array queries
CREATE INDEX idx_activities_source_uids ON activities USING GIN (source_uids);

COMMIT;

