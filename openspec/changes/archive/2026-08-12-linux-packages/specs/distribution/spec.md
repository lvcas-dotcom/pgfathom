## ADDED Requirements

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
