-- Notas internas: mensagens visíveis apenas para a equipe.
alter table messages drop constraint if exists messages_role_check;
alter table messages add constraint messages_role_check
  check (role in ('visitor', 'bot', 'agent', 'system', 'note'));

alter table messages
  add column if not exists author_user_id text,
  add column if not exists author_name text not null default '',
  add column if not exists attachments jsonb not null default '[]'::jsonb;

-- Respostas prontas e macros do workspace.
create table if not exists saved_replies (
  id text primary key,
  workspace_id text not null references workspaces(id) on delete cascade,
  title text not null,
  body text not null,
  shortcut text not null default '',
  tags text[] not null default '{}',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists saved_replies_workspace_idx on saved_replies (workspace_id);

-- Anexos (imagens, áudio e arquivos) enviados pelo painel ou pelo widget.
create table if not exists attachments (
  id text primary key,
  workspace_id text not null references workspaces(id) on delete cascade,
  conversation_id text not null references conversations(id) on delete cascade,
  message_id text,
  kind text not null check (kind in ('image', 'audio', 'file')),
  filename text not null,
  mime text not null,
  size_bytes bigint not null,
  stored_name text not null,
  created_at timestamptz not null default now()
);

create index if not exists attachments_conversation_idx on attachments (conversation_id, created_at);
create index if not exists attachments_message_idx on attachments (message_id);
