package httpapi

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"chatfree/api/internal/chat"
	"chatfree/api/internal/files"
	"chatfree/api/internal/model"
	"chatfree/api/internal/store"
)

// maxUploadBytes caps a single attachment upload (content plus multipart
// overhead). The chat service enforces the same ceiling on the metadata.
const maxUploadBytes = chat.MaxAttachmentBytes + (64 << 10)

// uploadKinds maps a safe file extension to the kind and the MIME type the
// browser is told to use. Anything not listed here is refused with 415.
var uploadKinds = map[string]struct {
	kind string
	mime string
}{
	".png":  {model.KindImage, "image/png"},
	".jpg":  {model.KindImage, "image/jpeg"},
	".jpeg": {model.KindImage, "image/jpeg"},
	".gif":  {model.KindImage, "image/gif"},
	".webp": {model.KindImage, "image/webp"},
	".mp3":  {model.KindAudio, "audio/mpeg"},
	".wav":  {model.KindAudio, "audio/wav"},
	".ogg":  {model.KindAudio, "audio/ogg"},
	".webm": {model.KindAudio, "audio/webm"},
	".m4a":  {model.KindAudio, "audio/mp4"},
	".pdf":  {model.KindFile, "application/pdf"},
	".txt":  {model.KindFile, "text/plain"},
	".csv":  {model.KindFile, "text/csv"},
	".json": {model.KindFile, "application/json"},
	".zip":  {model.KindFile, "application/zip"},
}

func kindForExtension(extension string) (string, string, bool) {
	entry, ok := uploadKinds[strings.ToLower(extension)]
	if !ok {
		return "", "", false
	}
	return entry.kind, entry.mime, true
}

// uploadAttachment accepts one multipart file for an agent conversation.
func (a *API) uploadAttachment(w http.ResponseWriter, r *http.Request) {
	if a.files == nil {
		writeError(w, http.StatusServiceUnavailable, "envio de anexos não configurado nesta instância")
		return
	}

	claims := claimsOf(r)
	attachment, ok := a.receiveAttachment(w, r, claims.WorkspaceID, r.PathValue("id"))
	if !ok {
		return
	}

	writeJSON(w, http.StatusCreated, attachment)
}

// widgetUploadAttachment is the public counterpart: the visitor uploads to a
// conversation it already started, guarded by the widget rate limits and the
// channel origin allowlist.
func (a *API) widgetUploadAttachment(w http.ResponseWriter, r *http.Request) {
	if a.files == nil {
		writeError(w, http.StatusServiceUnavailable, "envio de anexos não configurado nesta instância")
		return
	}

	if !a.widget.allowMessage(w, r, r.PathValue("id")) {
		return
	}

	conversation, _, ok := a.widgetContext(w, r)
	if !ok {
		return
	}

	attachment, ok := a.receiveAttachment(w, r, conversation.WorkspaceID, conversation.ID)
	if !ok {
		return
	}

	writeJSON(w, http.StatusCreated, attachment)
}

// receiveAttachment validates, persists and registers one multipart file.
func (a *API) receiveAttachment(w http.ResponseWriter, r *http.Request, workspaceID, conversationID string) (model.Attachment, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

	file, header, err := r.FormFile("file")
	if err != nil {
		writeUploadError(w, err)
		return model.Attachment{}, false
	}
	defer file.Close()

	extension := files.ExtensionOf(header.Filename)
	kind, mime, ok := kindForExtension(extension)
	if !ok {
		writeError(w, http.StatusUnsupportedMediaType, "tipo de arquivo não permitido")
		return model.Attachment{}, false
	}
	if header.Size <= 0 || header.Size > chat.MaxAttachmentBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "anexo maior que o limite de 8 MB")
		return model.Attachment{}, false
	}

	attachment, err := a.chat.AddAttachment(r.Context(), workspaceID, conversationID, chat.AttachmentInput{
		Kind:     kind,
		Filename: header.Filename,
		Mime:     mime,
		Size:     header.Size,
	})
	if err != nil {
		a.writeChatError(w, err)
		return model.Attachment{}, false
	}

	if _, err := a.files.Save(attachment.ID, extension, file); err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível armazenar o anexo")
		return model.Attachment{}, false
	}

	return attachment, true
}

func writeUploadError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "anexo maior que o limite de 8 MB")
		return
	}
	writeError(w, http.StatusBadRequest, "envie um arquivo no campo \"file\"")
}

func (a *API) downloadAttachment(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	attachment, err := a.store.Attachment(r.Context(), claims.WorkspaceID, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "anexo não encontrado")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.serveAttachment(w, r, attachment)
}

// widgetAttachment serves a file to the visitor. Knowing the conversation id
// is the capability here, exactly like the other public widget routes.
func (a *API) widgetAttachment(w http.ResponseWriter, r *http.Request) {
	conversationID := strings.TrimSpace(r.URL.Query().Get("conversation"))
	if conversationID == "" {
		writeError(w, http.StatusBadRequest, "conversa não informada")
		return
	}

	if !a.widget.allowRead(w, r, conversationID) {
		return
	}

	attachments, err := a.store.AttachmentsByConversation(r.Context(), conversationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	requested := r.PathValue("id")
	for _, attachment := range attachments {
		if attachment.ID == requested {
			a.serveAttachment(w, r, attachment)
			return
		}
	}

	writeError(w, http.StatusNotFound, "anexo não encontrado")
}

func (a *API) serveAttachment(w http.ResponseWriter, r *http.Request, attachment model.Attachment) {
	if a.files == nil {
		writeError(w, http.StatusServiceUnavailable, "anexos não configurados nesta instância")
		return
	}

	extension := strings.TrimPrefix(attachment.StoredName, attachment.ID)
	file, err := a.files.Open(attachment.ID, extension)
	if err != nil {
		if errors.Is(err, files.ErrInvalidName) {
			writeError(w, http.StatusNotFound, "anexo não encontrado")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", attachment.Mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if attachment.Kind == model.KindFile {
		w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(attachment.Filename)+`"`)
	}

	http.ServeContent(w, r, attachment.Filename, attachment.CreatedAt, file)
}
