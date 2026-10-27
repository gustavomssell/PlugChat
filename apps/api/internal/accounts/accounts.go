package accounts

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"chatfree/api/internal/auth"
	"chatfree/api/internal/flow"
	"chatfree/api/internal/id"
	"chatfree/api/internal/model"
	"chatfree/api/internal/store"
)

var (
	ErrEmailTaken          = errors.New("accounts: email already taken")
	ErrInvalidCredentials  = errors.New("accounts: invalid credentials")
	ErrInvalidInput        = errors.New("accounts: invalid input")
	ErrInvalidAvailability = errors.New("accounts: invalid availability")
)

type Service struct {
	store  store.Store
	tokens *auth.Manager
}

func New(st store.Store, tokens *auth.Manager) *Service {
	return &Service{store: st, tokens: tokens}
}

type RegisterInput struct {
	Name          string
	Email         string
	Password      string
	WorkspaceName string
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (model.AuthResponse, error) {
	name := strings.TrimSpace(input.Name)
	email := normalizeEmail(input.Email)
	workspaceName := strings.TrimSpace(input.WorkspaceName)

	if len(name) < 2 || email == "" || len(input.Password) < 8 {
		return model.AuthResponse{}, ErrInvalidInput
	}
	if workspaceName == "" {
		workspaceName = "Meu workspace"
	}

	if _, _, err := s.store.UserByEmail(ctx, email); err == nil {
		return model.AuthResponse{}, ErrEmailTaken
	} else if !errors.Is(err, store.ErrNotFound) {
		return model.AuthResponse{}, err
	}

	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		return model.AuthResponse{}, err
	}

	now := time.Now().UTC()
	user := model.User{ID: id.New("user"), Email: email, Name: name, CreatedAt: now}
	workspace := model.Workspace{ID: id.New("ws"), Name: workspaceName, CreatedAt: now}

	if err := s.store.CreateUser(ctx, user, hash); err != nil {
		return model.AuthResponse{}, err
	}
	if err := s.store.CreateWorkspace(ctx, workspace); err != nil {
		return model.AuthResponse{}, err
	}
	if err := s.store.CreateMembership(ctx, workspace.ID, user.ID, "owner", "online"); err != nil {
		return model.AuthResponse{}, err
	}

	graph := flow.Starter(id.New("flow"), "Atendimento inicial")
	if err := s.store.CreateFlow(ctx, workspace.ID, graph); err != nil {
		return model.AuthResponse{}, err
	}

	channel := model.Channel{
		ID:             id.New("ch"),
		WorkspaceID:    workspace.ID,
		Key:            id.New("pk"),
		Name:           "Site principal",
		FlowID:         graph.ID,
		Appearance:     model.DefaultAppearance,
		AllowedOrigins: []string{},
		CreatedAt:      now,
	}
	if err := s.store.CreateChannel(ctx, channel); err != nil {
		return model.AuthResponse{}, err
	}

	token, err := s.tokens.Issue(user.ID, workspace.ID, "owner")
	if err != nil {
		return model.AuthResponse{}, err
	}

	return model.AuthResponse{
		Token: token,
		AuthResult: model.AuthResult{
			User:       user,
			Workspace:  workspace,
			Membership: model.Membership{Role: "owner", Availability: "online"},
		},
	}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (model.AuthResponse, error) {
	email = normalizeEmail(email)

	user, hash, err := s.store.UserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return model.AuthResponse{}, ErrInvalidCredentials
		}
		return model.AuthResponse{}, err
	}

	if !auth.CheckPassword(hash, password) {
		return model.AuthResponse{}, ErrInvalidCredentials
	}

	workspace, membership, err := s.store.WorkspaceForUser(ctx, user.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return model.AuthResponse{}, ErrInvalidCredentials
		}
		return model.AuthResponse{}, err
	}

	token, err := s.tokens.Issue(user.ID, workspace.ID, membership.Role)
	if err != nil {
		return model.AuthResponse{}, err
	}

	return model.AuthResponse{
		Token: token,
		AuthResult: model.AuthResult{
			User:       user,
			Workspace:  workspace,
			Membership: membership,
		},
	}, nil
}

func (s *Service) Me(ctx context.Context, userID string) (model.AuthResult, error) {
	user, err := s.store.UserByID(ctx, userID)
	if err != nil {
		return model.AuthResult{}, err
	}

	workspace, membership, err := s.store.WorkspaceForUser(ctx, userID)
	if err != nil {
		return model.AuthResult{}, err
	}

	return model.AuthResult{User: user, Workspace: workspace, Membership: membership}, nil
}

func (s *Service) AddAgent(ctx context.Context, workspaceID, name, email, password string) (model.Agent, error) {
	name = strings.TrimSpace(name)
	email = normalizeEmail(email)

	if len(name) < 2 || email == "" || len(password) < 8 {
		return model.Agent{}, ErrInvalidInput
	}

	if _, _, err := s.store.UserByEmail(ctx, email); err == nil {
		return model.Agent{}, ErrEmailTaken
	} else if !errors.Is(err, store.ErrNotFound) {
		return model.Agent{}, err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return model.Agent{}, err
	}

	now := time.Now().UTC()
	user := model.User{ID: id.New("user"), Email: email, Name: name, CreatedAt: now}

	if err := s.store.CreateUser(ctx, user, hash); err != nil {
		return model.Agent{}, err
	}
	if err := s.store.CreateMembership(ctx, workspaceID, user.ID, "agent", "offline"); err != nil {
		return model.Agent{}, err
	}

	return model.Agent{
		UserID:       user.ID,
		Name:         user.Name,
		Email:        user.Email,
		Role:         "agent",
		Availability: "offline",
	}, nil
}

func (s *Service) Agents(ctx context.Context, workspaceID string) ([]model.Agent, error) {
	return s.store.Members(ctx, workspaceID)
}

func (s *Service) SetAvailability(ctx context.Context, workspaceID, userID, availability string) error {
	switch availability {
	case "online", "busy", "offline":
	default:
		return ErrInvalidAvailability
	}

	if err := s.store.UpdateAvailability(ctx, workspaceID, userID, availability); err != nil {
		return err
	}
	return nil
}

func normalizeEmail(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return ""
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return ""
	}
	return email
}
