---
name: eprocia-release
description: Publicar uma release do eprocIA — subir a productVersion, ensaiar com tag rc, conferir nomes, SHA256SUMS e atestação, escrever as notas e publicar. Use quando o dono pedir uma release; só a sessão principal executa, nunca um subagente.
---

# Release do eprocIA

Quem compila é o `release.yml`, a partir da tag. Nada sai da máquina local. A publicação muda o que o `irm .../install.ps1 | iex` instala (`releases/latest`), então cada passo abaixo só roda com o pedido do dono.

## Antes

1. A `main` está verde (CI e security) no commit que vai virar a release.
2. `wails.json`: `info.productVersion` igual à base da tag (`0.4.1` para `v0.4.1` e para `v0.4.1-rc1`). O workflow recusa a tag se for diferente. A mudança entra por PR, junto com o README.
3. As notas cobrem tudo desde a última release **publicada** (`gh release list`): um rascunho que nunca foi publicado não tem notas (foi o caso da v0.4.0). Escreva num arquivo fora do repositório.

## Ensaio

Tags `v*` não podem ser apagadas nem movidas (ruleset), as de ensaio também. Se o ensaio falhar, conserte e use `-rc2`.

```bash
git tag vX.Y.Z-rc1 <commit da main> && git push origin vX.Y.Z-rc1
gh run list --workflow release.yml --limit 1
gh run watch <id> --exit-status
```

O resultado é um rascunho marcado como pre-release. Baixe para uma pasta nova e vazia, fora do repositório, e não rode nada de dentro dela:

```bash
gh release download vX.Y.Z-rc1 --dir <pasta>
```

Confira:

- cinco arquivos: `eprocia-<versão>-windows-amd64.exe`, `eprocia-<versão>-windows-amd64-instalador.exe`, `install.ps1`, `LICENSE`, `SHA256SUMS.txt`;
- `SHA256SUMS.txt` com duas linhas no formato `<sha256 minúsculo> *<nome>`, fim de linha LF, só os dois exe, e as somas iguais às de `sha256sum <pasta>/*.exe`;
- `gh attestation verify <arquivo> -R jonyduque/eprocia --format json` com saída 0 para os dois exe e o `install.ps1`, e `sourceRepositoryRef` igual a `refs/tags/<tag>`. O `install.ps1` não muda entre releases, então a lista traz também atestações de tags antigas: procure a da tag atual;
- versão dos exe pelos campos numéricos (`[Diagnostics.FileVersionInfo]::GetVersionInfo(...).FileMajorPart` etc.). O `ProductVersion` do exe principal sai 0.0.0.0, e isso é esperado;
- `install.ps1` igual ao da `main` (`MSYS_NO_PATHCONV=1 git show origin/main:install.ps1 | sha256sum`) e sem BOM.

Depois apague só o rascunho, sem `--cleanup-tag`: `gh release delete vX.Y.Z-rc1 --yes`.

## Release

```bash
git tag vX.Y.Z <mesmo commit> && git push origin vX.Y.Z
```

Repita a conferência no rascunho novo. O exe principal sai com a mesma soma do ensaio (a tag não entra no binário); o instalador muda.

```bash
gh release edit vX.Y.Z --title "eprocIA vX.Y.Z" --notes-file <notas> --draft=false --latest
gh api repos/jonyduque/eprocia/releases/latest --jq .tag_name   # vX.Y.Z
curl -sL https://github.com/jonyduque/eprocia/releases/latest/download/SHA256SUMS.txt
```

## Depois

- Tire do README o que só valia antes da release (notas do tipo "a main está à frente da versão publicada").
- Não rode o `install.ps1` real para testar: ele encerra e substitui o eprocIA instalado do dono.
