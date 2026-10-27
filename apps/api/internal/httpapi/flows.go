package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"chatfree/api/internal/flow"
	"chatfree/api/internal/id"
	"chatfree/api/internal/model"
	"chatfree/api/internal/store"
)

func (a *API) listFlows(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	graphs, err := a.store.Flows(r.Context(), claims.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	summaries := make([]model.FlowSummary, 0, len(graphs))
	for _, graph := range graphs {
		summaries = append(summaries, model.FlowSummary{
			ID:        graph.ID,
			Name:      graph.Name,
			Version:   graph.Version,
			Nodes:     len(graph.Nodes),
			UpdatedAt: graph.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, summaries)
}

func (a *API) createFlow(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)
	if !canManage(claims) {
		writeError(w, http.StatusForbidden, "somente administradores podem criar fluxos")
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "nome do fluxo é obrigatório")
		return
	}

	graph := flow.Starter(id.New("flow"), name)
	if err := a.store.CreateFlow(r.Context(), claims.WorkspaceID, graph); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, graph)
}

func (a *API) getFlow(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	graph, err := a.store.Flow(r.Context(), claims.WorkspaceID, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "fluxo não encontrado")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, graph)
}

func (a *API) saveFlow(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)
	if !canManage(claims) {
		writeError(w, http.StatusForbidden, "somente administradores podem editar fluxos")
		return
	}

	var graph flow.Graph
	if err := readJSON(r, &graph); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	id := r.PathValue("id")
	if graph.ID == "" {
		graph.ID = id
	}
	if graph.ID != id {
		writeError(w, http.StatusBadRequest, "id do fluxo não corresponde à rota")
		return
	}

	existing, err := a.store.Flow(r.Context(), claims.WorkspaceID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "fluxo não encontrado")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := flow.Validate(graph); err != nil {
		writeError(w, http.StatusBadRequest, "fluxo inválido: "+err.Error())
		return
	}

	graph.Version = existing.Version + 1
	if strings.TrimSpace(graph.Name) == "" {
		graph.Name = existing.Name
	}

	if err := a.store.SaveFlow(r.Context(), claims.WorkspaceID, graph); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, graph)
}

func (a *API) deleteFlow(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)
	if !canManage(claims) {
		writeError(w, http.StatusForbidden, "somente administradores podem remover fluxos")
		return
	}

	channels, err := a.store.Channels(r.Context(), claims.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, channel := range channels {
		if channel.FlowID == r.PathValue("id") {
			writeError(w, http.StatusConflict, "o fluxo está em uso por um canal")
			return
		}
	}

	if err := a.store.DeleteFlow(r.Context(), claims.WorkspaceID, r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "fluxo não encontrado")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
