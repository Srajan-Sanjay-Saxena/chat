-- Revert conversation type/display_name changes
ALTER TABLE public.conversations DROP COLUMN IF EXISTS display_name;
ALTER TABLE public.conversations DROP COLUMN IF EXISTS type;

-- Re-add unique constraint on title and set NOT NULL
ALTER TABLE public.conversations ALTER COLUMN title SET NOT NULL;
ALTER TABLE public.conversations ADD CONSTRAINT conversations_title_key UNIQUE (title);
