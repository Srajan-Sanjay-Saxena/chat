-- Add canonical_name column and unique partial index for private conversations
ALTER TABLE public.conversations ADD COLUMN IF NOT EXISTS canonical_name text;

CREATE UNIQUE INDEX IF NOT EXISTS idx_conversations_canonical_private ON public.conversations(canonical_name) WHERE (type = 'private');
