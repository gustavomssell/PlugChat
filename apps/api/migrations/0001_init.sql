create table users (
  id text primary key,
  email text not null unique,
  name text not null,
  password_hash text not null,
  created_at timestamptz not null default now()
);

create table workspaces (
  id text primary key,
  name text not null,
  created_at timestamptz not null default now()
);

create table memberships (
  workspace_id text not null references workspaces(id) on delete cascade,
  user_id text not null references users(id) on delete cascade,
  role text not null check (role in ('owner', 'admin', 'agent')),
  availability text not null default 'offline' check (availability in ('online', 'busy', 'offline')),
  created_at timestamptz not null default now(),
  primary key (workspace_id, user_id)
);

create table flows (
  id text primary key,
  workspace_id text not null references workspaces(id) on delete cascade,
  name text not null,
  version int not null default 1,
  graph jsonb not null,
  updated_at timestamptz not null default now()
);

create index flows_workspace_idx on flows (workspace_id);

create table channels (
  id text primary key,
  workspace_id text not null references workspaces(id) on delete cascade,
  public_key text not null unique,
  name text not null,
  flow_id text not null references flows(id) on delete restrict,
  appearance jsonb not null default '{}'::jsonb,
  allowed_origins text[] not null default '{}',
  created_at timestamptz not null default now()
);

create index channels_workspace_idx on channels (workspace_id);

create table conversations (
  id text primary key,
  workspace_id text not null references workspaces(id) on delete cascade,
  channel_id text not null references channels(id) on delete cascade,
  visitor_id text not null,
  status text not null check (status in ('bot', 'waiting', 'queued', 'human', 'closed')),
  flow_id text not null,
  flow_version int not null,
  flow_snapshot jsonb not null,
  current_node_id text not null default '',
  state jsonb not null,
  assigned_user_id text references users(id) on delete set null,
  tags text[] not null default '{}',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index conversations_workspace_updated_idx on conversations (workspace_id, updated_at desc);
create index conversations_status_idx on conversations (workspace_id, status);
create index conversations_channel_idx on conversations (channel_id);

create table messages (
  id text primary key,
  conversation_id text not null references conversations(id) on delete cascade,
  role text not null check (role in ('visitor', 'bot', 'agent', 'system')),
  text text not null,
  created_at timestamptz not null default now()
);

create index messages_conversation_created_idx on messages (conversation_id, created_at);
