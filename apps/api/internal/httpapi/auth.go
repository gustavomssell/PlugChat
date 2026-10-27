package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"chatfree/api/internal/accounts"
	"chatfree/api/internal/chat"
)

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name          string `json:"name"`
		Email         string `json:"email"`
		Password      string `json:"password"`
		WorkspaceName string `json:"workspaceName"`
	}

	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	result, err := a.accounts.Register(r.Context(), accounts.RegisterInput{
		Name:          body.Name,
		Email:         body.Email,
		Password:      body.Password,
		WorkspaceName: body.WorkspaceName,
	})
	if err != nil {
		switch {
		case errors.Is(err, accounts.ErrEmailTaken):
			writeError(w, http.StatusConflict, "e-mail já cadastrado")
		case errors.Is(err, accounts.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "nome, e-mail e senha (mínimo 8 caracteres) são obrigatórios")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	result, err := a.accounts.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		if errors.Is(err, accounts.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "e-mail ou senha inválidos")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	result, err := a.accounts.Me(r.Context(), claims.Subject)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "sessão inválida")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *API) setAvailability(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	var body struct {
		Availability string `json:"availability"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	availability := strings.TrimSpace(body.Availability)
	if err := a.accounts.SetAvailability(r.Context(), claims.WorkspaceID, claims.Subject, availability); err != nil {
		if errors.Is(err, accounts.ErrInvalidAvailability) {
			writeError(w, http.StatusBadRequest, "disponibilidade inválida")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.hub.Publish(chat.InboxTopic(claims.WorkspaceID), "agent", map[string]string{
		"userId":       claims.Subject,
		"availability": availability,
	})

	writeJSON(w, http.StatusOK, map[string]string{"availability": availability})
}

func (a *API) listAgents(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	agents, err := a.accounts.Agents(r.Context(), claims.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, agents)
}

func (a *API) createAgent(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)
	if !canManage(claims) {
		writeError(w, http.StatusForbidden, "somente administradores podem adicionar agentes")
		return
	}

	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	agent, err := a.accounts.AddAgent(r.Context(), claims.WorkspaceID, body.Name, body.Email, body.Password)
	if err != nil {
		switch {
		case errors.Is(err, accounts.ErrEmailTaken):
			writeError(w, http.StatusConflict, "e-mail já cadastrado")
		case errors.Is(err, accounts.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "nome, e-mail e senha (mínimo 8 caracteres) são obrigatórios")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	a.hub.Publish(chat.InboxTopic(claims.WorkspaceID), "agent", agent)

	writeJSON(w, http.StatusCreated, agent)
}

func (a *API) stats(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	stats, err := a.chat.Stats(r.Context(), claims.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stats)
}
