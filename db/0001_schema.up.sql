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
    title text unique not null,
    created_at timestamptz not null default now()
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
    content text not null,
    created_at timestamptz not null default now()
);

create index if not exists idx_convpart_conv on public.conversation_participants(conversation_id);
create index if not exists idx_convpart_user on public.conversation_participants(user_id);
create index if not exists idx_messages_conv_created on public.messages(conversation_id, created_at desc);