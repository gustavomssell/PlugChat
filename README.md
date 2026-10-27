# ChatFree

Plataforma de atendimento conversacional multi-tenant em que o bot faz a triagem no widget do site e transfere a conversa para um atendente humano quando necessário. O repositório reúne uma API em Go com PostgreSQL, um dashboard em React e um widget embutível em Preact isolado por Shadow DOM.

<img src="docs/screenshots/dashboard.png" width="900" alt="Visão geral do dashboard do ChatFree com métricas de atendimento" />

## Funcionalidades

- Cadastro e login com JWT, workspace multi-tenant e papéis `owner`, `admin` e `agent`.
- Disponibilidade do atendente (`online`, `busy`, `offline`) e presença no painel atualizada em tempo real.
- Inbox em tempo real por SSE, com busca, filtros por status e canal, atribuição de atendente, tags e encerramento.
- Notas internas nas conversas: comentários com autor e horário que a equipe vê em faixa própria e o visitante nunca recebe.
- Respostas prontas e macros por workspace, com atalho (`/horario`), tags opcionais e inserção com um clique no compositor.
- Ações em massa na inbox: seleção múltipla para atribuir a um atendente, aplicar tag ou encerrar até 100 conversas de uma vez.
- Relatórios com volume, tempo médio de primeira resposta, carga por canal e por atendente, além de exportação CSV da inbox filtrada.
- Anexos e áudio: imagens, arquivos e mensagens de voz gravadas no microfone, com upload pelo painel ou pelo widget e limite de 8 MB por arquivo.
- Fluxos de automação com editor visual (React Flow), versionamento, validação do grafo e snapshot por conversa.
- Nós de fluxo com execução no servidor: início, mensagem, coleta de resposta (texto, escolha com atalho ou formulário multi-campo em uma tela), condição, tag, atribuição de atendente, webhook, atraso, IA opcional e transferência para humano.
- Velocidade percebida no chat: opções com atalho pulam passos intermediários, formulário coleta vários campos em uma interação, indicador "digitando…" durante atrasos e chamadas de IA, e trava anti-loop por conversa.
- Canais por site, com chave pública, escolha de fluxo, aparência, origens permitidas e snippet de incorporação.
- Widget em Shadow DOM com modos bolha, popup, tela cheia e inline; temas claro/escuro/automático, posição configurável, respostas rápidas, retomada da conversa salva no navegador e reinício após o encerramento.
- Seed automático de demonstração na primeira subida do banco.
- Rate limit nas rotas públicas do widget, validação rígida de origem, limites de corpo e de tamanho de mensagem, cabeçalhos de segurança e timeouts no servidor.

<div>
  <img src="docs/screenshots/inbox.png" width="49%" alt="Inbox do ChatFree com conversas e painel de mensagens" />
  <img src="docs/screenshots/flow-editor.png" width="49%" alt="Editor visual de fluxos do ChatFree" />
</div>

## Começando

Pré-requisitos:

- Node.js 20.19 ou superior
- Go 1.26 ou superior
- Docker com Docker Compose

```powershell
npm install
npm run db:up
npm run dev:api
```

Em outro terminal:

```powershell
npm run dev:dashboard
```

Dashboard em `http://localhost:5173`, API em `http://localhost:8090`. O playground do widget roda com:

```powershell
npm run dev:widget
```

Acesse `http://localhost:5174` para conversar com o widget usando o canal `pk_demo`.

### Credenciais de demonstração

| Papel | E-mail | Senha |
| --- | --- | --- |
| Proprietário | `demo@chatfree.dev` | `demo1234` |
| Atendente | `ana@chatfree.dev` | `demo1234` |

O seed só é criado quando o banco não tem nenhum usuário.

### Scripts

| Comando | O que faz |
| --- | --- |
| `npm run db:up` | Sobe o PostgreSQL do `docker-compose.yml` na porta `5437`. |
| `npm run db:down` | Derruba os containers do Compose. |
| `npm run dev:api` | Roda a API Go (porta padrão `8090`). |
| `npm run dev:dashboard` | Roda o dashboard em modo desenvolvimento (porta `5173`). |
| `npm run dev:widget` | Roda o playground do widget (porta `5174`). |
| `npm run typecheck` | Executa o TypeScript de todos os workspaces. |
| `npm run build` | Gera os builds do widget e do dashboard. |
| `npm run build:api` | Compila a API em `apps/api/bin/api.exe` no Windows. |
| `npm run test:api` | Roda os testes Go (a integração é pulada sem `TEST_DATABASE_URL`). |

### Variáveis de ambiente

A API lê variáveis do processo. Os valores padrão já funcionam com o Compose local; use `apps/api/.env.example` como referência e exporte as variáveis no shell quando quiser sobrescrever.

