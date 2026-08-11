# cli-foundation Specification

## Purpose
A definir - criado ao arquivar alteração bootstrap-core-model. Atualize o Purpose após o arquivamento.
## Requirements
### Requirement: Binário único com subcomandos

O projeto SHALL produzir um binário chamado `pgfathom`, sem dependência de runtime externo e sem cgo, com subcomandos.

`pgfathom` sem argumento SHALL exibir a ajuda e sair com código zero.

Os subcomandos que produzem resultado SHALL aceitar `--format` com os valores `table`, `json` e `sql`, e `--out` apontando um diretório de artefatos. `--format sql` SHALL exigir `--out`, e sua ausência SHALL ser erro de uso.

`--out` SHALL ser aceito junto de qualquer formato: quando presente, os artefatos são escritos e o relatório do formato escolhido continua saindo. Formato desconhecido SHALL ser erro de uso, nunca degradação silenciosa para um padrão.

SQL não sai em stdout porque um formato que cabe num pipe convida ao pipe, e o cabeçalho de revisão obrigatória vira decoração que ninguém lê porque ninguém abriu o arquivo.

Os subcomandos que leem catálogo SHALL aceitar `--schema` com a lista explícita de schemas, `--all-schemas` para resolver o escopo a partir do catálogo, e `--exclude-schema` com padrões glob de schemas a remover do escopo.

`--schema` e `--all-schemas` SHALL ser mutuamente exclusivas, e fornecê-las juntas SHALL ser erro de uso. Nenhuma precedência entre elas é aceitável: qualquer que fosse, produziria uma linha de comando cujo escopo real não é o que ela aparenta pedir.

A detecção de que `--schema` foi fornecida SHALL usar o estado de alteração da flag, e MUST NOT ser inferida da comparação com o valor padrão — `--schema public --all-schemas` é exatamente o caso ambíguo que a exclusividade existe para recusar.

Escopo de schema vazio SHALL ser erro de uso.

#### Scenario: Versão

- **WHEN** `pgfathom version` é executado
- **THEN** a versão, o commit e a data de build são impressos em stdout e o código de saída é 0

#### Scenario: Sem argumento

- **WHEN** `pgfathom` é executado sem argumento
- **THEN** a ajuda é impressa e o código de saída é 0

#### Scenario: Subcomando desconhecido

- **WHEN** `pgfathom naoexiste` é executado
- **THEN** a mensagem de erro vai para stderr e o código de saída é 2

#### Scenario: `--format sql` sem `--out` é erro de uso

- **WHEN** um comando é executado com `--format sql` e sem `--out`
- **THEN** a mensagem explica que o formato exige um diretório e o código de saída é o de erro de uso

#### Scenario: `--out` acompanha os outros formatos

- **WHEN** um comando é executado com `--format table` e `--out`
- **THEN** os artefatos são escritos no diretório e o relatório em tabela sai normalmente em stdout

#### Scenario: Formato desconhecido não degrada

- **WHEN** um comando é executado com um valor de `--format` que não existe
- **THEN** a execução falha com erro de uso, sem cair para nenhum formato padrão

#### Scenario: `--schema` junto de `--all-schemas` é erro de uso

- **WHEN** um comando é executado com `--schema` e `--all-schemas` ao mesmo tempo
- **THEN** a execução falha com código 2 e a mensagem cita as duas flags como mutuamente exclusivas

#### Scenario: `--schema public --all-schemas` também falha

- **WHEN** um comando é executado com `--schema` recebendo exatamente o valor padrão e `--all-schemas` junto
- **THEN** a execução falha com código 2, porque a flag foi fornecida ainda que o valor coincida com o padrão

#### Scenario: Exclusão que esvazia o escopo é erro de uso

- **WHEN** `--exclude-schema` remove todos os schemas que entrariam no escopo
- **THEN** a execução falha com código 2 e a mensagem indica que a exclusão esvaziou o escopo

### Requirement: Separação entre resultado e diagnóstico

Resultado destinado a consumo — tabela, JSON, SQL — SHALL ser escrito em stdout. Diagnóstico, aviso, progresso e erro SHALL ser escritos em stderr.

Esta separação SHALL valer sem exceção, porque a saída da ferramenta será canalizada para arquivo e para pipeline de CI, e diagnóstico misturado em stdout corrompe o consumo programático.

Quando o resultado for entregue como arquivo em disco, stdout SHALL receber o manifesto do que foi escrito — um caminho por artefato, com a contagem de achados de cada categoria. O manifesto é o resultado naquele modo; a ausência dele deixaria stdout vazio numa execução bem-sucedida, o que é indistinguível de falha.

#### Scenario: Redirecionamento preserva o resultado

- **WHEN** um comando é executado com stdout redirecionado para arquivo
- **THEN** o arquivo contém apenas o resultado, sem nenhuma linha de progresso ou aviso

#### Scenario: Erro não polui stdout

- **WHEN** um comando falha
- **THEN** a mensagem de erro está em stderr e stdout está vazio ou contém apenas resultado parcial válido

