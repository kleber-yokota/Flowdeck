# AGENTS.md — FlowDeck

Orientações para agentes de código trabalhando neste repositório.

## O projeto

FlowDeck é um painel de orquestração de workflows duráveis. **Um único serviço em Go**: a camada web (HTMX, `:8080`) faz o boot do core DBOS e expõe workflows duráveis no Postgres 18 (`:5432`). Todo o design (arquitetura, estados, sequências) está documentado em `diagrams/` — leia o `diagrams/README.md` e os `.html` antes de implementar.

## Stack

| Camada | Tecnologia | Observações |
| --- | --- | --- |
| Backend + Web | **Go** (serviço único) | webserver em `:8080`, serve templates com HTMX |
| Frontend | **HTMX** (no navegador, nada de SPA/JS framework) | páginas são HTML server-side + `hx-*`; interações são GET/POST HTMX |
| Orquestração | **DBOS Go** — `github.com/dbos-inc/dbos-transact-golang` | workflows, filas e estado durável no Postgres; API: `dbos.NewContext(ctx, dbos.Config{...})`, `RegisterQueue`, `Enqueue`/`WithQueue` |
| Banco | **Postgres 18** (`:5432`) | via `DBOS_SYSTEM_DATABASE_URL`; DBOS exige um sistema database dedicado |
| Deploy | **docker-compose**, uma máquina | serviço FlowDeck + Postgres 18; não há nenhum outro container |

## Invariantes de arquitetura (não violar)

1. **Um processo, dois mundos**: a web é uma *abstração sobre o boot do core DBOS* — o core é iniciado dentro do serviço Go e a web faz chamadas nele. O DBOS **não sobe webserver nenhum**; se você vir o DBOS expondo portas/rotas, está errado.
2. **Zero hardcode de workflow na web**: a lista de workflows vem do **registry do core**. A web lista o que o core registrou; adicionar um novo workflow = registrá-lo no core, nunca adicionar item fixo no template/HTML.
3. **Fila no DBOS**: disparo = enqueue (`POST` HTMX → web → core → `Enqueue` no Postgres). O executor do DBOS seleciona o job da fila, executa e atualiza o status — o webserver nunca executa o workflow diretamente.
4. **Status por polling**: o front consulta o status a cada **2s** (polling HTMX, `hx-trigger="every 2s"` ou similar); não usar websocket/SSE para o status.

## Máquina de estados do workflow

```
na fila → em execução (retry ≤ 3) → sucesso | falha
```

- Estados DBOS mapeados: `enqueued` → na fila, `running` → em execução, `success` → sucesso, `error` → falha (retry esgotado).
- Estados **terminais** (sucesso/falha) exibem dot na UI.

## Cores de status (mesmas das badges da UI — não inventar outras)

| Status | Cor | Hex |
| --- | --- | --- |
| na fila | azul claro | `#7cc0ff` |
| em execução | laranja | `#ff9142` |
| sucesso | verde | `#46d19a` |
| falha | vermelho | `#ff5450` |

Tema visual: fundo escuro metálico `#0b0c0e`, bordas aço (`#b8bcc4`/`#8f96a3`), texto `#e8e6e1`, destaque laranja `#ff9142` (paleta dos diagramas).

## Fluxos (referência: `diagrams/`)

- **Fase 1 — listagem dinâmica**: `GET` HTMX → core lista workflows do registry → UI renderiza a lista.
- **Fase 2 — disparar e acompanhar**: `POST` HTMX → enqueue no Postgres (ACK) → DBOS executa o job e atualiza o status → polling de 2s atualiza o badge nas quatro cores.
- **Fase 3 — execuções de um workflow**: `GET /workflows/{nome}` lista todas as execuções (workflow_id + badge); click no workflow_id → detalhe: status, retries, status de cada retry — em falha: erro + step com falha.

## Convenções

- Idioma do código e dos commits: **inglês**; comentários/docstrings podem ser pt-BR.
- Rotas HTMX devolvem **fragmentos de HTML** (partials), não JSON.
- Não introduzir framework de frontend, bundler, npm ou banco diferente do Postgres sem decisão explícita.
- Postgres sempre **18** no compose (compatibilidade com o DBOS).
- Diagramas em `diagrams/` são a fonte de verdade do design: se o código divergir do diagrama, o diagrama vence — atualize o diagrama junto com o código.

## Comandos

```bash
go test ./...
docker compose up --build
```

Estrutura do serviço: um compose (`docker-compose.yml`) com `postgres:18` (porta `5432`) e o serviço FlowDeck (`:8080`). Um único `main.go` faz o boot (core DBOS → webserver HTMX); o código está em dois pacotes: `core/` (singleton do core DBOS: `Boot`/`Launch`/`Shutdown`/`List` em `core.go`, steps em `steps.go`, workflows em `workflows.go` — cada arquivo se registra no próprio `init()`, sem lista central) e `web/` (webserver HTMX + templates).
