# docs/revisao

Saídas do harness de revisão (`/revisar` e `/corrigir`, ver `CLAUDE.md`).

- `AAAA-MM-DD-relatorio.md` — relatório da revisão. A correção atualiza o status de cada id.
- `capturas/AAAA-MM-DD/{antes,depois}/` — screenshots com dados fictícios (nunca dados reais).
- `estado.json` — progresso lido pela statusline (`.claude/statusline.mjs`).

## Formato de `estado.json`

```json
{
  "relatorio": "docs/revisao/2026-10-15-relatorio.md",
  "fase": "lentes | aguardando-aprovacao | correcao | concluida",
  "ids": { "B-EF-01": "pendente | feito | bloqueado | pulado" }
}
```

Os workflows reescrevem o arquivo inteiro ao fim de cada fase. Arquivo ausente ou inválido faz a statusline mostrar só a branch.
