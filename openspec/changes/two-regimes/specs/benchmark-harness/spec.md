## ADDED Requirements

### Requirement: A medição tem duas regimes, e cada número diz qual

Cada schema SHALL ser medido em duas regimes, e o relatório SHALL publicar as duas:

**Parcial** — metade das chaves declaradas é removida e constitui o gabarito; a outra metade permanece no catálogo. O recall SHALL ser calculado apenas sobre a metade removida.

**Greenfield** — a metade restante também é removida, e o gabarito passa a ser o conjunto inteiro das chaves originais.

A regime SHALL aparecer no rótulo de cada número publicado, e o relatório SHALL dizer qual das duas descreve a primeira execução contra um banco legado que declarou alguma integridade.

A detecção de nomenclatura deriva o afixo de referência das chaves que o schema **já declara**. Uma medição que remove todas elas mede a ferramenta sem essa camada: contra um schema real de gestão municipal, isso produziu 1,8% onde a ferramenta entrega 82,2% com as chaves no lugar. Publicar apenas esse número, sob o rótulo "recall", descreve o procedimento em vez da ferramenta.

Manter a regime greenfield é igualmente necessário: banco que não declara integridade nenhuma existe, é o caso mais difícil, e é onde a ferramenta se justifica.

#### Scenario: As duas regimes aparecem por schema

- **WHEN** um schema do corpus é medido
- **THEN** o relatório traz o recall da regime parcial e o da greenfield, cada um com o cenário nomeado

#### Scenario: A parcial mede só o que foi removido

- **WHEN** a regime parcial é medida
- **THEN** o denominador é a metade removida, e as chaves mantidas não entram no cálculo

#### Scenario: A detecção tem evidência na parcial

- **WHEN** um schema cuja convenção de afixo não está em nenhum perfil embarcado é medido na regime parcial
- **THEN** a detecção lê o afixo das chaves mantidas, e o recall reflete isso

### Requirement: A divisão do gabarito é determinística

A escolha de qual metade das chaves é removida na regime parcial SHALL ser derivada da ordenação das relações, e MUST NOT depender de sorteio, com ou sem semente.

Duas execuções sobre o mesmo corpus SHALL produzir a mesma divisão e, portanto, o mesmo número.

A divisão SHALL alternar posições na ordem das relações, em vez de cortar a lista pela metade, para que as chaves removidas se distribuam entre tabelas em vez de se concentrarem num prefixo do alfabeto.

O relatório de recall é arquivo versionado. O diff dele precisa significar "o comportamento mudou", nunca "o sorteio caiu diferente".

#### Scenario: Execuções repetidas dividem igual

- **WHEN** o mesmo corpus é medido duas vezes
- **THEN** as mesmas chaves compõem o gabarito da regime parcial, e o recall publicado é idêntico

## MODIFIED Requirements

### Requirement: O relatório decompõe por origem de sinal e publica o custo

Cada schema SHALL ser medido em três configurações — perfil embarcado sozinho, com detecção de nomenclatura, e com evidência de uso — em cada uma das duas regimes, e o relatório SHALL publicar o recall de cada combinação.

O relatório SHALL publicar também, por execução: tempo por etapa, número de candidatos em cada ponto do funil, número de queries de validação e as tabelas fora da análise com o motivo.

A decomposição é o argumento central do projeto em número: mostra quanto o casamento de nome recupera sozinho e quanto a evidência de uso acrescenta. Um total sem ela não distingue uma ferramenta que lê convenções de uma que lê código.

Cruzada com as duas regimes, a decomposição mostra mais: na parcial, a linha da detecção mede o que ela aprende de um schema que declarou parte da integridade; na greenfield, mede o que sobra quando não há nada a aprender.

#### Scenario: As três configurações aparecem

- **WHEN** um schema é medido
- **THEN** o relatório traz o recall do perfil sozinho, o com detecção e o com evidência de uso, em cada regime

#### Scenario: O recorte aparece junto do número

- **WHEN** um schema tem tabelas fora da análise ou chaves apontando para fora do escopo medido
- **THEN** o relatório as nomeia, e a taxa publicada declara sobre qual denominador foi calculada