#### Scenario: Artefato em disco reporta o manifesto

- **WHEN** um comando escreve artefatos num diretório de saída
- **THEN** stdout lista cada caminho escrito com a contagem da respectiva categoria, e os avisos permanecem em stderr

### Requirement: Formatação adapta-se ao destino

O sistema SHALL detectar se stdout é um terminal interativo. Quando não for, MUST NOT emitir sequências ANSI de cor ou de controle.

Detecção SHALL respeitar também as convenções de ambiente: `NO_COLOR` definido desliga cor, e uma flag explícita de cor sobrepõe a detecção automática.

#### Scenario: Saída em pipe é limpa

- **WHEN** a saída é canalizada para outro processo ou para arquivo
- **THEN** nenhuma sequência ANSI é emitida

#### Scenario: NO_COLOR é respeitado

- **WHEN** a variável de ambiente `NO_COLOR` está definida e stdout é um terminal
- **THEN** nenhuma sequência ANSI é emitida

### Requirement: Códigos de saída são estáveis e documentados

Os códigos de saída SHALL ser estáveis desde este release, porque serão consumidos por pipeline de CI a partir da fase de `check`.

O contrato SHALL ser: 0 para execução bem-sucedida, 1 para falha de execução — conexão, privilégio, erro interno —, 2 para uso incorreto da linha de comando, e 3 reservado para "achados presentes" nos comandos que precisarem sinalizar regressão sem caracterizar falha.

#### Scenario: Sucesso

- **WHEN** um comando completa sem erro
- **THEN** o código de saída é 0

#### Scenario: Uso incorreto

- **WHEN** uma flag desconhecida ou um valor inválido é fornecido
- **THEN** o código de saída é 2 e a mensagem indica o uso correto

### Requirement: Cancelamento é honrado

Todo comando SHALL propagar um `context.Context` cancelável a partir da raiz, e SHALL cancelá-lo em `SIGINT` e `SIGTERM`.

Interrupção MUST NOT deixar trabalho pendurado no servidor de banco. Esta requisição existe nesta change, antes de haver qualquer acesso a banco, precisamente para que nenhuma camada posterior seja escrita sem receber contexto.

#### Scenario: Interrupção encerra limpo

- **WHEN** o processo recebe `SIGINT` durante a execução
- **THEN** o contexto raiz é cancelado, o processo encerra sem panic, e o código de saída indica interrupção

### Requirement: Logging estruturado sem vazamento

O sistema SHALL usar `log/slog` para diagnóstico, com nível configurável e destino em stderr.

Nenhum registro de log SHALL conter valor de dado do usuário. Atributo de log SHALL carregar apenas nome de objeto de catálogo, contagem, proporção ou duração.

#### Scenario: Nível configurável

- **WHEN** o nível de log é elevado para depuração
- **THEN** registros adicionais aparecem em stderr, e stdout permanece inalterado

#### Scenario: Log não vaza dado

- **WHEN** qualquer registro de log é emitido em qualquer nível
- **THEN** nenhum valor originado de tabela do usuário aparece nele

### Requirement: A execução é invocável fora do comando

A sequência completa de uma execução de descoberta — catálogo, detecção de nomenclatura, evidência de uso, geração, pré-filtro, validação e montagem do resultado — SHALL viver numa unidade com entrada programática, e o comando de linha SHALL apenas validar flags, montar opções, chamá-la e renderizar o que ela devolver.

Cada estágio SHALL ser identificável, e a unidade SHALL devolver o tempo gasto em cada um, na ordem de execução. A medição SHALL vir de dentro, porque um cronômetro de fora só enxerga o total, e o total não diz onde o tempo foi.

Um número publicado sobre a ferramenta só é reproduzível por terceiros se medir o mesmo caminho que o usuário executa. Uma orquestração que existe apenas dentro de uma função de comando obriga quem mede a atravessar o binário ou a reimplementar a sequência, e nos dois casos o que se mede deixa de ser o que se entrega.

#### Scenario: Execução programática produz o mesmo resultado do comando

- **WHEN** a unidade de execução é chamada diretamente com as mesmas opções que o comando montaria
- **THEN** o resultado é o mesmo que o comando produz, sem passar por análise de flags nem por renderização

#### Scenario: Estágio que degrada avisa quem chamou

- **WHEN** um estágio cujo regime é degradar — evidência de uso, pré-filtro estatístico ou verificação de privilégio — falha
- **THEN** a unidade reporta o aviso a quem a chamou, em vez de escrever em um stream por conta própria, e a execução continua

#### Scenario: O custo por etapa volta com o resultado

- **WHEN** uma execução termina
- **THEN** o resultado carrega o tempo de cada estágio executado, identificado pelo nome do estágio e na ordem em que rodaram

#### Scenario: Estágio desligado não inventa medida

- **WHEN** um estágio é desligado por opção
- **THEN** ele não aparece entre os tempos, em vez de aparecer com duração zero

