## 1. A nota vem do CHANGELOG

- [x] 1.1 `CHANGELOG.md`, com as seções da v0.1.0 e da v0.1.1
- [x] 1.2 `scripts/release-notes.sh` extrai a seção da tag
- [x] 1.3 O workflow passa `--release-notes`, e a geração por commits é desligada

## 2. Falhar antes de publicar

- [x] 2.1 Seção ausente encerra o script com erro, antes do goreleaser
- [x] 2.2 O passo vem antes de qualquer publicação no workflow

## 3. Procedimento

- [x] 3.1 `docs/RELEASING.md` ganha o passo, com o comando de conferência

## 4. Verificação

- [x] 4.1 Extração conferida nas duas versões, incluindo a última do arquivo
- [x] 4.2 `goreleaser check` limpo
