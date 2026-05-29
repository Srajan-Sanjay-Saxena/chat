-- Add type and display_name to conversations, make title optional and drop unique constraint
ALTER TABLE public.conversations ADD COLUMN IF NOT EXISTS type text NOT NULL DEFAULT 'group';
ALTER TABLE public.conversations ADD COLUMN IF NOT EXISTS display_name text;

-- Drop unique constraint on title if present (constraint name created by `title unique` is usually conversations_title_key)
ALTER TABLE public.conversations DROP CONSTRAINT IF EXISTS conversations_title_key;

-- Make title nullable so private conversations can omit it
ALTER TABLE public.conversations ALTER COLUMN title DROP NOT NULL;

-- Note: existing rows will default to type='group' and keep their titles
