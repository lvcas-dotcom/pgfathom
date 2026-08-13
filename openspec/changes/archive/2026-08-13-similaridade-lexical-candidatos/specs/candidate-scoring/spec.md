## ADDED Requirements

### Requirement: Sinal de similaridade lexical pontua abaixo do casamento normalizado por perfil

O sistema SHALL emitir um sinal de similaridade lexical para candidato nascido da via de
fallback por trigrama, com peso proporcional à similaridade medida, escalado linearmente até
um teto estritamente menor que o peso do sinal de nome normalizado por perfil.

Mesmo no teto — o par mais lexicalmente próximo possível dentro do limiar — a evidência de
similaridade é mais fraca que uma convenção de nomenclatura confirmada pelo perfil, porque não
carrega a mesma garantia de regra de linguagem: é proximidade de string, não convenção
reconhecida.

Um candidato SHALL carregar no máximo um sinal de nome — exato, normalizado por perfil, ou de
similaridade lexical —, nunca mais de um, porque as três vias de casamento são mutuamente
exclusivas por construção: a via por similaridade só é avaliada quando as outras duas não
encontraram nenhuma tabela.

#### Scenario: Similaridade no teto ainda pontua menos que normalizado

- **WHEN** dois candidatos são idênticos exceto pelo sinal de nome, um com casamento
  normalizado por perfil e outro com similaridade lexical no valor máximo possível
- **THEN** o de casamento normalizado por perfil tem score maior ou igual

#### Scenario: Peso escala com a similaridade medida

- **WHEN** dois candidatos nascem da via de similaridade lexical, um com similaridade maior
  que o outro, ambos acima do limiar mínimo
- **THEN** o de similaridade maior tem peso de sinal de nome estritamente maior

#### Scenario: Nenhum candidato carrega mais de um sinal de nome

- **WHEN** um candidato de qualquer origem é inspecionado
- **THEN** ele carrega exatamente um sinal entre nome exato, nome normalizado e similaridade
  lexical, nunca dois

### Requirement: Limiar de geração por similaridade é distinto do limiar de pontuação

O sistema SHALL manter o limiar mínimo de similaridade lexical — que decide se a via de
fallback gera candidato — configurável separadamente do limiar de pontuação que decide se um
candidato sobrevive até a validação. Os dois SHALL ter valores padrão independentes e SHALL
poder ser ajustados sem afetar um ao outro.

Um candidato pode cruzar o limiar de similaridade e ainda ser descartado pelo limiar de
pontuação, se os demais sinais forem fracos — o mesmo comportamento que já vale hoje para
casamento normalizado fraco por perfil.

#### Scenario: Limiares ajustáveis independentemente

- **WHEN** o usuário fornece um limiar de similaridade diferente do padrão, sem alterar o
  limiar de pontuação
- **THEN** o conjunto de candidatos gerados pela via de similaridade muda, e o corte que
  decide sobrevivência à validação permanece o mesmo

#### Scenario: Cruzar o limiar de similaridade não garante sobreviver ao limiar de pontuação

- **WHEN** um candidato nascido da via de similaridade lexical cruza o limiar mínimo de
  similaridade, mas a soma de todos os seus sinais fica abaixo do limiar de pontuação
- **THEN** o candidato é descartado antes da validação, com o motivo registrado, como
  qualquer outro candidato abaixo do limiar de pontuação
