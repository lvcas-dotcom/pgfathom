# benchmark-harness Specification

## Purpose
A definir - criado ao arquivar alteração benchmark-corpus. Atualize o Purpose após o arquivamento.
## Requirements
### Requirement: A medição executa o mesmo caminho que o usuário executa

O harness SHALL invocar a unidade de execução de descoberta em processo, com as mesmas opções que o comando montaria, e MUST NOT reimplementar a orquestração nem reparsear a saída do binário.

Um número publicado sobre a ferramenta só significa alguma coisa se medir o que se entrega. Uma medição que atravessa o binário não enxerga o funil por etapa, e uma que reimplementa a sequência mede a reimplementação.

#### Scenario: Medição em processo

- **WHEN** o harness mede um schema do corpus
- **THEN** ele chama a unidade de execução diretamente e lê o resultado dela, sem inspecionar texto de saída

#### Scenario: A configuração medida é declarada

- **WHEN** um resultado é publicado
- **THEN** ele nomeia a versão da ferramenta, a versão do servidor, o perfil ativo e quais estágios estavam ligados

### Requirement: O corpus é reproduzível por terceiros sem entrar no repositório

O manifesto do corpus SHALL ser versionado e SHALL declarar, para cada schema: nome, tipo de aquisição, origem, commit e checksum `sha256`. O dump em si MUST NOT ser versionado; ele SHALL ser baixado para diretório ignorado e SHALL ser conferido contra o checksum antes de qualquer uso.

Checksum divergente SHALL abortar com o valor esperado e o obtido, e MUST NOT ser tratado como aviso.

O tipo de aquisição SHALL existir desde a primeira entrada, para que um schema que exija subir a aplicação entre como linha nova em vez de reescrita do harness.

A busca e a medição SHALL ser operações separadas, de modo que medir não dependa de rede.

#### Scenario: Checksum divergente aborta

- **WHEN** o arquivo baixado não corresponde ao checksum do manifesto
- **THEN** a operação falha nomeando o esperado e o obtido, e nenhuma medição é executada

#### Scenario: Medir não usa rede

- **WHEN** o corpus já está no diretório de cache e a medição é executada
- **THEN** nenhuma requisição de rede é feita

#### Scenario: Corpus ausente diz o que fazer

- **WHEN** a medição é executada sem o corpus baixado
- **THEN** ela falha nomeando o comando que resolve, em vez de medir menos schemas em silêncio

### Requirement: O gabarito é o conjunto de chaves derrubado, e quem derruba é o harness

Para cada schema, o harness SHALL ler as chaves estrangeiras declaradas antes de qualquer modificação, SHALL removê-las todas, e SHALL usar esse conjunto como gabarito.

A remoção SHALL ser executada pela conexão do próprio harness, contra um servidor descartável que ele mesmo subiu. Nenhum código de `pgfathom` MUST emitir qualquer statement que altere o banco, e a separação entre as duas conexões SHALL ser explícita no código.

#### Scenario: A ferramenta continua sem escrever

- **WHEN** um schema do corpus é medido
- **THEN** toda DDL executada partiu do harness, e a conexão entregue à execução de descoberta mantém as políticas de sessão de sempre

#### Scenario: Gabarito lido antes da remoção

- **WHEN** o harness prepara um schema
- **THEN** as chaves declaradas são lidas do catálogo antes de serem removidas, e o conjunto lido é o gabarito

### Requirement: Recuperação é casamento exato de chave

Um candidato SHALL contar como recuperado quando a chave filha e a chave pai corresponderem a uma chave do gabarito, coluna a coluna, na ordem, comparadas sem distinção de caixa. Casamento parcial MUST NOT receber crédito.

#### Scenario: Chave composta recuperada na ordem

- **WHEN** o candidato aponta as mesmas colunas do gabarito, na mesma ordem
- **THEN** ele conta como recuperado

#### Scenario: Ordem trocada não conta

- **WHEN** o candidato aponta as mesmas colunas do gabarito em outra ordem
- **THEN** ele não conta como recuperado

#### Scenario: Parte da chave não conta

- **WHEN** o candidato cobre parte das posições de uma chave do gabarito
- **THEN** ele não conta como recuperado

### Requirement: Candidato fora do gabarito não é reportado como erro

Candidatos que não correspondem a nenhuma chave do gabarito SHALL ser contados e publicados como propostas fora do gabarito. Eles MUST NOT ser chamados de falso positivo, nem entrar em qualquer métrica de precisão calculada contra o gabarito.

Num schema real, uma relação verdadeira que nunca foi declarada é o produto desta ferramenta. Contá-la como defeito seria publicar como erro exatamente aquilo que se está entregando.

#### Scenario: Não casado é publicado como o que é

- **WHEN** o relatório de um schema é gerado
- **THEN** os candidatos fora do gabarito aparecem com contagem própria, sem serem somados a nenhuma taxa de erro

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

### Requirement: O harness declara o que não mede

O relatório SHALL declarar que os schemas do corpus público não contêm dados e que, portanto, nenhum veredito é medido neles.

O relatório SHALL declarar quando uma entrada opcional do manifesto não estava disponível na máquina, em vez de omiti-la.

Um corpus sem linhas não pode confirmar nem quebrar nada. Publicar recall ao lado de um silêncio sobre veredito convidaria a ler uma coisa como se fosse a outra, que é a forma de engano que este projeto trata como bug.

#### Scenario: Ausência de dados é declarada

- **WHEN** o relatório de um schema sem linhas é gerado
- **THEN** ele diz que nenhum veredito foi medido ali e por quê

#### Scenario: Entrada indisponível é declarada

- **WHEN** uma entrada local do manifesto não existe na máquina
- **THEN** o relatório registra que ela não foi medida, e a medição das demais prossegue

### Requirement: A calibração vem depois da linha de base

Os valores declarados como estimativa — limiar de score, margem de rejeição do pré-filtro, limiares de veredito e pesos de aridade — SHALL ser medidos primeiro com os valores vigentes, e a linha de base SHALL ser registrada antes de qualquer ajuste.

Toda mudança de valor SHALL vir acompanhada do número do corpus que a motivou, com o antes e o depois.

Calibrar antes de registrar a linha de base transformaria o corpus num ajuste de curva sobre si mesmo.

#### Scenario: Linha de base registrada antes do ajuste

- **WHEN** um limiar é alterado em função do corpus
- **THEN** existe registro da medição anterior à alteração, e a justificativa nomeia o número que a motivou

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

