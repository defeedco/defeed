-- Migration to change embedding dimensions from 3072 to 1536
-- This will require re-embedding all existing activities with the new model

BEGIN;

-- First, clear existing embeddings (they'll be regenerated with new dimensions)
UPDATE activities SET embedding = NULL WHERE embedding IS NOT NULL;

-- Note: You'll need to run a database migration to change the column type
-- This depends on your migration tool, but the general approach is:
ALTER TABLE activities ALTER COLUMN embedding TYPE vector(1536);

-- The system will automatically re-embed activities when they're next processed
-- Since we've implemented the processing guard, this will only happen once per activity

COMMIT;
