# candidate-generation Specification

## Purpose
TBD - created by archiving change fk-candidate-inference. Update Purpose after archive.
## Requirements
### Requirement: Colunas elegíveis a candidato

O sistema SHALL considerar como coluna filha toda coluna que não seja a chave primária da própria tabela e que não participe de chave estrangeira já declarada.

Coluna que já tem FK declarada não precisa de inferência: a relação está no catálogo. Reprocessá-la só produziria ruído duplicado no relatório.

#### Scenario: Coluna com FK declarada é ignorada

- **WHEN** uma coluna já participa de uma chave estrangeira declarada, validada ou não
- **THEN** nenhum candidato é gerado para ela

#### Scenario: Chave primária da própria tabela é ignorada

- **WHEN** a coluna é a chave primária da tabela em que está
- **THEN** nenhum candidato é gerado para ela

#### Scenario: Coluna comum é elegível

- **WHEN** a coluna não é chave primária nem participa de FK declarada
- **THEN** ela é considerada para geração de candidatos

### Requirement: Alvo precisa ter chave primária de coluna única

Um candidato SHALL apontar para uma tabela cuja chave primária tenha exatamente uma coluna.

Tabela alvo com chave composta SHALL ser pulada com registro do motivo, nunca ignorada em silêncio, porque a relação pode ser real e simplesmente estar fora do escopo desta versão.

#### Scenario: Alvo com chave composta é pulado com nota

- **WHEN** o nome de entidade casa com uma tabela cuja chave primária tem duas colunas
- **THEN** nenhum candidato é gerado, e o motivo é registrado

#### Scenario: Alvo sem chave primária é pulado com nota

- **WHEN** o nome de entidade casa com uma tabela sem chave primária
- **THEN** nenhum candidato é gerado, e o motivo é registrado

### Requirement: Compatibilidade de tipo filtra antes do casamento de nome

Um candidato SHALL exigir compatibilidade entre o tipo base da coluna filha e o tipo base da chave primária alvo.

As regras SHALL ser: tipo base idêntico é o caso ideal; inteiro de largura menor apontando para inteiro de largura maior é aceitável; `text` e `varchar` de qualquer tamanho são intercambiáveis entre si; `uuid` casa apenas com `uuid`; tipo numérico MUST NOT casar com tipo textual, em nenhuma circunstância.

#### Scenario: Tipos idênticos

- **WHEN** a coluna filha e a chave alvo são ambas `int8`
- **THEN** os tipos são compatíveis e o sinal de tipo idêntico é emitido

#### Scenario: Inteiro menor para inteiro maior

- **WHEN** a coluna filha é `int4` e a chave alvo é `int8`
- **THEN** os tipos são compatíveis, com o sinal de tipo apenas compatível

#### Scenario: Inteiro maior para inteiro menor é rejeitado

- **WHEN** a coluna filha é `int8` e a chave alvo é `int4`
- **THEN** os tipos são incompatíveis, porque a filha admite valores que a chave não pode conter

#### Scenario: Textuais são intercambiáveis

- **WHEN** a coluna filha é `varchar` e a chave alvo é `text`
- **THEN** os tipos são compatíveis

#### Scenario: UUID é estrito

- **WHEN** a coluna filha é `text` e a chave alvo é `uuid`
- **THEN** os tipos são incompatíveis

#### Scenario: Numérico nunca casa com textual

- **WHEN** a coluna filha é `int8` e a chave alvo é `text`
- **THEN** os tipos são incompatíveis

### Requirement: Casamento usa o conjunto de formas do perfil

O casamento entre o nome de entidade extraído da coluna e o nome da tabela alvo SHALL usar o conjunto de formas produzido pelo perfil de nomenclatura ativo, e SHALL registrar qual forma casou.

Casamento na forma original SHALL emitir sinal distinto de casamento obtido por normalização.

#### Scenario: Casamento exato

- **WHEN** a coluna `cliente_id` casa com a tabela `cliente`
- **THEN** o candidato carrega o sinal de nome exato

