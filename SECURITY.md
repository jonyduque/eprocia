# Segurança

## Como reportar uma vulnerabilidade

Não abra uma issue pública. Use **Security → Report a vulnerability** neste repositório (reporte privado do GitHub). Diga o que acontece, como reproduzir e a versão do eprocIA.

Interessam em especial:

- o `install.ps1` (roda com `irm | iex` e cria uma tarefa agendada quando o PowerShell está elevado);
- a leitura de ZIPs e PDFs vindos de fora (caminhos dentro do ZIP, arquivos malformados);
- os binários embutidos de OCR (Tesseract e Poppler) e a pasta para onde são extraídos;
- o que o frontend consegue pedir ao backend pelos bindings do Wails.

O eprocIA processa autos judiciais localmente e não faz chamadas de rede durante o processamento. Um caminho que envie dados para fora é uma vulnerabilidade.

## Versões com correção

Só a release mais recente (e a pre-release mais nova, se houver).

## Como verificar uma release

A partir da v0.3.1, o exe, o instalador e o `install.ps1` são compilados pelo GitHub Actions a partir da tag e levam uma atestação de procedência:

```sh
gh attestation verify eprocia-<versão>-windows-amd64.exe -R jonyduque/eprocia
```

O `SHA256SUMS.txt` da release confere a integridade do download, como faz o `install.ps1`.
