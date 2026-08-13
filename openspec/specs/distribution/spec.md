# distribution Specification

## Purpose
A definir - criado ao arquivar alteração release-distribution. Atualize o Purpose após o arquivamento.
## Requirements
### Requirement: O binário publicado sabe a própria procedência

Todo binário publicado SHALL responder com versão, commit e data de build quando perguntado, e MUST NOT responder `unknown` para nenhum dos três.

O carimbo é injetado em tempo de link por duas configurações independentes — a de desenvolvimento e a de release — que precisam concordar sobre os mesmos nomes de variável. A concordância SHALL ser verificada por construção real seguida de execução, não por revisão.

Um binário sem carimbo falha em silêncio e chega inteiro ao usuário: o relatório JSON sai sem `tool_version`, que é o campo pelo qual uma linha de base é identificada.

#### Scenario: Snapshot construído responde quem é

- **WHEN** um binário é construído pelo caminho de release e executado com o subcomando de versão
- **THEN** versão, commit e data aparecem preenchidos, e nenhum deles é `unknown`

#### Scenario: Renomear a variável de carimbo quebra a verificação

- **WHEN** uma das variáveis de carimbo é renomeada em apenas uma das configurações
- **THEN** a verificação falha antes de qualquer publicação

### Requirement: O release cobre as plataformas alvo e publica checksums

Um release SHALL conter binário para Linux, macOS e Windows, em `amd64` e `arm64`, construído sem cgo.

Um release SHALL publicar o checksum de cada artefato.

Quem autoriza rodar esta ferramenta contra produção confere o que baixou. Publicar sem checksum transfere para o usuário uma verificação que ele não tem como fazer.

#### Scenario: As quatro plataformas saem

- **WHEN** um release é produzido
- **THEN** existe artefato para cada combinação de sistema e arquitetura declarada, e nenhum deles exige runtime externo

#### Scenario: Checksums acompanham

- **WHEN** um release é produzido
- **THEN** o arquivo de checksums está entre os artefatos e cobre todos os demais

### Requirement: A imagem de contêiner conecta com TLS

A imagem publicada SHALL conter certificados raiz e SHALL executar como usuário sem privilégio.

A ferramenta abre conexão com o servidor do usuário, e a configuração recomendada para produção verifica o certificado. Numa imagem sem CA raiz isso falha com uma mensagem que parece defeito do servidor, e a primeira execução de quem escolheu o contêiner vira um relatório de bug.

#### Scenario: Verificação de certificado funciona dentro da imagem

- **WHEN** a ferramenta é executada a partir da imagem contra um servidor que exige verificação de certificado
- **THEN** a conexão é estabelecida, e nenhuma falha decorre de ausência de autoridade certificadora

#### Scenario: A imagem não roda como root

- **WHEN** a imagem publicada é inspecionada
- **THEN** o usuário de execução não é `root`

### Requirement: Publicar é precedido por verificação e é irreversível

O procedimento de release SHALL executar a suíte que não depende de contêiner e o lint antes de publicar qualquer artefato, e SHALL abortar sem publicar nada se algo falhar.

Tag apagada continua no cache de quem já baixou, e versão republicada com conteúdo diferente destrói a confiança que uma ferramenta de infraestrutura leva anos para obter.

#### Scenario: Suíte vermelha não publica

- **WHEN** a suíte ou o lint falham durante um release
- **THEN** nenhum artefato é publicado

### Requirement: Canal que depende de recurso externo é opcional e declarado

Canal de distribuição que exija repositório ou credencial fora deste repositório — o tap do Homebrew é o caso — SHALL ser opcional: sua ausência SHALL custar aquele canal, nunca o release.

O pré-requisito SHALL estar escrito no procedimento de release, antes do passo que o consome.

#### Scenario: Sem o token, o release continua completo

- **WHEN** um release é executado sem a credencial do canal externo
- **THEN** os demais artefatos são publicados normalmente, e a ausência daquele canal é visível em vez de silenciosa

### Requirement: O que o release não oferece é declarado

Ausências relevantes para quem avalia procedência — assinatura criptográfica de artefato e SBOM, hoje — SHALL estar escritas no procedimento de release.

