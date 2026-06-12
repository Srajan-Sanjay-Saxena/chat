create extension if not exists "uuid-ossp";

create table if not exists public.users (
    id uuid primary key default uuid_generate_v4(),
    username text unique not null,
    password_hash text not null,
    email text unique not null,
    created_at timestamptz not null default now()
);

create table if not exists public.conversations (
    id uuid primary key default uuid_generate_v4(),
    title text unique,
    created_at timestamptz not null default now(),
    type text not null default 'group',
    display_name text,
    canonical_name text unique
);

create table if not exists public.conversation_participants (
    id uuid primary key default uuid_generate_v4(),
    conversation_id uuid not null references public.conversations(id) on delete cascade,
    user_id uuid not null references public.users(id) on delete cascade,
    created_at timestamptz not null default now(),
    unique (conversation_id, user_id)
);

create table if not exists public.messages (
    id uuid primary key default uuid_generate_v4(),
    conversation_id uuid not null references public.conversations(id) on delete cascade,
    sender_id uuid not null references public.users(id) on delete cascade,
    sender_username text not null,
    content text not null,
    created_at timestamptz not null default now()
);

-- Conversation participant lookups
CREATE UNIQUE INDEX IF NOT EXISTS idx_convpart_conv_user
ON public.conversation_participants (conversation_id, user_id);

-- Optional: keep only if you frequently query by user_id alone
CREATE INDEX IF NOT EXISTS idx_convpart_user
ON public.conversation_participants (user_id);

-- Message pagination
CREATE INDEX IF NOT EXISTS idx_messages_conv_created
ON public.messages (conversation_id, created_at DESC, id DESC);

-- Private conversation uniqueness
CREATE UNIQUE INDEX IF NOT EXISTS idx_conversations_canonical_private
ON public.conversations (canonical_name)
WHERE type = 'private';