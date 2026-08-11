# candidate-generation Specification

## Purpose
TBD - created by archiving change fk-candidate-inference. Update Purpose after archive.
## Requirements
### Requirement: Colunas elegíveis a candidato

O sistema SHALL considerar como coluna filha toda coluna que não seja, sozinha, a chave primária da própria tabela, e que não participe de chave estrangeira já declarada. Coluna que apenas **participa** de uma chave primária composta SHALL ser elegível.

Coluna que já tem FK declarada não precisa de inferência: a relação está no catálogo. Reprocessá-la só produziria ruído duplicado no relatório.

Chave primária de coluna única é surrogate e não aponta para lugar nenhum, então continua fora. Estender isso a cada membro de uma chave composta descartaria o relacionamento identificador — `item(nota_id, seq)` com chave nas duas colunas e referência em `nota_id` —, que é a razão de existir da maioria das chaves compostas em base legada. A regra antiga excluía justamente as colunas que esta change existe para alcançar.

#### Scenario: Coluna com FK declarada é ignorada

- **WHEN** uma coluna já participa de uma chave estrangeira declarada, validada ou não
- **THEN** nenhum candidato é gerado para ela

#### Scenario: Chave primária de coluna única é ignorada

- **WHEN** a tabela tem chave primária de uma coluna e a coluna examinada é ela
- **THEN** nenhum candidato é gerado para ela

#### Scenario: Membro de chave composta é elegível

- **WHEN** a tabela tem chave primária composta e a coluna examinada é uma de suas colunas
- **THEN** ela é considerada para geração de candidatos

#### Scenario: Coluna comum é elegível

- **WHEN** a coluna não é chave primária nem participa de FK declarada
- **THEN** ela é considerada para geração de candidatos

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

A geração SHALL aceitar, além do casamento de nome, uma lista de evidências de junção, e SHALL aplicar às hipóteses vindas dela as mesmas regras duras: âncora na chave primária do alvo, de qualquer aridade, compatibilidade de tipo, coluna filha sem FK declarada.

As duas origens produzem o mesmo tipo de candidato, indistinguível no restante do pipeline exceto pelos sinais que carrega.

#### Scenario: Origens convergem no mesmo candidato

- **WHEN** nome e junção apontam o mesmo par de colunas
- **THEN** existe um único candidato carregando os sinais das duas origens

#### Scenario: Determinismo se mantém

- **WHEN** a geração roda repetidas vezes com o mesmo modelo e a mesma evidência
- **THEN** a saída é idêntica, na mesma ordem

### Requirement: Alvo precisa ter chave primária, de uma ou mais colunas

Um candidato SHALL apontar para uma tabela que tenha chave primária. A aridade da chave MUST NOT ser critério de exclusão.

Tabela alvo sem chave primária SHALL continuar sendo pulada com registro do motivo, nunca ignorada em silêncio, porque a relação pode ser real e simplesmente não ter âncora.

Chave estrangeira apontando para constraint `UNIQUE` em vez da chave primária é legal em PostgreSQL e permanece fora de escopo. A âncora é a chave primária.

#### Scenario: Alvo com chave composta gera candidato

- **WHEN** o nome de entidade casa com uma tabela cuja chave primária tem duas colunas, e o lado filho oferece correspondente para as duas
- **THEN** um candidato composto é gerado, com as colunas na ordem da chave primária

#### Scenario: Alvo sem chave primária é pulado com nota

- **WHEN** o nome de entidade casa com uma tabela sem chave primária
- **THEN** nenhum candidato é gerado, e o motivo é registrado

### Requirement: Casamento de chave composta é total, uniforme e sem desempate

Para gerar candidato composto, o lado filho SHALL oferecer exatamente uma coluna correspondente a **cada** coluna da chave primária alvo, na mesma tabela, com tipo compatível em todas as posições.

A correspondência SHALL ser por uma de duas derivações: **espelho**, em que a coluna filha tem o nome da coluna da chave; e **prefixada**, em que uma forma do nome da entidade alvo, segundo o perfil ativo, precede o nome da coluna da chave. A derivação escolhida SHALL ser a mesma para todas as posições.

Casamento parcial MUST NOT gerar candidato, e SHALL ser registrado como observação com quantas posições casaram sobre o total. Posição que case com mais de uma coluna filha SHALL pular o alvo com nota, e MUST NOT ser desempatada por posição, por tipo, por ordem de declaração ou por qualquer outro critério.

Chave composta é onde o casamento por nome fica perigoso: `(empresa_id, filial_id)` existe em dezenas de tabelas de um ERP. Casamento total e uniforme é o que separa uma chave de uma coincidência de duas colunas comuns na mesma tabela; casamento parcial promovido a candidato proporia uma constraint que rejeita linha válida; desempate por palpite produziria relação inventada com aparência de evidência forte.

#### Scenario: Espelho casa

- **WHEN** a tabela filha tem colunas com os mesmos nomes das colunas da chave primária alvo, e tipos compatíveis
- **THEN** o candidato composto é gerado

#### Scenario: Prefixada casa

- **WHEN** cada coluna da chave primária alvo aparece na filha precedida por uma forma do nome da entidade alvo
- **THEN** o candidato composto é gerado

#### Scenario: Derivação misturada não casa

- **WHEN** uma posição casa por espelho e outra só casaria por prefixo
- **THEN** nenhum candidato é gerado

#### Scenario: Parcial vira observação, nunca candidato

- **WHEN** duas das três colunas da chave primária alvo têm correspondente na filha
- **THEN** nenhum candidato é gerado, e a observação registra que duas de três posições casaram

#### Scenario: Ambiguidade de posição pula o alvo

- **WHEN** uma coluna da chave primária alvo tem mais de uma correspondente possível na tabela filha
- **THEN** o alvo é pulado com nota, e nenhuma das correspondentes é escolhida

#### Scenario: Uma posição incompatível derruba o conjunto

- **WHEN** todas as posições casam por nome mas uma delas tem tipo incompatível
- **THEN** nenhum candidato é gerado

### Requirement: Casamento espelho exige alvo único

Quando a correspondência for por espelho — o filho carrega os nomes das próprias colunas da chave, e nada nele nomeia o alvo —, o candidato SHALL ser gerado apenas se **uma única** tabela do escopo carregar aquela assinatura de chave. Havendo mais de uma, todas SHALL ser puladas com nota que diga quantas são, e nenhuma MUST ser escolhida.

A assinatura de chave é a evidência inteira de um casamento espelho. Com várias tabelas respondendo por ela, cada candidato seria validado por conta própria, e mais de um pode alcançar contenção total — a mesma tupla existe nas duas. Confirmar os dois é confirmar um relacionamento que não existe, que é a única coisa que este projeto trata como inaceitável.

Casamento prefixado não cai nesta regra: o nome do alvo está no nome da coluna, então a ambiguidade ali é a comum, e segue penalizada em vez de descartada.

#### Scenario: Assinatura compartilhada não é adivinhada

- **WHEN** duas tabelas têm a mesma chave primária composta e uma terceira carrega essas colunas sem nomear nenhuma delas
- **THEN** nenhum candidato é gerado, e a nota registra quantas tabelas dividem a assinatura

#### Scenario: Ambiguidade prefixada continua penalizada

- **WHEN** duas tabelas respondem à mesma forma de nome e ambas casam por prefixo
- **THEN** os candidatos são gerados carregando o sinal de alvo ambíguo

