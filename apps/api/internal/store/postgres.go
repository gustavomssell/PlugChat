package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chatfree/api/internal/flow"
	"chatfree/api/internal/model"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (p *Postgres) CreateUser(ctx context.Context, user model.User, passwordHash string) error {
	_, err := p.pool.Exec(ctx, `
		insert into users (id, email, name, password_hash, created_at)
		values ($1, $2, $3, $4, $5)
	`, user.ID, user.Email, user.Name, passwordHash, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("store: create user: %w", err)
	}
	return nil
}

func (p *Postgres) UserByEmail(ctx context.Context, email string) (model.User, string, error) {
	var user model.User
	var hash string

	err := p.pool.QueryRow(ctx, `
		select id, email, name, created_at, password_hash
		from users where email = $1
	`, email).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, "", ErrNotFound
	}
	if err != nil {
		return model.User{}, "", fmt.Errorf("store: user by email: %w", err)
	}

	return user, hash, nil
}

func (p *Postgres) UserByID(ctx context.Context, id string) (model.User, error) {
	var user model.User

	err := p.pool.QueryRow(ctx, `
		select id, email, name, created_at from users where id = $1
	`, id).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("store: user by id: %w", err)
	}

	return user, nil
}

func (p *Postgres) CountUsers(ctx context.Context) (int, error) {
	var count int
	if err := p.pool.QueryRow(ctx, `select count(*) from users`).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: count users: %w", err)
	}
	return count, nil
}

// CreateOwner grava o bootstrap de um workspace em uma transação só. Se
// qualquer inserção falhar, nada é mantido e o e-mail continua livre.
func (p *Postgres) CreateOwner(ctx context.Context, input OwnerInput) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin owner: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	appearance, err := json.Marshal(input.Channel.Appearance)
	if err != nil {
		return fmt.Errorf("store: encode appearance: %w", err)
	}
	graph, err := json.Marshal(input.StarterFlow)
	if err != nil {
		return fmt.Errorf("store: encode flow: %w", err)
	}

	statements := []struct {
		sql   string
		args  []any
		label string
	}{
		{
			sql:   `insert into users (id, email, name, password_hash, created_at) values ($1, $2, $3, $4, $5)`,
			args:  []any{input.User.ID, input.User.Email, input.User.Name, input.PasswordHash, input.User.CreatedAt},
			label: "user",
		},
		{
			sql:   `insert into workspaces (id, name, created_at) values ($1, $2, $3)`,
			args:  []any{input.Workspace.ID, input.Workspace.Name, input.Workspace.CreatedAt},
			label: "workspace",
		},
		{
			sql:   `insert into memberships (workspace_id, user_id, role, availability) values ($1, $2, $3, $4)`,
			args:  []any{input.Workspace.ID, input.User.ID, input.Role, "online"},
			label: "membership",
		},
		{
			sql:   `insert into flows (id, workspace_id, name, version, graph, updated_at) values ($1, $2, $3, $4, $5, $6)`,
			args:  []any{input.StarterFlow.ID, input.Workspace.ID, input.StarterFlow.Name, input.StarterFlow.Version, graph, input.StarterFlow.UpdatedAt},
			label: "flow",
		},
		{
			sql: `insert into channels (id, workspace_id, public_key, name, flow_id, appearance, allowed_origins, created_at)
				values ($1, $2, $3, $4, $5, $6, $7, $8)`,
			args:  []any{input.Channel.ID, input.Workspace.ID, input.Channel.Key, input.Channel.Name, input.Channel.FlowID, appearance, input.Channel.AllowedOrigins, input.Channel.CreatedAt},
			label: "channel",
		},
	}

	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.sql, statement.args...); err != nil {
			return fmt.Errorf("store: create owner %s: %w", statement.label, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit owner: %w", err)
	}
	return nil
}

// AddMember grava usuário e membership na mesma transação, para que um
// atendente nunca exista sem estar ligado a um workspace.
func (p *Postgres) AddMember(ctx context.Context, input MemberInput) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin member: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		insert into users (id, email, name, password_hash, created_at) values ($1, $2, $3, $4, $5)
	`, input.User.ID, input.User.Email, input.User.Name, input.PasswordHash, input.User.CreatedAt); err != nil {
		return fmt.Errorf("store: create member user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		insert into memberships (workspace_id, user_id, role, availability) values ($1, $2, $3, $4)
	`, input.WorkspaceID, input.User.ID, input.Role, input.Availability); err != nil {
		return fmt.Errorf("store: create member membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit member: %w", err)
	}
	return nil
}

