# FlowDeck — Diagramas

Estilo: pele escura metálica (fundo `#0b0c0e`, bordas aço, destaque laranja) com as cores de status
`na fila` (azul claro), `em execução` (laranja), `sucesso` (verde) e `falha` (vermelho).

## Arquitetura

- **Uma máquina, docker-compose**: o serviço **FlowDeck (Go)** contém a camada **Web (HTMX, :8080)**
  que faz boot do **core DBOS**; o core conversa com o **Postgres 18 (:5432)**. O DBOS não sobe
  nenhum webserver — a web é a abstração sobre o boot do core.

## Fase 1 — Listagem dinâmica

- O navegador pede a página; o core lista os workflows registrados (registry, **zero hardcode na web**);
  a UI renderiza a lista via HTMX.

## Fase 2 — Disparar, executar e acompanhar

- O usuário seleciona o workflow e dispara (POST HTMX); webserver → core → Postgres 18 (com retorno);
- Em paralelo: o executor do DBOS seleciona o job na fila (`SELECT NEXT`), executa e
  atualiza o status (`UPDATE SUCESSO`);
- polling de 2s (`hx`) consulta o status e atualiza o badge nas quatro cores.

## Estados

- `na fila → em execução (retry) → sucesso / falha` — terminais com dots; mesmas cores dos badges da UI.

## Arquivos

- `arquitetura.html`
- `sequencia-fase-1.html`
- `sequencia-fase-2.html`
- `estados-workflow.html`