Uma ausência declarada é uma decisão; uma ausência silenciosa é indistinguível de esquecimento, e este projeto trata a diferença como parte do produto.

#### Scenario: A ausência de assinatura está escrita

- **WHEN** o procedimento de release é lido
- **THEN** ele diz que os artefatos não são assinados e o que existe no lugar

### Requirement: O release cobre instalação por gerenciador de pacotes de distribuição

Um release SHALL publicar pacote `.deb` e `.rpm`, para `amd64` e `arm64`.

Cada pacote SHALL instalar o binário em `/usr/bin`, e SHALL incluir a licença e o README sob `/usr/share/doc/pgfathom/`.

Cada pacote SHALL incluir autocompletar para bash, zsh e fish, nos diretórios onde cada shell os procura.

O usuário mais provável desta ferramenta administra servidor Linux. Para ele, `go install` exige um toolchain, o cask do Homebrew não existe, e contêiner é fricção para experimentar. Um pacote é o caminho que ele já digita.

Pacote que inventa layout é um arquivo comprimido com metadados. Os caminhos SHALL ser os que a convenção de cada distribuição define, e o conteúdo do pacote SHALL ser verificado por listagem em vez de suposto.

#### Scenario: O pacote instala onde se espera

- **WHEN** o conteúdo de um pacote publicado é listado
- **THEN** o binário está em `/usr/bin`, a licença e o README sob `/usr/share/doc/pgfathom/`, e os arquivos de autocompletar nos diretórios de cada shell

#### Scenario: As duas famílias e as duas arquiteturas saem

- **WHEN** um release é produzido
- **THEN** existe `.deb` e `.rpm` para `amd64` e para `arm64`

### Requirement: O autocompletar é gerado do próprio comando e resolvido por ele

Os arquivos de autocompletar SHALL ser gerados a partir do comando durante o build, e MUST NOT ser mantidos à mão no repositório.

O script gerado SHALL delegar a resolução ao próprio binário em tempo de execução, em vez de embutir a lista de flags. É como o cobra os produz, e é mais forte do que embutir: o autocompletar de uma instalação descreve o binário daquela instalação, não o do momento em que o arquivo foi escrito.

O `discover` tem vinte e uma flags. Autocompletar mantido à mão divergiria no primeiro release que acrescentasse uma delas, e a divergência não teria como ser notada.

#### Scenario: A resolução vem do binário

- **WHEN** o script de autocompletar publicado é inspecionado
- **THEN** ele invoca o próprio comando para obter as opções, em vez de conter uma lista fixa

#### Scenario: Flag nova completa sem trabalho manual

- **WHEN** uma flag é acrescentada a um comando
- **THEN** o autocompletar a oferece sem que nenhum arquivo tenha sido editado

### Requirement: A ausência de repositório de pacotes é declarada

O procedimento de release SHALL registrar que a instalação é por arquivo baixado e que, portanto, não há atualização automática.

Repositório apt ou yum exigiria hospedagem e chave de assinatura sob custódia. É compromisso de outra ordem, e uma ausência declarada é uma decisão, enquanto uma ausência silenciosa é indistinguível de esquecimento.

#### Scenario: O procedimento diz que não atualiza sozinho

- **WHEN** o procedimento de release é lido
- **THEN** ele registra a ausência de repositório e o que isso implica para atualização

### Requirement: A nota de release é escrita, e sua ausência aborta o release

A nota de release SHALL vir de um changelog versionado no repositório, escrito para quem usa a ferramenta. Ela MUST NOT ser gerada a partir do histórico de commits.

O release SHALL falhar antes de publicar qualquer artefato quando o changelog não tiver seção para a versão sendo lançada.

#### Scenario: Versão sem seção não é publicada

- **WHEN** o workflow de release roda para uma tag que não tem seção correspondente no changelog
- **THEN** ele falha antes de publicar qualquer artefato, dizendo qual seção falta

#### Scenario: A nota publicada é a que foi escrita

- **WHEN** um release é publicado para uma versão que tem seção no changelog
- **THEN** o corpo da release é o conteúdo dessa seção, sem hashes de commit

