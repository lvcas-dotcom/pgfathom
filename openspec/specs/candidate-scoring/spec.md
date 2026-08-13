# candidate-scoring Specification

## Purpose
TBD - created by archiving change fk-candidate-inference. Update Purpose after archive.
## Requirements
### Requirement: Todo score é explicável pelos sinais que o produziram

Todo candidato SHALL carregar a lista de sinais que compuseram seu score, cada um com sua origem e seu peso.

Um número sem procedência não é auditável, e a ferramenta inteira depende de o usuário poder discordar de uma pontuação olhando o que a sustentou. O score MUST NOT ser calculável por caminho que não deixe rastro nos sinais.

#### Scenario: Sinais acompanham o candidato

- **WHEN** um candidato é pontuado
- **THEN** a lista de sinais que produziram o score acompanha o candidato

#### Scenario: Score reconstruível

- **WHEN** os pesos dos sinais de um candidato são combinados
- **THEN** o resultado é o score registrado

### Requirement: Score é normalizado entre zero e um

O score SHALL ficar sempre no intervalo de zero a um, independentemente de quantos sinais positivos ou negativos incidam.

O limiar de corte é configurável pelo usuário, e um intervalo que varia com o número de sinais tornaria o valor do limiar impossível de raciocinar.

#### Scenario: Muitos sinais positivos não estouram o teto

- **WHEN** um candidato acumula todos os sinais positivos disponíveis
- **THEN** o score é no máximo um

#### Scenario: Muitos sinais negativos não furam o piso

- **WHEN** um candidato acumula todos os sinais negativos disponíveis
- **THEN** o score é no mínimo zero

### Requirement: Sinais positivos e seus pesos relativos

O sistema SHALL emitir os seguintes sinais positivos, em ordem decrescente de peso:

Casamento exato do nome de entidade com o nome da tabela, sem normalização, é o sinal de nome mais forte. Tipo base idêntico à chave alvo é forte. Alvo único para aquele nome é forte, porque ambiguidade é o principal gerador de ruído. Coluna já possuir índice é moderado, porque indica que alguém faz join por ela. Comentário de coluna ou de tabela mencionando a entidade alvo é moderado. Coluna ser `NOT NULL` é fraco mas positivo.

Concordância de aridade — todas as colunas de uma chave de mais de uma coluna casando de uma vez — SHALL emitir um sinal positivo próprio, uma única vez, com peso crescendo sublinearmente na aridade. Ele MUST NOT ser emitido para chave de coluna única.

Os demais sinais SHALL ser avaliados sobre a chave inteira e emitidos uma vez por candidato, qualquer que seja a aridade: tipo compatível significa todas as posições compatíveis; coluna indexada significa índice cujas colunas iniciais são a chave filha na ordem dela; `NOT NULL` significa todas as colunas não nulas.

Emitir os sinais existentes uma vez por coluna seria errado duas vezes. A soma satura no mesmo teto, então a aridade viraria um caminho indireto para fixar o score no máximo; e o score deixaria de ser explicável, porque uma chave de quatro colunas exibiria vinte sinais dizendo quatro fatos.

#### Scenario: Casamento exato pontua mais que normalizado

- **WHEN** dois candidatos são idênticos exceto pela forma de casamento, um exato e outro normalizado
- **THEN** o de casamento exato tem score estritamente maior

#### Scenario: Tipo idêntico pontua mais que apenas compatível

- **WHEN** dois candidatos são idênticos exceto pelo tipo, um idêntico e outro apenas compatível
- **THEN** o de tipo idêntico tem score estritamente maior

#### Scenario: Coluna indexada pontua mais

- **WHEN** dois candidatos são idênticos exceto por um ter índice na coluna filha
- **THEN** o indexado tem score estritamente maior

#### Scenario: Concordância de aridade pontua mais que uma coluna só

- **WHEN** dois candidatos são idênticos exceto pela aridade, um de duas colunas e outro de uma
- **THEN** o de duas colunas tem score estritamente maior, e carrega o sinal de aridade uma única vez

#### Scenario: Aridade não multiplica os outros sinais

- **WHEN** um candidato de chave composta é inspecionado
- **THEN** cada sinal de nome, tipo, índice, comentário e nulidade aparece no máximo uma vez

#### Scenario: Casamento espelho não reivindica evidência de nome

- **WHEN** um candidato composto nasce de casamento espelho, em que nada no filho nomeia o alvo
- **THEN** ele carrega o sinal de aridade e nenhum sinal de nome, e pontua abaixo de um candidato de coluna única cujo nome aponta para o alvo

