---
description: Corrige os achados aprovados da última revisão do eprocIA, um commit por id, com gates de build, benchmark e visual
argument-hint: "<ids separados por espaço ou vírgula | todos> [relatorio=docs/revisao/AAAA-MM-DD-relatorio.md]"
---

Corrija os achados aprovados pelo usuário com o harness do repositório. Argumentos: `$ARGUMENTS`.

1. **Relatório:** use o `relatorio=` dos argumentos ou, na falta dele, o `relatorio` de `docs/revisao/estado.json`; se nenhum existir, use o `docs/revisao/*-relatorio.md` mais recente. A `data` é a do nome do relatório.
2. **Ids:** os ids dos argumentos (ex.: `B-ES-01 B-EF-02 F-RS-01`). `todos` significa todos os ids com `Status: pendente` no relatório. Confira que cada id existe no relatório e mostre ao usuário a lista final antes de começar.
3. **Árvore limpa:** `git status --porcelain` não pode ter arquivos rastreados modificados. Se tiver, pare e pergunte ao usuário o que fazer; não descarte nada.
4. **eprocia.exe:** se estiver aberto a partir de `build/bin`, peça ao usuário para fechá-lo pela bandeja (Sair); o gate de build precisa do arquivo livre. Nunca encerre o processo por conta própria.
5. **Config:** leia `.claude/review.json` e use o objeto inteiro como `config`.
6. **Atribuição:** monte `atribuicao` com as linhas de atribuição de commit que esta sessão usa (as do system-reminder; ex.: `Co-Authored-By: ...` e `Claude-Session: ...`), separadas por quebra de linha.
7. **Workflow:** chame a ferramenta Workflow com `name: "eprocia-corrigir"` (se a sessão responder que o workflow não existe, porque foi aberta antes dele, use `scriptPath: ".claude/workflows/eprocia-corrigir.js"` com o caminho absoluto) e `args: { data, relatorio, ids, config, atribuicao, grupos }` como objeto JSON. `grupos` é opcional: listas de ids da mesma camada que viram uma só tarefa e um só commit. Agrupe achados pequenos que mexem no mesmo código (ex.: os B-GM, ou contraste e paleta do frontend); cada rodada custa ≈20 min fixos (TDD, `test -race`, `wails build`, revisor), então agrupar corta o tempo total. Mostre os grupos ao usuário junto com a lista de ids. A ordem dos ids nos argumentos é ignorada: o workflow ordena as tarefas pela lente, conforme `config.correcao.ordem`. Para outra ordem (ex.: testes antes de uma refatoração), passe uma cópia do config com `correcao.ordem` reordenada.
8. **Ao terminar:** mostre o status por id (`feito`, `bloqueado`, `pulado`), os commits criados (`git log --oneline <base>..HEAD`), o resumo da revisão final da branch e os pares de capturas antes/depois dos achados visuais, para o usuário aprovar olhando as imagens. Merge e push ficam com o usuário; não faça nenhum dos dois.
