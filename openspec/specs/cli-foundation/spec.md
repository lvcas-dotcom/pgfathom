# cli-foundation Specification

## Purpose
A definir - criado ao arquivar alteração bootstrap-core-model. Atualize o Purpose após o arquivamento.
## Requirements
### Requirement: Binário único com subcomandos

O projeto SHALL produzir um binário chamado `pgfathom`, sem dependência de runtime externo e sem cgo, com subcomandos.

`pgfathom` sem argumento SHALL exibir a ajuda e sair com código zero. Um guia interativo MUST NOT tomar o lugar desse comportamento: quem usa o comando em script conta com ele, e um guia que se abre sozinho é surpresa em ambiente que não tem quem responda.

Os subcomandos que produzem resultado SHALL aceitar `--format` com os valores `table`, `json` e `sql`, e `--out` apontando um diretório de artefatos. `--format sql` SHALL exigir `--out`, e sua ausência SHALL ser erro de uso.

`--out` SHALL ser aceito junto de qualquer formato: quando presente, os artefatos são escritos e o relatório do formato escolhido continua saindo. Formato desconhecido SHALL ser erro de uso, nunca degradação silenciosa para um padrão.

SQL não sai em stdout porque um formato que cabe num pipe convida ao pipe, e o cabeçalho de revisão obrigatória vira decoração que ninguém lê porque ninguém abriu o arquivo.

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
- **THEN** a tabela sai em stdout e os artefatos são escritos no diretório

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

### Requirement: A execução relata progresso a quem a chama

A unidade de execução SHALL relatar, para quem a chamou, o estágio que começou. Na validação, SHALL relatar também quantos candidatos de quantos já terminaram.

O relato SHALL ser uma função recebida por opção, e a unidade MUST NOT escrever progresso em stream algum por conta própria. Quem executa a mesma unidade para medir — o harness de benchmark — precisa contar sem que nada seja desenhado.

Estágio sem denominador conhecido antes de terminar SHALL relatar apenas que começou. Progresso com denominador inventado afirma saber quanto falta.

#### Scenario: Cada estágio anuncia o início

- **WHEN** uma execução atravessa os estágios
- **THEN** quem chamou recebe um relato por estágio, na ordem de execução

#### Scenario: A validação relata contagem

- **WHEN** a validação atravessa os candidatos
- **THEN** os relatos trazem quantos terminaram e quantos são no total

#### Scenario: Nada é escrito pela unidade

- **WHEN** a unidade é executada sem função de progresso
- **THEN** nenhum byte é escrito em stream algum por causa de progresso

### Requirement: O progresso é diagnóstico, e só aparece em terminal interativo

O comando SHALL exibir o progresso em stderr, nunca em stdout.

A exibição SHALL ocorrer somente quando stderr for terminal interativo, e SHALL ser suprimida quando `NO_COLOR` estiver definido ou o terminal for `dumb`. A decisão SHALL ser tomada uma vez, na fronteira do processo, e passada adiante como valor.

Progresso em stdout corromperia o consumo programático. Progresso num destino que não é terminal encheria log e arquivo de linhas que existiam para ser sobrescritas.

`NO_COLOR` desliga o progresso apesar de a linha não ser cor: quem define a variável está pedindo um stream sem sequências de escape, e reescrever linha é sequência de escape.

#### Scenario: Pipe não recebe progresso

- **WHEN** stderr é redirecionado para arquivo ou canalizado
- **THEN** nenhuma linha de progresso é escrita

#### Scenario: `NO_COLOR` suprime

- **WHEN** `NO_COLOR` está definido e stderr é terminal
- **THEN** nenhuma linha de progresso é escrita

#### Scenario: Aviso não é atropelado

- **WHEN** um estágio emite aviso durante a execução com progresso visível
- **THEN** o aviso aparece em linha própria, sem resto da linha de progresso

