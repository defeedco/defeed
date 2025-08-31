-- Migration script to update feeds table from 'summary' to 'summaries'
-- This script handles the transition from the old single summary field to the new period-based summaries

-- First, add the new summaries column if it doesn't exist
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'feeds' AND column_name = 'summaries'
    ) THEN
        ALTER TABLE feeds ADD COLUMN summaries JSONB;
    END IF;
END $$;

-- Migrate existing summary data to the new summaries structure
-- Convert the old 'summary' field to 'summaries' with 'all' period
UPDATE feeds 
SET summaries = jsonb_build_object('all', summary)
WHERE summary IS NOT NULL AND summaries IS NULL;

-- Drop the old summary column after migration
DO $$ 
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'feeds' AND column_name = 'summary'
    ) THEN
        ALTER TABLE feeds DROP COLUMN summary;
    END IF;
END $$;
