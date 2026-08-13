## ADDED Requirements

### Requirement: Artefato escrito é artefato anunciado

Quando artefatos forem escritos, a execução SHALL informar quais arquivos foram escritos, onde, e que eles devem ser lidos antes de executados — em qualquer formato de saída.

O anúncio MUST NOT corromper a saída destinada a consumo programático: no formato JSON ele SHALL ir para stderr, e stdout SHALL permanecer um documento parseável.

Escrever SQL sem dizer que escreveu deixa em disco arquivos que ninguém foi avisado para revisar, o que contraria a regra de que nada é executado por conta de ninguém.

#### Scenario: Execução comum anuncia o que escreveu

- **WHEN** `discover --out` roda com o formato de terminal
- **THEN** o relatório termina listando os arquivos escritos e o lembrete de revisá-los

#### Scenario: O anúncio não corrompe o documento

- **WHEN** `discover --out --format json` roda
- **THEN** stdout continua sendo um documento JSON parseável, e o anúncio aparece em stderr
