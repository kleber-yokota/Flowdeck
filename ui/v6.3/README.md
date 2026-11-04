# v6.3 "Petróleo" (FINAL)

Soft Terminal com pele de petróleo (teal `#45d6b5`, fósforo nos estados).
Escala global **1.25×** (root `20px`) + constantes px escaladas.
`style.css` standalone — sem `@import`.

## Páginas

- `index.html` — boxes `running` / `queued` / `cancelled · error` →
  `runnings.html`, `queued.html`, `failed.html`; catálogo só nomes;
  paginação 20 por página (`?page=N`).
- `runnings.html` / `queued.html` / `failed.html` — as listas das boxes.
- `workflow.html` — breadcrumb do arquivo (`workflows/pipeline/process.go`);
  dividida em 2: esquerda dispatch + execuções · **direita o grafo top-down**
  (fiel ao emissor do app `web/graph`: boxes = blocos L0/L1, símbolos
  ◉ child · ∥ async · ⋯ wait · ↻ loop, setas nas edges).
- `execution.html` — actions **no topo**; dividida em 2: esquerda step log ·
  **direita o executed path** (um box por step completo, top-down; cauda
  tracejada = "ainda rodando").
- `execution-child.html` — a child aparece sob o step que a lançou
  (`StepInfo.ChildWorkflowID`); grafo à direita com o box da child (◉).

## Regras do grafo (executed path)

- Box aparece quando o step **completa** (`StepInfo` via poll 2s — mesma
  honestidade do step log).
- A **cauda tracejada** é o único "em execução" — o DBOS não expõe o
  próximo step, então nada é inventado.
- Child vira box (com ◉) quando o `ChildWorkflowID` aparece no step pai.
- Estado na stroke do box: `queued` / `running` / `success` / `error` /
  `cancelled`; cauda = tracejado.
- Geometria proporcional ao texto: font 12px mono ≈ 7.2px/char, box
  h=36, largura = texto + padding, gaps 20/32, outline pad 10/14.

## Produção (mapeamento)

- Boxes: `ListWorkflows` com `status=ENQUEUED`/`RUNNING`; failed = último
  run por workflow com erro (via `Port.Executions`).
- Paginação: `limit=20, offset`.
- Cancel/resume: `POST /cancel/{id}`, `POST /resume/{id}` (existentes).
- Poll: `GET /status/{id}`, `GET /executions/{id}/steps` (existentes).
