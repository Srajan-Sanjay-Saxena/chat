-- Revert canonical_name and index
DROP INDEX IF EXISTS idx_conversations_canonical_private;
ALTER TABLE public.conversations DROP COLUMN IF EXISTS canonical_name;