#### Scenario: Índice conta pela ordem da chave

- **WHEN** a tabela filha tem índice sobre as colunas da chave em ordem diferente da chave
- **THEN** o sinal de coluna indexada não é emitido

### Requirement: Sinais negativos e o que eles protegem

O sistema SHALL emitir sinais negativos para: nome de entidade genérico apontando para tabela pequena, tipicamente tabela de domínio como `status` ou `tipo`; múltiplas tabelas candidatas para o mesmo nome; e tipo compatível mas não idêntico.

O sinal de nome genérico existe porque essas relações costumam ser reais e pouco interessantes, e sem penalidade elas dominariam o relatório por volume, empurrando os achados valiosos para o fim.

#### Scenario: Nome genérico com alvo pequeno é penalizado

- **WHEN** a coluna `status_id` casa com uma tabela `status` de poucas linhas
- **THEN** o candidato carrega o sinal de nome genérico e tem score reduzido

#### Scenario: Ambiguidade é penalizada

- **WHEN** um nome de entidade casa com mais de uma tabela
- **THEN** todos os candidatos resultantes carregam o sinal de alvo ambíguo e têm score reduzido

### Requirement: Corte por limiar configurável com motivo registrado

Candidatos com score abaixo de um limiar configurável SHALL ser descartados antes de qualquer validação, e cada descarte SHALL registrar o motivo.

Este corte é o único mecanismo que impede a fase de validação de disparar milhares de anti-joins contra um banco de produção, e por isso o limiar precisa ser ajustável sem recompilar.

O padrão SHALL ser conservador o bastante para que uma execução em schema de centenas de tabelas produza um conjunto de candidatos que caiba numa revisão humana.

#### Scenario: Abaixo do limiar é descartado

- **WHEN** um candidato pontua abaixo do limiar configurado
- **THEN** ele não segue para validação, e o motivo do descarte fica registrado

#### Scenario: Descartes são reportáveis

- **WHEN** o usuário pede para ver os descartados
- **THEN** eles aparecem com score e motivo, para que ninguém se pergunte por que uma coluna óbvia foi ignorada

#### Scenario: Limiar é ajustável

- **WHEN** o usuário fornece um limiar diferente do padrão
- **THEN** o conjunto de candidatos sobreviventes muda de acordo

### Requirement: Discover reporta sem veredito nesta fase

O comando `discover` SHALL reportar os candidatos gerados e pontuados com o veredito de não avaliado, e SHALL declarar na saída que nenhuma validação contra dados foi executada.

Um candidato bem pontuado ainda é uma hipótese. Apresentá-lo sem essa ressalva convidaria o usuário a criar uma constraint a partir de um casamento de nome, que é exatamente o erro que a ferramenta existe para evitar.

#### Scenario: Nenhum candidato sai confirmado

- **WHEN** `discover` é executado nesta fase
- **THEN** todo candidato reportado carrega o veredito de não avaliado

#### Scenario: A limitação é declarada

- **WHEN** `discover` produz saída
- **THEN** a saída afirma explicitamente que os dados não foram consultados

### Requirement: Contagem de candidatos entra na cobertura

O número de candidatos gerados e o número de sobreviventes ao corte SHALL constar do bloco de cobertura.

É o número que permite calibrar o limiar antes que exista qualquer anti-join, e o que revela se uma execução futura de validação vai custar caro.

#### Scenario: Cobertura contabiliza a inferência

- **WHEN** `discover` termina
- **THEN** a cobertura informa quantos candidatos foram gerados e quantos sobreviveram ao limiar

### Requirement: O score é recomponível por um único dono da regra de saturação

O sistema SHALL expor a recomposição de score a partir dos sinais de um candidato, e toda camada que acrescenta sinais após a geração MUST recompor o score por esse caminho único, nunca por implementação própria.

Duas implementações da saturação divergiriam mais cedo ou mais tarde, e o limiar passaria a significar coisas diferentes dependendo de qual camada tocou o candidato por último.

#### Scenario: Sinal acrescentado recompõe pelo mesmo mecanismo

- **WHEN** uma camada posterior à geração acrescenta um sinal a um candidato e recompõe o score
- **THEN** o resultado é idêntico ao que a geração produziria com o mesmo conjunto de sinais

#### Scenario: Score recomposto continua saturado

- **WHEN** sinais negativos acumulados levariam o score abaixo de zero
- **THEN** o score recomposto é zero, nunca negativo

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

