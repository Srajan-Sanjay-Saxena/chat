create extension if not exists "uuid-ossp";

create table if not exists users (
    id uuid primary key default uuid_generate_v4(),
    username text unique not null,
    password_hash text not null,
    email text unique not null,
    created_at timestamptz not null default now()
);

create table if not exists conversations (
    id uuid primary key default uuid_generate_v4(),
    title text unique not null,
    created_at timestamptz not null default now()
);

create table if not exists conversation_participants (
    id uuid primary key default uuid_generate_v4(),
    conversation_id uuid not null references conversations(id) on delete cascade,
    user_id uuid not null references users(id) on delete cascade,
    created_at timestamptz not null default now(),
    unique (conversation_id, user_id)
);

create table if not exists messages (
    id uuid primary key default uuid_generate_v4(),
    conversation_id uuid not null references conversations(id) on delete cascade,
    sender_id uuid not null references users(id) on delete cascade,
    content text not null,
    created_at timestamptz not null default now()
);

create index if not exists idx_convpart_conv on conversation_participants(conversation_id);
create index if not exists idx_convpart_user on conversation_participants(user_id);
create index if not exists idx_messages_conv_created on messages(conversation_id, created_at desc);