func (p *Postgres) CreateWorkspace(ctx context.Context, workspace model.Workspace) error {
	_, err := p.pool.Exec(ctx, `
		insert into workspaces (id, name, created_at) values ($1, $2, $3)
	`, workspace.ID, workspace.Name, workspace.CreatedAt)
	if err != nil {
		return fmt.Errorf("store: create workspace: %w", err)
	}
	return nil
}

func (p *Postgres) WorkspaceForUser(ctx context.Context, userID string) (model.Workspace, model.Membership, error) {
	var workspace model.Workspace
	var membership model.Membership

	err := p.pool.QueryRow(ctx, `
		select w.id, w.name, w.created_at, m.role, m.availability
		from memberships m
		join workspaces w on w.id = m.workspace_id
		where m.user_id = $1
		order by m.created_at asc
		limit 1
	`, userID).Scan(&workspace.ID, &workspace.Name, &workspace.CreatedAt, &membership.Role, &membership.Availability)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Workspace{}, model.Membership{}, ErrNotFound
	}
	if err != nil {
		return model.Workspace{}, model.Membership{}, fmt.Errorf("store: workspace for user: %w", err)
	}

	return workspace, membership, nil
}

func (p *Postgres) CreateMembership(ctx context.Context, workspaceID, userID, role, availability string) error {
	_, err := p.pool.Exec(ctx, `
		insert into memberships (workspace_id, user_id, role, availability)
		values ($1, $2, $3, $4)
	`, workspaceID, userID, role, availability)
	if err != nil {
		return fmt.Errorf("store: create membership: %w", err)
	}
	return nil
}

func (p *Postgres) Members(ctx context.Context, workspaceID string) ([]model.Agent, error) {
	rows, err := p.pool.Query(ctx, `
		select u.id, u.name, u.email, m.role, m.availability
		from memberships m
		join users u on u.id = m.user_id
		where m.workspace_id = $1
		order by u.name asc
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("store: members: %w", err)
	}
	defer rows.Close()

	agents := make([]model.Agent, 0)
	for rows.Next() {
		var agent model.Agent
		if err := rows.Scan(&agent.UserID, &agent.Name, &agent.Email, &agent.Role, &agent.Availability); err != nil {
			return nil, fmt.Errorf("store: scan member: %w", err)
		}
		agents = append(agents, agent)
	}

	return agents, rows.Err()
}

func (p *Postgres) UpdateAvailability(ctx context.Context, workspaceID, userID, availability string) error {
	tag, err := p.pool.Exec(ctx, `
		update memberships set availability = $3 where workspace_id = $1 and user_id = $2
	`, workspaceID, userID, availability)
	if err != nil {
		return fmt.Errorf("store: update availability: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) ChannelByKey(ctx context.Context, key string) (model.Channel, error) {
	return p.scanChannel(p.pool.QueryRow(ctx, `
		select id, workspace_id, public_key, name, flow_id, appearance, allowed_origins, created_at
		from channels where public_key = $1
	`, key))
}

func (p *Postgres) Channels(ctx context.Context, workspaceID string) ([]model.Channel, error) {
	rows, err := p.pool.Query(ctx, `
		select id, workspace_id, public_key, name, flow_id, appearance, allowed_origins, created_at
		from channels where workspace_id = $1
		order by created_at asc
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("store: channels: %w", err)
	}
	defer rows.Close()

	channels := make([]model.Channel, 0)
	for rows.Next() {
		channel, err := p.scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}

	return channels, rows.Err()
}

func (p *Postgres) Channel(ctx context.Context, workspaceID, id string) (model.Channel, error) {
	return p.scanChannel(p.pool.QueryRow(ctx, `
		select id, workspace_id, public_key, name, flow_id, appearance, allowed_origins, created_at
		from channels where workspace_id = $1 and id = $2
	`, workspaceID, id))
}

func (p *Postgres) scanChannel(row pgx.Row) (model.Channel, error) {
	var channel model.Channel
	var appearance []byte

	err := row.Scan(
		&channel.ID,
		&channel.WorkspaceID,
		&channel.Key,
		&channel.Name,
		&channel.FlowID,
		&appearance,
		&channel.AllowedOrigins,
		&channel.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Channel{}, ErrNotFound
	}
	if err != nil {
		return model.Channel{}, fmt.Errorf("store: scan channel: %w", err)
	}
	if len(appearance) > 0 {
		if err := json.Unmarshal(appearance, &channel.Appearance); err != nil {
			return model.Channel{}, fmt.Errorf("store: decode appearance: %w", err)
		}
	}
	if channel.AllowedOrigins == nil {
		channel.AllowedOrigins = []string{}
	}

	return channel, nil
}

