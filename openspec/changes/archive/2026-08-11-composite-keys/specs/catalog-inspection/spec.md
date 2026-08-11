## MODIFIED Requirements

### Requirement: Tabela pulada é registrada, nunca silenciada

Tabela que não puder ser analisada SHALL ser registrada na cobertura com o motivo, e a execução SHALL continuar. Os motivos SHALL cobrir: falta de privilégio `SELECT`, ausência de chave primária, tabela particionada e herança de tabela.

Chave primária composta MUST NOT ser motivo de tabela não analisada, e o motivo correspondente SHALL deixar de existir. Motivo enumerado sem produtor é pior que motivo ausente: ele aparece no contrato JSON como possibilidade que nunca ocorre, e um consumidor que trate esse caso escreve código para um mundo que não existe mais.

Tabela particionada SHALL ser lida a partir da tabela pai, e as partições MUST NOT ser iteradas separadamente.

#### Scenario: Falta de privilégio

- **WHEN** o papel conectado não tem `SELECT` numa tabela do escopo
- **THEN** a tabela aparece na lista de puladas por privilégio, e a execução prossegue

#### Scenario: Forma não suportada

- **WHEN** uma tabela não tem chave primária
- **THEN** ela aparece na cobertura com o motivo correspondente

#### Scenario: Chave composta é analisada como qualquer outra

- **WHEN** o schema contém tabelas com chave primária composta
- **THEN** elas contam como analisadas e não aparecem entre as não suportadas

#### Scenario: Partições não são iteradas

- **WHEN** o schema contém uma tabela particionada com várias partições
- **THEN** o modelo contém a tabela pai e não contém uma entrada por partição