| Variável | Padrão | Usada por |
| --- | --- | --- |
| `PORT` | `8090` | API |
| `DATABASE_URL` | `postgres://chatfree:chatfree@localhost:5437/chatfree?sslmode=disable` | API |
| `JWT_SECRET` | `chatfree-dev-secret` | API |
| `LLM_API_URL` | vazio (nó de IA usa a resposta de reserva) | API |
| `LLM_API_KEY` | vazio | API |
| `LLM_MODEL` | `gpt-4o-mini` | API |
| `ATTACHMENTS_DIR` | `data/uploads` | API |
| `VITE_API_URL` | vazio no dashboard (usa o proxy do Vite); `http://localhost:8090` no playground do widget | Dashboard e widget |

`LLM_API_URL` deve apontar para a base de uma API compatível com OpenAI, incluindo a versão, por exemplo `https://api.openai.com/v1`. A API monta `POST {LLM_API_URL}/chat/completions`.

Exemplo no PowerShell:

```powershell
$env:JWT_SECRET="uma-chave-local"
npm run dev:api
```

## Arquitetura

```mermaid
flowchart LR
  Site[Site do cliente] --> Widget[Widget Preact<br/>Shadow DOM]
  Painel[Dashboard React] -->|JWT e SSE| API[API Go]
  Widget -->|chave pública e SSE| API
  API --> Runtime[Runtime de fluxos]
  API --> PG[(PostgreSQL)]
```

- O widget usa apenas a chave pública do canal e cria uma conversa anônima.
- O dashboard autentica com JWT e escopa todas as consultas pelo workspace do usuário.
- O runtime de fluxos é executado no servidor; cada conversa guarda uma cópia do grafo publicado, então editar um fluxo não altera conversas em andamento.
- SSE distribui eventos para o inbox e para a conversa aberta no widget.

## Como funciona

1. O visitante abre o widget e a API cria a conversa com o snapshot do fluxo do canal.
2. O runtime avança pelos nós até precisar de uma resposta, transferir para humano ou encerrar. Tags e atribuições são aplicadas na conversa, webhooks recebem um POST com o contexto, atrasos são retomados pelo worker do servidor e o nó de IA consulta o provedor configurado.
3. Cada resposta do visitante retoma o fluxo a partir do nó de coleta atual.
4. Na transferência, a conversa entra na fila (`queued`); um atendente assume e passa a responder pelo dashboard.
5. Mensagens e mudanças de status chegam em tempo real por SSE nos dois lados.

## Protocolo HTTP

Rotas do dashboard exigem `Authorization: Bearer <token>`.

| Método | Rota | Uso |
| --- | --- | --- |
| `POST` | `/api/auth/register` | Cria usuário, workspace e membership de owner. |
| `POST` | `/api/auth/login` | Autentica e devolve token JWT. |
| `GET` | `/api/auth/me` | Devolve usuário, workspace, papel e disponibilidade. |
| `PATCH` | `/api/auth/me/availability` | Atualiza a disponibilidade do atendente. |
| `GET` | `/api/stats` | Métricas da visão geral. |
| `GET` | `/api/metrics?days=7` | Relatório do período (7, 30 ou 90 dias). |
| `GET/POST` | `/api/replies` | Lista e cria respostas prontas (escrita exige `owner`/`admin`). |
| `PUT/DELETE` | `/api/replies/{id}` | Edita e remove uma resposta pronta. |
| `GET/POST` | `/api/agents` | Lista e cria atendentes. |
| `GET/POST` | `/api/channels` | Lista e cria canais. |
| `GET/PUT/DELETE` | `/api/channels/{id}` | Detalha, edita e remove um canal. |
| `GET/POST` | `/api/flows` | Lista e cria fluxos. |
| `GET/PUT/DELETE` | `/api/flows/{id}` | Lê, salva e remove um fluxo. |
| `GET` | `/api/conversations` | Lista conversas com filtros de status, canal e busca. |
| `GET` | `/api/conversations/export` | CSV das conversas filtradas (mesmos filtros da inbox). |
| `POST` | `/api/conversations/bulk` | Aplica `assign`, `tag` ou `status` em até 100 conversas. |
| `GET/PATCH` | `/api/conversations/{id}` | Detalha e altera status, responsável e tags. |
| `GET/POST` | `/api/conversations/{id}/messages` | Lista mensagens e responde como atendente (`attachmentIds` opcional). |
| `POST` | `/api/conversations/{id}/notes` | Cria nota interna, invisível para o visitante. |
| `POST` | `/api/conversations/{id}/attachments` | Upload multipart de um anexo (até 8 MB). |
| `GET` | `/api/attachments/{id}` | Baixa um anexo do workspace (autenticado). |
| `GET` | `/api/events` | SSE do inbox do workspace. |
| `GET` | `/api/conversations/{id}/events` | SSE de uma conversa. |

