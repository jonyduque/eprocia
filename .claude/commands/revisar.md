---
description: Revisão geral do eprocIA (backend, frontend ou tudo) com lentes, refutação cética e relatório priorizado
argument-hint: "[backend|frontend|tudo] [lentes=EF,ES,...]"
---

Rode a revisão do eprocIA com o harness do repositório. Argumentos: `$ARGUMENTS`.

1. **Escopo:** o primeiro argumento é `backend`, `frontend` ou `tudo` (padrão `tudo`). Um argumento `lentes=A,B` restringe às lentes com esses ids (ex.: `lentes=TE` para a execução de fumaça).
2. **Data:** a data de hoje no formato `AAAA-MM-DD`.
3. **Config:** leia `.claude/review.json` e use o objeto inteiro como `config`.
4. **Estado:** escreva `docs/revisao/estado.json` com `{"relatorio": "docs/revisao/<data>-relatorio.md", "fase": "lentes", "ids": {}}`.
5. **Custo:** diga ao usuário quantos agentes vão rodar antes de iniciar: por lente, um revisor e um cético; mais 1 de captura visual se o escopo incluir frontend; mais 1 de síntese. `tudo` ≈ 22, `backend` ≈ 11, `frontend` ≈ 12.
6. **Workflow:** chame a ferramenta Workflow com `name: "eprocia-revisao"` (se a sessão responder que o workflow não existe, porque foi aberta antes dele, use `scriptPath: ".claude/workflows/eprocia-revisao.js"` com o caminho absoluto) e `args: { escopo, data, config, lentes }` (omita `lentes` se não vier). Passe `args` como objeto JSON, não como string.
7. **Ao terminar:** leia o relatório gerado e apresente ao usuário, em poucas linhas, a tabela-resumo (id, título, gravidade, custo) e quantos achados foram derrubados. Termine dizendo como aprovar: `/corrigir <ids>` (ou `/corrigir todos`). Não corrija nada nesta etapa.

Se o workflow falhar, mostre a linha decisiva do erro e não tente consertar à mão sem perguntar.
