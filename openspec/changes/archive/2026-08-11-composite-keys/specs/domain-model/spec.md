## ADDED Requirements

### Requirement: Candidato referencia chave, não coluna

Um candidato SHALL referenciar, de cada lado, uma chave: schema, tabela e lista **ordenada** de colunas. Chave de coluna única SHALL ser o caso de aridade 1 da mesma estrutura, e MUST NOT existir campo alternativo que represente o caso escalar em separado.

A ordem das colunas é parte da identidade da chave: a DDL de chave estrangeira corresponde posição a posição, e uma lista fora de ordem produz constraint que liga as colunas erradas sem erro de sintaxe.

Duas representações do mesmo fato divergiriam. O consumidor que lesse o campo escalar de um candidato composto emitiria metade de uma chave, e emitiria em silêncio.

#### Scenario: Aridade 1 e aridade n são a mesma estrutura

- **WHEN** um candidato de coluna única e um candidato de chave composta são inspecionados
- **THEN** ambos carregam a mesma estrutura de referência, diferindo apenas no comprimento da lista de colunas

#### Scenario: A ordem das colunas é preservada

- **WHEN** um candidato aponta para uma chave primária composta
- **THEN** as colunas do lado pai aparecem na ordem da chave primária, e as do lado filho na posição correspondente a cada uma

## MODIFIED Requirements

### Requirement: Validação reporta contenção em duas dimensões

O resultado de validação SHALL registrar contenção por linha e contenção por valor distinto como métricas separadas, junto das contagens de órfãos correspondentes. Para chave de mais de uma coluna, "valor distinto" SHALL significar tupla distinta.

As duas contam histórias diferentes e nenhuma substitui a outra. Um único valor inválido repetido em um milhão de linhas derruba a contenção por linha e mal arranha a por valor distinto; muitos valores raros e inválidos fazem o contrário. Divergência grande entre as duas é, em si, um achado.

O resultado SHALL registrar também o número máximo de linhas por valor distinto, que é o que distingue cardinalidade um-para-um de um-para-muitos.

O resultado SHALL registrar quantas linhas foram isentadas por terem NULL em parte da chave sem terem em todas. Sob `MATCH SIMPLE` — o padrão do SQL e o que a ferramenta emite — essas linhas escapam da integridade referencial por regra, e escapam em silêncio. São zero por construção em chave de coluna única, e um achado em chave composta.

#### Scenario: Ambas as contenções presentes

- **WHEN** uma validação completa
- **THEN** o resultado contém contenção por linha, contenção por valor, contagem de órfãos por linha, contagem de órfãos por valor e o máximo de linhas por valor

#### Scenario: Método de validação registrado

- **WHEN** uma validação é executada em modo amostrado
- **THEN** o resultado registra que foi amostrada e quantas linhas foram examinadas

#### Scenario: Linha parcialmente nula é contada, não considerada órfã

- **WHEN** a tabela filha tem linhas com NULL em parte das colunas da chave composta e valor nas demais
- **THEN** elas aparecem na contagem de linhas isentas por `MATCH SIMPLE` e não entram em nenhuma contagem de órfãos