#### Scenario: Casamento por normalização

- **WHEN** a coluna `cliente_id` casa com a tabela `tb_clientes`
- **THEN** o candidato carrega o sinal de nome normalizado, não o de nome exato

### Requirement: Ambiguidade gera candidatos e é sinalizada

Quando um nome de entidade casa com mais de uma tabela alvo, o sistema SHALL gerar candidato para cada uma e SHALL marcar todos com o sinal de alvo ambíguo.

Escolher uma arbitrariamente esconderia a incerteza. Gerar todos e deixar a validação decidir é o comportamento honesto, e a penalidade de score evita que ambiguidade domine o relatório.

#### Scenario: Mesmo nome em schemas diferentes

- **WHEN** o nome de entidade casa com tabelas homônimas em dois schemas do escopo
- **THEN** um candidato é gerado para cada, ambos com o sinal de alvo ambíguo

#### Scenario: Alvo único não é penalizado

- **WHEN** o nome de entidade casa com exatamente uma tabela
- **THEN** o candidato carrega o sinal de alvo único

### Requirement: Par polimórfico é reconhecido, não apenas rejeitado

Quando uma coluna de referência tiver, na mesma tabela, uma coluna irmã cujo nome seja o mesmo prefixo seguido de sufixo de tipo — `documento_id` ao lado de `documento_tipo` — o sistema SHALL reconhecer o padrão polimórfico e SHALL registrá-lo.

A validação encontraria contenção baixa e rejeitaria, o que é o resultado correto pelo caminho errado: o usuário veria "coincidência de nome" onde na verdade existe uma relação que esta versão não modela. Reportar o padrão transforma um falso descarte numa observação útil.

#### Scenario: Par detectado

- **WHEN** a tabela tem as colunas `documento_id` e `documento_tipo`
- **THEN** o padrão polimórfico é registrado para esse par

#### Scenario: Coluna de referência comum não é confundida

- **WHEN** a tabela tem `cliente_id` e nenhuma coluna irmã de tipo
- **THEN** nenhum padrão polimórfico é registrado

### Requirement: Auto-referência é permitida

O sistema SHALL gerar candidato quando a coluna filha e a tabela alvo pertencem à mesma tabela, como em `funcionario.gerente_id` apontando para `funcionario.id`.

Hierarquia por auto-referência é comum em banco legado, e excluí-la perderia uma classe inteira de relacionamento real.

#### Scenario: Hierarquia na mesma tabela

- **WHEN** a tabela `funcionario` tem a coluna `funcionario_id` e chave primária `id`
- **THEN** um candidato de `funcionario.funcionario_id` para `funcionario.id` é gerado

### Requirement: Geração é determinística

Para o mesmo modelo de entrada e o mesmo perfil, a saída SHALL ser idêntica em conteúdo e em ordem, execução após execução.

Ordenação instável quebraria os golden files das fases seguintes e tornaria qualquer diff de relatório ilegível.

#### Scenario: Ordem estável entre execuções

- **WHEN** a geração roda repetidas vezes sobre o mesmo modelo
- **THEN** a sequência de candidatos é idêntica em toda execução

### Requirement: A geração aceita evidência de junção como segunda origem

A geração SHALL aceitar, além do casamento de nome, uma lista de evidências de junção, e SHALL aplicar às hipóteses vindas dela as mesmas regras duras: alvo com chave única de coluna única, compatibilidade de tipo, coluna filha sem FK declarada.

As duas origens produzem o mesmo tipo de candidato, indistinguível no restante do pipeline exceto pelos sinais que carrega.

#### Scenario: Origens convergem no mesmo candidato

- **WHEN** nome e junção apontam o mesmo par de colunas
- **THEN** existe um único candidato carregando os sinais das duas origens

#### Scenario: Determinismo se mantém

- **WHEN** a geração roda repetidas vezes com o mesmo modelo e a mesma evidência
- **THEN** a saída é idêntica, na mesma ordem