Rotas públicas do widget usam a chave do canal e validam a origem quando ela está configurada. Elas também são limitadas por taxa e têm tamanho de corpo e de mensagem limitados; os padrões e as variáveis de ambiente estão em [Proteções do widget público](#proteções-do-widget-público).

| Método | Rota | Uso |
| --- | --- | --- |
| `GET` | `/api/widget/config/{key}` | Configuração pública do widget. |
| `POST` | `/api/widget/start/{key}` | Inicia uma conversa. |
| `GET` | `/api/widget/conversations/{id}` | Estado atual da conversa. |
| `GET/POST` | `/api/widget/conversations/{id}/messages` | Lista e envia mensagens do visitante. |
| `POST` | `/api/widget/conversations/{id}/attachments` | Upload multipart de imagem, áudio ou arquivo. |
| `GET` | `/api/widget/attachments/{id}?conversation={id}` | Baixa um anexo da conversa. |
| `GET` | `/api/widget/conversations/{id}/events` | SSE da conversa no widget. |

### Eventos SSE

| Rota | Evento | Conteúdo |
| --- | --- | --- |
| `/api/events` | `conversation` | Conversa criada ou atualizada (status, tags, responsável). |
| `/api/events` | `agent` | Atendente adicionado ou disponibilidade alterada. |
| `/api/events` | `note` | Nota interna criada em uma conversa do workspace. |
| `/api/conversations/{id}/events` | `chat` | Nova mensagem da conversa. |
| `/api/conversations/{id}/events` | `conversation` | Mudança de status ou metadados da conversa. |
| `/api/widget/conversations/{id}/events` | `chat` | Mensagens do bot, atendente ou sistema. |
| `/api/widget/conversations/{id}/events` | `conversation` | Mudança de status, opções de resposta ou encerramento. |

## Estrutura do projeto

```text
apps/
  api/                  API Go, migrações SQL, fluxo, store e testes
  dashboard/            Painel React com TanStack Router/Query e React Flow
  widget/               Widget Preact em Shadow DOM e playground de desenvolvimento
packages/
  shared/               Tipos, schemas Zod e contratos compartilhados
docs/screenshots/       Imagens usadas neste README
docker-compose.yml      PostgreSQL 16 local
```

## Testes

Os testes unitários do auth, do runtime de fluxos e da ordenação de mensagens rodam sem banco:

```powershell
npm run test:api
```

O teste de integração percorre cadastro, canal, fluxo, conversa, handoff e mensagens. Ele exige um PostgreSQL acessível e limpa os dados que cria:

```powershell
$env:TEST_DATABASE_URL="postgres://chatfree:chatfree@localhost:5437/chatfree?sslmode=disable"
npm run test:api
```

Verificação de tipos e build:

```powershell
npm run typecheck
npm run build
```

## Limitações conhecidas

- Projeto em estágio de MVP, sem CI, licença definida ou deploy automatizado.
- Login apenas por e-mail e senha; não há convite por e-mail, recuperação de senha ou SSO.
- O rate limit do widget é em memória por processo (token bucket). Com várias instâncias cada uma aplica seus próprios limites e não há estado compartilhado; CAPTCHA e reputação de IP seguem fora do escopo.
- Com `TRUST_PROXY=false` (padrão) o IP do cliente vem de `RemoteAddr`; atrás de proxy sem essa variável, todos os visitantes parecem o mesmo IP.
- O nó de IA depende de `LLM_API_URL`; sem essa variável (ou em caso de falha do provedor), o nó usa a resposta de reserva configurada no editor.
- Webhooks enviam um POST com timeout curto, sem assinatura, retentativa ou fila de entrega.
- A retomada de atrasos é feita por polling no banco dentro da API. Com mais de uma instância, duas podem tentar retomar a mesma conversa.
- O hub de SSE é em memória, adequado a uma única instância da API.
- Sem notificações externas, exclusão de conversas em massa ou convite por e-mail.
- Anexos ficam no disco local (`ATTACHMENTS_DIR`): o banco guarda só os metadados. Sem storage externo, replicação ou varredura de órfãos; o nome no disco é o id com a extensão, e o original fica registrado no banco.
- Relatórios usam médias simples de primeira resposta por período e não filtram por fuso do visitante; o processamento é em SQL direto, sem materialização.
- O snippet gerado aponta para `https://cdn.chatfree.app/w.js` como placeholder; o build local do widget fica em `apps/widget/dist/w.js`.

### Proteções do widget público

As rotas `/api/widget/*` são abertas por chave pública, então carregam limites próprios:

| Proteção | Onde | Padrão | Variável |
| --- | --- | --- | --- |
| Consulta de configuração por IP e canal | `GET /api/widget/config/{key}` | 120/min | `WIDGET_CONFIG_PER_MINUTE` |
| Início de conversa por IP e canal | `POST /api/widget/start/{key}` | 10 / 10 min | `WIDGET_STARTS_PER_10M` |
| Início de conversa por canal (válvula global) | `POST /api/widget/start/{key}` | 120/h | `WIDGET_STARTS_PER_HOUR` |
| Leituras por conversa e IP | `GET /api/widget/conversations/{id}` | 120/min | `WIDGET_READS_PER_MINUTE` |
| Mensagens por IP e conversa | `POST .../messages` | 20/min | `WIDGET_MESSAGES_PER_MINUTE` |
| Mensagens por conversa (válvula global) | `POST .../messages` | 60/min | `WIDGET_MESSAGES_PER_MINUTE_CONVERSATION` |
| Anexos por IP e conversa | `POST .../attachments` | mesmos limites de mensagens | — |
| Confiança em `X-Forwarded-For`/`X-Real-IP` | todas as acima | desligado | `TRUST_PROXY` |

Quando um limite estoura a API responde `429` com `Retry-After` em segundos. O corpo JSON das rotas do widget é limitado a 16 KB (`413` acima disso), mensagens de visitante a 4000 caracteres e o `visitanteId` a 128 caracteres sem caracteres de controle. Respostas do widget trazem `Cache-Control: no-store`, `X-Content-Type-Options: nosniff` e `Vary: Origin`.

### Respostas mais rápidas no fluxo

Três recursos do runtime reduzem o número de interações até o visitante chegar a um humano:

1. **Opção com atalho** — no nó "Coletar resposta", cada opção pode apontar direto para outro nó (`nextNodeId`). Clicar em "Suporte" vai reto para a fila, sem passar por condição nem mensagens intermediárias.
2. **Formulário em uma tela** (`fields`) — o mesmo nó pede vários campos de uma vez; o visitante responde com um único envio. Campos obrigatórios vazios voltam `400` informando qual campo, e apenas os nomes declarados no nó são aceitos.
3. **Fallback e limite de passos** — `fallbackNodeId` recebe respostas fora das opções (reenvia a pergunta ou escala), e cada conversa tem um teto de passos que interrompe grafos com laço infinito.

O atraso e a chamada de IA publicam um evento `typing` no SSE da conversa, então o widget mostra "digitando…" sem gravar mensagem. O widget também mostra o indicador de forma otimista enquanto a requisição está no ar.

Origens permitidas do canal são normalizadas ao salvar (`exemplo.com`, `exemplo.com/` e `https://exemplo.com:443` viram `https://exemplo.com`) e validadas: caminho, query, fragmento, usuário/senha e curinga de host são rejeitados com `400`. Com allowlist configurada, uma requisição sem cabeçalho `Origin` é recusada com `403` — allowlist vazia continua aceitando qualquer origem.

### Colaboração da equipe

Cinco recursos do painel trabalham junto no dia a dia do atendimento:

1. **Notas internas** — o compositor tem um botão de nota que grava um comentário com autor e horário. Notas aparecem em faixa própria no histórico, circulam apenas pelo evento `note` do inbox e nunca são devolvidas pelas rotas do widget.
2. **Respostas prontas** — biblioteca por workspace com título, corpo, atalho (`/horario`) e tags opcionais. Um clique insere o texto no compositor; outro aplica as tags na conversa. A escrita exige `owner` ou `admin`; o limite é de 200 respostas com atalhos únicos.
3. **Ações em massa** — a lista da inbox tem seleção múltipla: atribuir a um atendente, aplicar uma tag ou encerrar até 100 conversas por requisição. Cada conversa passa pela mesma validação do `PATCH` individual e publica os mesmos eventos SSE.
4. **Relatórios e exportação** — a página de relatórios mostra volume, encerradas, tempo médio de primeira resposta (do início da conversa até a primeira mensagem do bot ou do atendente), mensagens e a carga por canal e por atendente em 7, 30 ou 90 dias. A inbox exporta o filtro atual como CSV.
5. **Anexos e áudio** — imagens (PNG, JPEG, GIF, WebP), áudios (MP3, WAV, OGG, WebM, M4A) e arquivos (PDF, TXT, CSV, JSON, ZIP) de até 8 MB, enviados pelo painel ou pelo widget. O upload cria um anexo pendente que a próxima mensagem reclama por `attachmentIds`; o armazenamento é local em `ATTACHMENTS_DIR`.

## Widget

<img src="docs/screenshots/widget.png" width="520" alt="Widget do ChatFree com respostas rápidas no playground" />

O build do widget gera um único `w.js` com CSS embutido. Para testar em outro site, sirva `apps/widget/dist/` em qualquer host estático e referencie o arquivo:

```html
<script
  async
  src="/w.js"
  data-key="pk_demo"
  data-api="http://localhost:8090"
  data-mode="bubble"
  data-position="bottom-right"
></script>
```

Em produção, use `data-api` com a URL pública da API.