func (p *Postgres) CreateChannel(ctx context.Context, channel model.Channel) error {
	appearance, err := json.Marshal(channel.Appearance)
	if err != nil {
		return fmt.Errorf("store: encode appearance: %w", err)
	}

	_, err = p.pool.Exec(ctx, `
		insert into channels (id, workspace_id, public_key, name, flow_id, appearance, allowed_origins, created_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
	`, channel.ID, channel.WorkspaceID, channel.Key, channel.Name, channel.FlowID, appearance, channel.AllowedOrigins, channel.CreatedAt)
	if err != nil {
		return fmt.Errorf("store: create channel: %w", err)
	}
	return nil
}

func (p *Postgres) UpdateChannel(ctx context.Context, channel model.Channel) error {
	appearance, err := json.Marshal(channel.Appearance)
	if err != nil {
		return fmt.Errorf("store: encode appearance: %w", err)
	}

	tag, err := p.pool.Exec(ctx, `
		update channels
		set name = $3, flow_id = $4, appearance = $5, allowed_origins = $6
		where workspace_id = $1 and id = $2
	`, channel.WorkspaceID, channel.ID, channel.Name, channel.FlowID, appearance, channel.AllowedOrigins)
	if err != nil {
		return fmt.Errorf("store: update channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) DeleteChannel(ctx context.Context, workspaceID, id string) error {
	tag, err := p.pool.Exec(ctx, `delete from channels where workspace_id = $1 and id = $2`, workspaceID, id)
	if err != nil {
		return fmt.Errorf("store: delete channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) Flows(ctx context.Context, workspaceID string) ([]flow.Graph, error) {
	rows, err := p.pool.Query(ctx, `
		select graph, updated_at from flows where workspace_id = $1 order by name asc
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("store: flows: %w", err)
	}
	defer rows.Close()

	graphs := make([]flow.Graph, 0)
	for rows.Next() {
		var raw []byte
		var updatedAt time.Time
		if err := rows.Scan(&raw, &updatedAt); err != nil {
			return nil, fmt.Errorf("store: scan flow: %w", err)
		}
		var graph flow.Graph
		if err := json.Unmarshal(raw, &graph); err != nil {
			return nil, fmt.Errorf("store: decode flow: %w", err)
		}
		graph.UpdatedAt = updatedAt
		graphs = append(graphs, graph)
	}

	return graphs, rows.Err()
}

func (p *Postgres) Flow(ctx context.Context, workspaceID, id string) (flow.Graph, error) {
	var raw []byte
	var updatedAt time.Time

	err := p.pool.QueryRow(ctx, `
		select graph, updated_at from flows where workspace_id = $1 and id = $2
	`, workspaceID, id).Scan(&raw, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return flow.Graph{}, ErrNotFound
	}
	if err != nil {
		return flow.Graph{}, fmt.Errorf("store: flow: %w", err)
	}

	var graph flow.Graph
	if err := json.Unmarshal(raw, &graph); err != nil {
		return flow.Graph{}, fmt.Errorf("store: decode flow: %w", err)
	}
	graph.UpdatedAt = updatedAt

	return graph, nil
}

func (p *Postgres) CreateFlow(ctx context.Context, workspaceID string, graph flow.Graph) error {
	graph.UpdatedAt = time.Now().UTC()
	raw, err := json.Marshal(graph)
	if err != nil {
		return fmt.Errorf("store: encode flow: %w", err)
	}

	_, err = p.pool.Exec(ctx, `
		insert into flows (id, workspace_id, name, version, graph, updated_at)
		values ($1, $2, $3, $4, $5, $6)
	`, graph.ID, workspaceID, graph.Name, graph.Version, raw, graph.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: create flow: %w", err)
	}
	return nil
}

func (p *Postgres) SaveFlow(ctx context.Context, workspaceID string, graph flow.Graph) error {
	graph.UpdatedAt = time.Now().UTC()
	raw, err := json.Marshal(graph)
	if err != nil {
		return fmt.Errorf("store: encode flow: %w", err)
	}

	tag, err := p.pool.Exec(ctx, `
		update flows set name = $3, version = $4, graph = $5, updated_at = $6
		where workspace_id = $1 and id = $2
	`, workspaceID, graph.ID, graph.Name, graph.Version, raw, graph.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: save flow: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) DeleteFlow(ctx context.Context, workspaceID, id string) error {
	tag, err := p.pool.Exec(ctx, `delete from flows where workspace_id = $1 and id = $2`, workspaceID, id)
	if err != nil {
		return fmt.Errorf("store: delete flow: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) CreateConversation(ctx context.Context, conversation model.Conversation, state flow.State, snapshot flow.Graph) error {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("store: encode state: %w", err)
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("store: encode snapshot: %w", err)
	}

	_, err = p.pool.Exec(ctx, `
		insert into conversations (
			id, workspace_id, channel_id, visitor_id, status, flow_id, flow_version,
			flow_snapshot, current_node_id, state, assigned_user_id, tags, created_at, updated_at
		) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, nullif($11, ''), $12, $13, $14)
	`, conversation.ID, conversation.WorkspaceID, conversation.ChannelID, conversation.VisitorID,
		conversation.Status, conversation.FlowID, conversation.FlowVersion, snapshotJSON,
		conversation.CurrentNodeID, stateJSON, conversation.AssignedUserID, conversation.Tags,
		conversation.CreatedAt, conversation.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: create conversation: %w", err)
	}
	return nil
}

const conversationColumns = `
	id, workspace_id, channel_id, visitor_id, status, flow_id, flow_version,
	current_node_id, coalesce(assigned_user_id, ''), tags, state, created_at, updated_at
`

func (p *Postgres) Conversation(ctx context.Context, id string) (model.Conversation, error) {
	return p.scanConversation(p.pool.QueryRow(ctx, `
		select `+conversationColumns+` from conversations where id = $1
	`, id))
}

func (p *Postgres) ConversationInWorkspace(ctx context.Context, workspaceID, id string) (model.Conversation, error) {
	return p.scanConversation(p.pool.QueryRow(ctx, `
		select `+conversationColumns+` from conversations where workspace_id = $1 and id = $2
	`, workspaceID, id))
}

func (p *Postgres) scanConversation(row pgx.Row) (model.Conversation, error) {
	var conversation model.Conversation
	var stateJSON []byte

	err := row.Scan(
		&conversation.ID,
		&conversation.WorkspaceID,
		&conversation.ChannelID,
		&conversation.VisitorID,
		&conversation.Status,
		&conversation.FlowID,
		&conversation.FlowVersion,
		&conversation.CurrentNodeID,
		&conversation.AssignedUserID,
		&conversation.Tags,
		&stateJSON,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Conversation{}, ErrNotFound
	}
	if err != nil {
		return model.Conversation{}, fmt.Errorf("store: scan conversation: %w", err)
	}
	if conversation.Tags == nil {
		conversation.Tags = []string{}
	}
	if len(stateJSON) > 0 {
		var state flow.State
		if err := json.Unmarshal(stateJSON, &state); err != nil {
			return model.Conversation{}, fmt.Errorf("store: decode conversation state: %w", err)
		}
		conversation.Waiting = state.Waiting
	}
	return conversation, nil
}

// escapeLike neutraliza os curingas de um termo de busca. Sem isso um "%"
// digitado pelo usuário casaria com tudo e um "_" com qualquer caractere.
func escapeLike(term string) string {
	if term == "" {
		return ""
	}
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(term)
}

func (p *Postgres) Conversations(ctx context.Context, workspaceID string, filter ConversationFilter) ([]model.Conversation, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 200
	}

	rows, err := p.pool.Query(ctx, `
		select `+conversationColumns+`
		from conversations
		where workspace_id = $1
		  and ($2 = '' or status = $2)
		  and ($3 = '' or channel_id = $3)
		  and (
		    $4 = ''
		    or visitor_id ilike '%' || $4 || '%' escape '\'
		    or id ilike '%' || $4 || '%' escape '\'
		    or array_to_string(tags, ' ') ilike '%' || $4 || '%' escape '\'
		  )
		order by updated_at desc
		limit $5
	`, workspaceID, filter.Status, filter.ChannelID, escapeLike(filter.Search), limit)
	if err != nil {
		return nil, fmt.Errorf("store: conversations: %w", err)
	}
	defer rows.Close()

	conversations := make([]model.Conversation, 0)
	for rows.Next() {
		conversation, err := p.scanConversation(rows)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, conversation)
	}

	return conversations, rows.Err()
}

func (p *Postgres) DueConversations(ctx context.Context, limit int) ([]model.Conversation, error) {
	rows, err := p.pool.Query(ctx, `
		select `+conversationColumns+`
		from conversations
		where coalesce(state->>'pendingNodeId', '') <> ''
		  and (state->>'wakeAt') is not null
		  and (state->>'wakeAt')::timestamptz <= now()
		order by (state->>'wakeAt')::timestamptz asc
		limit $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: due conversations: %w", err)
	}
	defer rows.Close()

	conversations := make([]model.Conversation, 0)
	for rows.Next() {
		conversation, err := p.scanConversation(rows)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, conversation)
	}

	return conversations, rows.Err()
}

func (p *Postgres) UpdateConversation(ctx context.Context, conversation model.Conversation, state flow.State) error {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("store: encode state: %w", err)
	}

	tag, err := p.pool.Exec(ctx, `
		update conversations
		set status = $2, current_node_id = $3, flow_version = $4, state = $5,
		    assigned_user_id = nullif($6, ''), tags = $7, updated_at = $8
		where id = $1
	`, conversation.ID, conversation.Status, conversation.CurrentNodeID, conversation.FlowVersion,
		stateJSON, conversation.AssignedUserID, conversation.Tags, conversation.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: update conversation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) UpdateConversationMeta(ctx context.Context, conversation model.Conversation) error {
	tag, err := p.pool.Exec(ctx, `
		update conversations
		set status = $2, assigned_user_id = nullif($3, ''), tags = $4, updated_at = $5
		where id = $1
	`, conversation.ID, conversation.Status, conversation.AssignedUserID, conversation.Tags, conversation.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: update conversation meta: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) State(ctx context.Context, conversationID string) (flow.State, error) {
	var raw []byte
	err := p.pool.QueryRow(ctx, `select state from conversations where id = $1`, conversationID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return flow.State{}, ErrNotFound
	}
	if err != nil {
		return flow.State{}, fmt.Errorf("store: state: %w", err)
	}

	var state flow.State
	if err := json.Unmarshal(raw, &state); err != nil {
		return flow.State{}, fmt.Errorf("store: decode state: %w", err)
	}
	return state, nil
}

func (p *Postgres) FlowSnapshot(ctx context.Context, conversationID string) (flow.Graph, error) {
	var raw []byte
	err := p.pool.QueryRow(ctx, `select flow_snapshot from conversations where id = $1`, conversationID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return flow.Graph{}, ErrNotFound
	}
	if err != nil {
		return flow.Graph{}, fmt.Errorf("store: flow snapshot: %w", err)
	}

	var graph flow.Graph
	if err := json.Unmarshal(raw, &graph); err != nil {
		return flow.Graph{}, fmt.Errorf("store: decode flow snapshot: %w", err)
	}
	return graph, nil
}

// AppendMessages stores messages with their attachment snapshot. Internal
// notes keep the author so the inbox can label them.
func (p *Postgres) AppendMessages(ctx context.Context, messages ...model.Message) error {
	if len(messages) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, message := range messages {
		// nil serializaria como "null"; gravamos "[]" no lugar.
		attachments := []byte("[]")
		if message.Attachments != nil {
			encoded, err := json.Marshal(message.Attachments)
			if err != nil {
				return fmt.Errorf("store: encode attachments: %w", err)
			}
			attachments = encoded
		}

		batch.Queue(`
			insert into messages (id, conversation_id, role, text, author_user_id, author_name, attachments, created_at)
			values ($1, $2, $3, $4, nullif($5, ''), $6, $7, $8)
		`, message.ID, message.ConversationID, message.Role, message.Text,
			message.AuthorUserID, message.AuthorName, attachments, message.CreatedAt)
	}

	results := p.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range messages {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("store: append message: %w", err)
		}
	}

	return nil
}

// Messages is the agent view: notes are part of the history.
func (p *Postgres) Messages(ctx context.Context, conversationID string) ([]model.Message, error) {
	return p.messages(ctx, conversationID, false)
}

// MessagesForVisitor is the public view: internal notes never leave the team.
func (p *Postgres) MessagesForVisitor(ctx context.Context, conversationID string) ([]model.Message, error) {
	return p.messages(ctx, conversationID, true)
}

func (p *Postgres) messages(ctx context.Context, conversationID string, hideNotes bool) ([]model.Message, error) {
	query := `
		select id, conversation_id, role, text, coalesce(author_user_id, ''), author_name, attachments, created_at
		from messages
		where conversation_id = $1`
	if hideNotes {
		query += ` and role <> 'note'`
	}
	query += `
		order by created_at asc, id asc`

	rows, err := p.pool.Query(ctx, query, conversationID)
	if err != nil {
		return nil, fmt.Errorf("store: messages: %w", err)
	}
	defer rows.Close()

	messages := make([]model.Message, 0)
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: messages: %w", err)
	}

	return messages, nil
}

func scanMessage(row pgx.Row) (model.Message, error) {
	var message model.Message
	var attachments []byte

	err := row.Scan(
		&message.ID,
		&message.ConversationID,
		&message.Role,
		&message.Text,
		&message.AuthorUserID,
		&message.AuthorName,
		&attachments,
		&message.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Message{}, ErrNotFound
	}
	if err != nil {
		return model.Message{}, fmt.Errorf("store: scan message: %w", err)
	}

	if len(attachments) > 0 && string(attachments) != "null" {
		if err := json.Unmarshal(attachments, &message.Attachments); err != nil {
			return model.Message{}, fmt.Errorf("store: decode message attachments: %w", err)
		}
	}

	return message, nil
}

func (p *Postgres) CreateAttachment(ctx context.Context, attachment model.Attachment) error {
	_, err := p.pool.Exec(ctx, `
		insert into attachments (id, workspace_id, conversation_id, kind, filename, mime, size_bytes, stored_name, created_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, attachment.ID, attachment.WorkspaceID, attachment.ConversationID, attachment.Kind,
		attachment.Filename, attachment.Mime, attachment.SizeBytes, attachment.StoredName, attachment.CreatedAt)
	if err != nil {
		return fmt.Errorf("store: create attachment: %w", err)
	}
	return nil
}

const attachmentColumns = `
	id, conversation_id, kind, filename, mime, size_bytes, stored_name, created_at
`

func (p *Postgres) Attachment(ctx context.Context, workspaceID, id string) (model.Attachment, error) {
	var attachment model.Attachment

	err := p.pool.QueryRow(ctx, `
		select `+attachmentColumns+` from attachments where workspace_id = $1 and id = $2
	`, workspaceID, id).Scan(&attachment.ID, &attachment.ConversationID, &attachment.Kind,
		&attachment.Filename, &attachment.Mime, &attachment.SizeBytes, &attachment.StoredName, &attachment.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Attachment{}, ErrNotFound
	}
	if err != nil {
		return model.Attachment{}, fmt.Errorf("store: attachment: %w", err)
	}

	return attachment, nil
}

func (p *Postgres) AttachmentsByConversation(ctx context.Context, conversationID string) ([]model.Attachment, error) {
	rows, err := p.pool.Query(ctx, `
		select `+attachmentColumns+` from attachments where conversation_id = $1 order by created_at asc
	`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("store: attachments by conversation: %w", err)
	}
	defer rows.Close()

	attachments := make([]model.Attachment, 0)
	for rows.Next() {
		attachment, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}

	return attachments, rows.Err()
}

func (p *Postgres) AttachmentsByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.Attachment, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := p.pool.Query(ctx, `
		select `+attachmentColumns+`
		from attachments
		where workspace_id = $1 and id = any($2)
		order by created_at asc
	`, workspaceID, ids)
	if err != nil {
		return nil, fmt.Errorf("store: attachments by ids: %w", err)
	}
	defer rows.Close()

	attachments := make([]model.Attachment, 0, len(ids))
	for rows.Next() {
		attachment, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}

	return attachments, rows.Err()
}

func scanAttachment(row pgx.Row) (model.Attachment, error) {
	var attachment model.Attachment

	err := row.Scan(&attachment.ID, &attachment.ConversationID, &attachment.Kind,
		&attachment.Filename, &attachment.Mime, &attachment.SizeBytes, &attachment.StoredName, &attachment.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Attachment{}, ErrNotFound
	}
	if err != nil {
		return model.Attachment{}, fmt.Errorf("store: scan attachment: %w", err)
	}

	return attachment, nil
}

func (p *Postgres) LinkAttachments(ctx context.Context, messageID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	_, err := p.pool.Exec(ctx, `
		update attachments set message_id = $1 where id = any($2)
	`, messageID, ids)
	if err != nil {
		return fmt.Errorf("store: link attachments: %w", err)
	}
	return nil
}

func (p *Postgres) Stats(ctx context.Context, workspaceID string) (model.Stats, error) {
	var stats model.Stats

	err := p.pool.QueryRow(ctx, `
		select
			count(*),
			count(*) filter (where status in ('bot', 'human')),
			count(*) filter (where status in ('waiting', 'queued')),
			count(*) filter (where status = 'closed')
		from conversations where workspace_id = $1
	`, workspaceID).Scan(&stats.Total, &stats.Active, &stats.Waiting, &stats.Closed)
	if err != nil {
		return model.Stats{}, fmt.Errorf("store: stats: %w", err)
	}

	err = p.pool.QueryRow(ctx, `
		select count(*)
		from messages m
		join conversations c on c.id = m.conversation_id
		where c.workspace_id = $1 and m.created_at >= date_trunc('day', now())
	`, workspaceID).Scan(&stats.MessagesToday)
	if err != nil {
		return model.Stats{}, fmt.Errorf("store: stats messages: %w", err)
	}

	rows, err := p.pool.Query(ctx, `
		select to_char(day, 'YYYY-MM-DD'), count(c.id)
		from generate_series(current_date - interval '6 days', current_date, interval '1 day') as day
		left join conversations c
			on c.workspace_id = $1
			and c.created_at >= day
			and c.created_at < day + interval '1 day'
		group by day
		order by day asc
	`, workspaceID)
	if err != nil {
		return model.Stats{}, fmt.Errorf("store: stats series: %w", err)
	}
	defer rows.Close()

	stats.ConversationsByDay = make([]model.DayCount, 0, 7)
	for rows.Next() {
		var item model.DayCount
		if err := rows.Scan(&item.Date, &item.Count); err != nil {
			return model.Stats{}, fmt.Errorf("store: scan stats day: %w", err)
		}
		stats.ConversationsByDay = append(stats.ConversationsByDay, item)
	}

	return stats, rows.Err()
}

// Metrics builds the report consumed by the dashboard reports page. First
// response time is the gap between the conversation start and the first
// message that came from the team side (bot or agent).
func (p *Postgres) Metrics(ctx context.Context, workspaceID string, days int) (model.Metrics, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}

	metrics := model.Metrics{
		Days:               days,
		ByChannel:          make([]model.ChannelMetric, 0),
		ByAgent:            make([]model.AgentMetric, 0),
		ConversationsByDay: make([]model.DayCount, 0, days+1),
		MessagesByDay:      make([]model.DayCount, 0, days+1),
	}

	err := p.pool.QueryRow(ctx, `
		select
			count(*),
			count(*) filter (where status = 'closed'),
			coalesce(avg(first_response_ms), 0)::bigint
		from (
			select c.id, c.status,
				(extract(epoch from (m.created_at - c.created_at)) * 1000)::bigint as first_response_ms
			from conversations c
			left join lateral (
				select created_at from messages
				where conversation_id = c.id and role in ('agent', 'bot')
				order by created_at asc limit 1
			) m on true
			where c.workspace_id = $1 and c.created_at >= now() - make_interval(days => $2::int)
		) t
	`, workspaceID, days).Scan(&metrics.Conversations, &metrics.Closed, &metrics.FirstResponseMS)
	if err != nil {
		return model.Metrics{}, fmt.Errorf("store: metrics totals: %w", err)
	}

	err = p.pool.QueryRow(ctx, `
		select count(*)
		from messages m
		join conversations c on c.id = m.conversation_id
		where c.workspace_id = $1 and m.created_at >= now() - make_interval(days => $2::int)
	`, workspaceID, days).Scan(&metrics.Messages)
	if err != nil {
		return model.Metrics{}, fmt.Errorf("store: metrics messages: %w", err)
	}

	rows, err := p.pool.Query(ctx, `
		select to_char(day, 'YYYY-MM-DD'), count(c.id)
		from generate_series(current_date - ($2::int * interval '1 day'), current_date, interval '1 day') as day
		left join conversations c
			on c.workspace_id = $1
			and c.created_at >= day
			and c.created_at < day + interval '1 day'
		group by day
		order by day asc
	`, workspaceID, days)
	if err != nil {
		return model.Metrics{}, fmt.Errorf("store: metrics conversations series: %w", err)
	}
	for rows.Next() {
		var item model.DayCount
		if err := rows.Scan(&item.Date, &item.Count); err != nil {
			rows.Close()
			return model.Metrics{}, fmt.Errorf("store: scan metrics day: %w", err)
		}
		metrics.ConversationsByDay = append(metrics.ConversationsByDay, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return model.Metrics{}, fmt.Errorf("store: metrics conversations series: %w", err)
	}
	rows.Close()

	rows, err = p.pool.Query(ctx, `
		select to_char(day, 'YYYY-MM-DD'), count(m.id)
		from generate_series(current_date - ($2::int * interval '1 day'), current_date, interval '1 day') as day
		left join messages m
			on m.created_at >= day
			and m.created_at < day + interval '1 day'
			and exists (
				select 1 from conversations c
				where c.id = m.conversation_id and c.workspace_id = $1
			)
		group by day
		order by day asc
	`, workspaceID, days)
	if err != nil {
		return model.Metrics{}, fmt.Errorf("store: metrics messages series: %w", err)
	}
	for rows.Next() {
		var item model.DayCount
		if err := rows.Scan(&item.Date, &item.Count); err != nil {
			rows.Close()
			return model.Metrics{}, fmt.Errorf("store: scan metrics message day: %w", err)
		}
		metrics.MessagesByDay = append(metrics.MessagesByDay, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return model.Metrics{}, fmt.Errorf("store: metrics messages series: %w", err)
	}
	rows.Close()

	channelRows, err := p.pool.Query(ctx, `
		select ch.id, ch.name, count(c.id), count(c.id) filter (where c.status = 'closed'),
			coalesce(avg(extract(epoch from (m.created_at - c.created_at)) * 1000), 0)::bigint
		from channels ch
		left join conversations c
			on c.channel_id = ch.id
			and c.created_at >= now() - make_interval(days => $2::int)
		left join lateral (
			select created_at from messages
			where conversation_id = c.id and role in ('agent', 'bot')
			order by created_at asc limit 1
		) m on true
		where ch.workspace_id = $1
		group by ch.id, ch.name
		order by count(c.id) desc, ch.name asc
	`, workspaceID, days)
	if err != nil {
		return model.Metrics{}, fmt.Errorf("store: metrics channels: %w", err)
	}
	defer channelRows.Close()
	for channelRows.Next() {
		var item model.ChannelMetric
		if err := channelRows.Scan(&item.ChannelID, &item.Name, &item.Conversations, &item.Closed, &item.FirstResponseMS); err != nil {
			return model.Metrics{}, fmt.Errorf("store: scan metrics channel: %w", err)
		}
		metrics.ByChannel = append(metrics.ByChannel, item)
	}
	if err := channelRows.Err(); err != nil {
		return model.Metrics{}, fmt.Errorf("store: metrics channels: %w", err)
	}

	agentRows, err := p.pool.Query(ctx, `
		select u.id, u.name, count(c.id), count(c.id) filter (where c.status = 'closed'),
			coalesce(avg(extract(epoch from (m.created_at - c.created_at)) * 1000), 0)::bigint
		from memberships mem
		join users u on u.id = mem.user_id
		left join conversations c
			on c.assigned_user_id = u.id
			and c.workspace_id = mem.workspace_id
			and c.created_at >= now() - make_interval(days => $2::int)
		left join lateral (
			select created_at from messages
			where conversation_id = c.id and role in ('agent', 'bot')
			order by created_at asc limit 1
		) m on true
		where mem.workspace_id = $1
		group by u.id, u.name
		order by count(c.id) desc, u.name asc
	`, workspaceID, days)
	if err != nil {
		return model.Metrics{}, fmt.Errorf("store: metrics agents: %w", err)
	}
	defer agentRows.Close()
	for agentRows.Next() {
		var item model.AgentMetric
		if err := agentRows.Scan(&item.UserID, &item.Name, &item.Conversations, &item.Closed, &item.FirstResponseMS); err != nil {
			return model.Metrics{}, fmt.Errorf("store: scan metrics agent: %w", err)
		}
		metrics.ByAgent = append(metrics.ByAgent, item)
	}
	if err := agentRows.Err(); err != nil {
		return model.Metrics{}, fmt.Errorf("store: metrics agents: %w", err)
	}

	return metrics, nil
}

func (p *Postgres) SavedReplies(ctx context.Context, workspaceID string) ([]model.SavedReply, error) {
	rows, err := p.pool.Query(ctx, `
		select id, workspace_id, title, body, shortcut, tags, created_at, updated_at
		from saved_replies
		where workspace_id = $1
		order by title asc
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("store: saved replies: %w", err)
	}
	defer rows.Close()

	replies := make([]model.SavedReply, 0)
	for rows.Next() {
		reply, err := scanSavedReply(rows)
		if err != nil {
			return nil, err
		}
		replies = append(replies, reply)
	}

	return replies, rows.Err()
}

func (p *Postgres) SavedReply(ctx context.Context, workspaceID, id string) (model.SavedReply, error) {
	return scanSavedReply(p.pool.QueryRow(ctx, `
		select id, workspace_id, title, body, shortcut, tags, created_at, updated_at
		from saved_replies
		where workspace_id = $1 and id = $2
	`, workspaceID, id))
}

func (p *Postgres) CreateSavedReply(ctx context.Context, reply model.SavedReply) error {
	_, err := p.pool.Exec(ctx, `
		insert into saved_replies (id, workspace_id, title, body, shortcut, tags, created_at, updated_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
	`, reply.ID, reply.WorkspaceID, reply.Title, reply.Body, reply.Shortcut, reply.Tags, reply.CreatedAt, reply.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: create saved reply: %w", err)
	}
	return nil
}

func (p *Postgres) UpdateSavedReply(ctx context.Context, reply model.SavedReply) error {
	tag, err := p.pool.Exec(ctx, `
		update saved_replies
		set title = $3, body = $4, shortcut = $5, tags = $6, updated_at = $7
		where workspace_id = $1 and id = $2
	`, reply.WorkspaceID, reply.ID, reply.Title, reply.Body, reply.Shortcut, reply.Tags, reply.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: update saved reply: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) DeleteSavedReply(ctx context.Context, workspaceID, id string) error {
	tag, err := p.pool.Exec(ctx, `delete from saved_replies where workspace_id = $1 and id = $2`, workspaceID, id)
	if err != nil {
		return fmt.Errorf("store: delete saved reply: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSavedReply(row pgx.Row) (model.SavedReply, error) {
	var reply model.SavedReply

	err := row.Scan(&reply.ID, &reply.WorkspaceID, &reply.Title, &reply.Body,
		&reply.Shortcut, &reply.Tags, &reply.CreatedAt, &reply.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.SavedReply{}, ErrNotFound
	}
	if err != nil {
		return model.SavedReply{}, fmt.Errorf("store: scan saved reply: %w", err)
	}
	if reply.Tags == nil {
		reply.Tags = []string{}
	}

	return reply, nil
}
