## ADDED Requirements

### Requirement: Similaridade lexical gera candidato quando o perfil não encontra nada

Quando o casamento pelo conjunto de formas do perfil de nomenclatura ativo não encontrar
nenhuma tabela para a entidade extraída de uma coluna, o sistema SHALL comparar essa entidade
por similaridade lexical de trigrama de caractere contra o nome de cada tabela do escopo, e
SHALL gerar candidato para toda tabela cuja similaridade atingir o limiar mínimo configurado.

Esta via MUST NOT ser acionada quando o casamento por perfil encontrar ao menos uma tabela,
mesmo que todas sejam descartadas por chave composta ou ausência de chave primária — nesse
caso o motivo do descarte já é registrado pela via existente, e tentar de novo por outro
caminho arrisca confundir por que uma tabela foi ignorada com por que outra apareceu.

Candidato nascido desta via SHALL passar pelo mesmo filtro de compatibilidade de tipo, pela
mesma regra de ambiguidade de alvo e pelo mesmo limiar de pontuação final que um candidato
nascido do casamento por perfil.

#### Scenario: Casamento por perfil ausente aciona a via por similaridade

- **WHEN** a entidade extraída de uma coluna não casa com nenhuma forma de nenhuma tabela do
  escopo, e uma tabela do escopo cruza o limiar mínimo de similaridade lexical
- **THEN** um candidato é gerado para essa tabela, carregando o sinal de similaridade lexical

#### Scenario: Casamento por perfil presente não aciona a via por similaridade

- **WHEN** a entidade extraída de uma coluna casa com pelo menos uma tabela pelo conjunto de
  formas do perfil, mesmo que essa tabela seja descartada por chave composta ou ausência de
  chave primária
- **THEN** a via por similaridade lexical não é avaliada para essa coluna

#### Scenario: Abaixo do limiar de similaridade não gera candidato

- **WHEN** o casamento por perfil não encontra nenhuma tabela, e nenhuma tabela do escopo
  atinge o limiar mínimo de similaridade lexical
- **THEN** nenhum candidato é gerado para essa coluna, como no comportamento anterior a esta
  via existir

#### Scenario: Múltiplas tabelas acima do limiar geram ambiguidade

- **WHEN** mais de uma tabela do escopo cruza o limiar mínimo de similaridade lexical para a
  mesma coluna
- **THEN** um candidato é gerado para cada uma, todos carregando o sinal de alvo ambíguo

#### Scenario: Tipo incompatível descarta mesmo acima do limiar de similaridade

- **WHEN** uma tabela cruza o limiar mínimo de similaridade lexical, mas o tipo base da coluna
  filha é incompatível com o tipo base da chave primária da tabela
- **THEN** nenhum candidato é gerado para esse par

#### Scenario: Alvo sem chave primária ou de chave composta é pulado com nota

- **WHEN** uma tabela cruza o limiar mínimo de similaridade lexical e não tem chave primária,
  ou tem chave primária composta
- **THEN** nenhum candidato é gerado para esse par, e o motivo é registrado como pulado
