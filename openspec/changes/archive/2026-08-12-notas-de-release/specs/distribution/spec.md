## ADDED Requirements

### Requirement: A nota de release é escrita, e sua ausência aborta o release

A nota de release SHALL vir de um changelog versionado no repositório, escrito para quem usa a ferramenta. Ela MUST NOT ser gerada a partir do histórico de commits.

O release SHALL falhar antes de publicar qualquer artefato quando o changelog não tiver seção para a versão sendo lançada.

#### Scenario: Versão sem seção não é publicada

- **WHEN** o workflow de release roda para uma tag que não tem seção correspondente no changelog
- **THEN** ele falha antes de publicar qualquer artefato, dizendo qual seção falta

#### Scenario: A nota publicada é a que foi escrita

- **WHEN** um release é publicado para uma versão que tem seção no changelog
- **THEN** o corpo da release é o conteúdo dessa seção, sem hashes de commit
