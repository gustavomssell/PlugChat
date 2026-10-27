package bootstrap

import (
	"context"
	"log"
	"time"

	"chatfree/api/internal/auth"
	"chatfree/api/internal/flow"
	"chatfree/api/internal/id"
	"chatfree/api/internal/model"
	"chatfree/api/internal/store"
)

const DemoPassword = "demo1234"

func Run(ctx context.Context, st store.Store) error {
	count, err := st.CountUsers(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := auth.HashPassword(DemoPassword)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	demoUser := model.User{ID: "user_demo", Email: "demo@chatfree.dev", Name: "Demo", CreatedAt: now}
	workspace := model.Workspace{ID: "ws_demo", Name: "ChatFree Demo", CreatedAt: now}

	if err := st.CreateUser(ctx, demoUser, hash); err != nil {
		return err
	}
	if err := st.CreateWorkspace(ctx, workspace); err != nil {
		return err
	}
	if err := st.CreateMembership(ctx, workspace.ID, demoUser.ID, "owner", "online"); err != nil {
		return err
	}

	if err := st.CreateFlow(ctx, workspace.ID, flow.Sample()); err != nil {
		return err
	}

	channel := model.Channel{
		ID:             "ch_demo",
		WorkspaceID:    workspace.ID,
		Key:            "pk_demo",
		Name:           "Site principal",
		FlowID:         "flow_support",
		Appearance:     model.DefaultAppearance,
		AllowedOrigins: []string{},
		CreatedAt:      now,
	}
	if err := st.CreateChannel(ctx, channel); err != nil {
		return err
	}

	// Respostas prontas de exemplo: o compositor da inbox já nasce utilizável.
	samples := []model.SavedReply{
		{
			ID:          "reply_demo_hours",
			WorkspaceID: workspace.ID,
			Title:       "Horário de atendimento",
			Body:        "Atendemos de segunda a sexta, das 9h às 18h (horário de Brasília).",
			Shortcut:    "horario",
			Tags:        []string{"horario"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "reply_demo_close",
			WorkspaceID: workspace.ID,
			Title:       "Encerramento com pesquisa",
			Body:        "Posso ajudar em algo mais antes de encerrarmos? Se preferir, abra uma nova conversa a qualquer momento.",
			Shortcut:    "encerrar",
			Tags:        []string{"pesquisa"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	for _, reply := range samples {
		if err := st.CreateSavedReply(ctx, reply); err != nil {
			return err
		}
	}

	agentHash, err := auth.HashPassword(DemoPassword)
	if err != nil {
		return err
	}

	ana := model.User{ID: id.New("user"), Email: "ana@chatfree.dev", Name: "Ana Souza", CreatedAt: now}
	if err := st.CreateUser(ctx, ana, agentHash); err != nil {
		return err
	}
	if err := st.CreateMembership(ctx, workspace.ID, ana.ID, "agent", "online"); err != nil {
		return err
	}

	log.Printf("bootstrap: demo data created (demo@chatfree.dev / %s, ana@chatfree.dev / %s, channel pk_demo)", DemoPassword, DemoPassword)
	return nil
}
