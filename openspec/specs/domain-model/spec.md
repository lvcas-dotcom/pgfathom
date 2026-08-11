# domain-model Specification

## Purpose
A definir - criado ao arquivar alteração bootstrap-core-model. Atualize o Purpose após o arquivamento.
## Requirements
### Requirement: Modelo é puro e sem I/O

O pacote `internal/model` SHALL conter exclusivamente tipos e funções puras. Ele MUST NOT importar nenhuma outra camada do projeto, e MUST NOT importar pacote de acesso a banco, a rede ou a sistema de arquivos.

Esta restrição SHALL ser verificada por teste automatizado, não por revisão humana, porque é a dependência mais fácil de introduzir por acidente e a que degrada a testabilidade de todo o resto.

#### Scenario: Ausência de dependência de camada

- **WHEN** o grafo de importação de `internal/model` é inspecionado
- **THEN** ele não contém nenhum pacote `internal/*` do projeto nem `pgx`

### Requirement: Proveniência é explícita e inseparável do dado

O modelo SHALL distinguir estruturalmente três origens de conhecimento sobre um relacionamento: **declarado** no catálogo, **evidenciado** por uso em view, função ou log de queries, e **inferido** por heurística.

Chave estrangeira declarada SHALL residir em campo distinto de candidato inferido. MUST NOT existir campo ou construção que permita a um relacionamento inferido ser lido como declarado.

#### Scenario: FK declarada não se confunde com inferida

- **WHEN** uma tabela tem uma FK declarada no catálogo e um candidato inferido para a mesma coluna
- **THEN** os dois ocupam campos distintos do modelo e são distinguíveis sem inspecionar conteúdo

#### Scenario: Sinal registra sua origem

- **WHEN** um candidato recebe um sinal
- **THEN** o sinal identifica de qual origem veio — nome, comentário, índice, view, função, log de queries ou estatística

### Requirement: Chave estrangeira declarada carrega estado de validação

Toda FK lida do catálogo SHALL carregar o valor de `pg_constraint.convalidated`. Uma constraint `NOT VALID` impede violações novas mas nunca verificou linhas preexistentes, e tratá-la como garantia de integridade é uma das formas de a ferramenta estar errada com confiança.

#### Scenario: Constraint não validada é distinguível

- **WHEN** o catálogo contém uma FK criada com `NOT VALID` e nunca validada
- **THEN** o modelo a representa com o estado de validação falso, e não como FK íntegra

### Requirement: Estatística de uso é inseparável do timestamp de reset

Todo contador de uso de tabela SHALL vir acompanhado do momento do último reset de estatística. Contador de uso sem esse timestamp não tem significado: as estatísticas zeram em `pg_stat_reset()` e em `pg_upgrade`, e são locais ao nó, de modo que uma tabela lida apenas em réplica aparece com zero leituras na primária.

O modelo MUST NOT expor um contador de uso de forma que permita interpretá-lo sem o timestamp.

#### Scenario: Contador sempre acompanhado

- **WHEN** o modelo carrega estatística de uso de uma tabela
- **THEN** o timestamp de reset está disponível junto, ou o estado é explicitamente marcado como desconhecido

### Requirement: Nenhum campo serializável transporta valor de dado do usuário

Nenhum campo de tipo exportado ou serializável do modelo SHALL conter valor lido de tabela do usuário. O que o modelo transporta são contagens, proporções, timestamps e nomes de objeto de catálogo.

Estatística de coluna proveniente de `pg_stats` é o caso perigoso: `most_common_vals` e `histogram_bounds` **são** dados do usuário. Quando o modelo precisar carregá-los para pontuação, eles SHALL residir em campo não exportado e não serializável, e SHALL ser descartados assim que produzirem sua métrica.

Campo de texto livre destinado a diagnóstico — descrição de sinal, motivo de descarte, mensagem de erro — SHALL conter apenas nome de objeto, nunca valor.

#### Scenario: Serialização não vaza

- **WHEN** qualquer estrutura do modelo é serializada em JSON
- **THEN** nenhum valor originado de tabela do usuário aparece no resultado

#### Scenario: Limite do histograma não é exportado

- **WHEN** estatística de coluna é carregada de `pg_stats`
- **THEN** os limites do histograma residem em campo não exportado e não aparecem em nenhuma serialização

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

### Requirement: Vereditos são enumerados e incluem o caso de não conclusão

O conjunto de vereditos SHALL ser fechado e SHALL incluir explicitamente um veredito para candidato que não pôde ser avaliado — por timeout, por falta de privilégio ou por recair em caso não suportado.

Candidato não avaliado MUST NOT ser representado como rejeitado. São coisas diferentes: rejeitado significa que a evidência derrubou a hipótese; não avaliado significa que não houve evidência.

#### Scenario: Timeout não vira rejeição

- **WHEN** a validação de um candidato estoura o `statement_timeout`
- **THEN** o veredito é o de não avaliado, com o motivo registrado, e não o de rejeitado

### Requirement: Cobertura é parte do modelo

O modelo SHALL incluir uma estrutura de cobertura descrevendo o que foi efetivamente analisado sobre o que existe: total de tabelas, tabelas analisadas, tabelas puladas por falta de privilégio, tabelas excluídas por filtro, tabelas em caso não suportado, candidatos gerados, candidatos validados e candidatos que estouraram timeout.

Cobertura MUST NOT ser opcional nem construída no momento da renderização. Um relatório limpo precisa significar "analisei e está limpo", nunca "não consegui olhar".

#### Scenario: Tabela sem privilégio é contabilizada

- **WHEN** o usuário de conexão não tem `SELECT` em uma tabela do escopo
- **THEN** a tabela aparece na lista de puladas por privilégio da cobertura

#### Scenario: Cobertura acompanha todo resultado

- **WHEN** um resultado de análise é construído
- **THEN** a estrutura de cobertura está presente e preenchida

### Requirement: A estimativa de linhas tem um único intérprete

A resolução de `pg_class.reltuples` — incluindo a sentinela que distingue "nunca analisada" de "vazia" — SHALL ter uma única implementação, no modelo, e toda camada que precise de estimativa de linhas MUST consumi-la por ela.

Desde o PostgreSQL 14 uma tabela nunca analisada devolve `-1`. Lida como contagem, ela parece vazia: a pontuação passa a tratá-la como tabela de domínio e a amostragem passa a tratá-la como cabendo inteira no alvo. São dois comportamentos errados e diferentes a partir da mesma leitura, o que é o desfecho previsível de duas implementações da mesma regra.

#### Scenario: Sentinela nunca é confundida com zero

- **WHEN** uma tabela nunca analisada tem sua estimativa de linhas consultada
- **THEN** o resultado declara que a estimativa é desconhecida, e nenhuma camada a interpreta como zero

#### Scenario: Uma implementação só

- **WHEN** as camadas que consomem estimativa de linhas são inspecionadas
- **THEN** todas resolvem a sentinela pela mesma função do modelo

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

