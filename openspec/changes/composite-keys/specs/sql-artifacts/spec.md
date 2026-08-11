## MODIFIED Requirements

### Requirement: Relação confirmada gera DDL executável em duas etapas

Para cada relação confirmada, o arquivo SHALL emitir `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY ... REFERENCES ... NOT VALID` executável, e SHALL emitir o `VALIDATE CONSTRAINT` correspondente **comentado**, em bloco separado, com a razão da separação escrita ao lado.

Para chave de mais de uma coluna, as listas dos dois lados SHALL sair na ordem da chave e SHALL corresponder posição a posição. Uma lista fora de ordem produz constraint que liga as colunas erradas sem erro de sintaxe, o que é a forma mais cara de errar disponível nesta camada.

Nenhuma cláusula `MATCH` SHALL ser emitida. O padrão é `MATCH SIMPLE`, é o que a validação mediu, e escrever `MATCH FULL` mudaria a semântica em relação ao número que sustenta o achado.

`NOT VALID` não varre linha alguma e segura o lock forte por um instante. O `VALIDATE` posterior varre com lock que não bloqueia leitura nem escrita. Emitir os dois juntos troca isso por uma varredura completa segurando o lock forte, que em tabela grande de produção é uma janela de manutenção.

#### Scenario: As duas etapas saem separadas

- **WHEN** uma relação confirmada gera DDL
- **THEN** o `ADD CONSTRAINT ... NOT VALID` sai executável e o `VALIDATE CONSTRAINT` sai comentado em bloco próprio

#### Scenario: Executar o arquivo inteiro roda só a parte barata

- **WHEN** o arquivo da categoria confirmada é executado por inteiro
- **THEN** as constraints são criadas como `NOT VALID` e nenhuma varredura de validação é disparada

#### Scenario: Chave composta sai com as duas listas na ordem da chave

- **WHEN** uma relação composta confirmada gera DDL
- **THEN** a lista de colunas filhas e a de colunas do pai saem na ordem da chave primária, correspondendo posição a posição

### Requirement: Coluna filha sem índice gera `CREATE INDEX CONCURRENTLY` comentado

Quando o lado filho de uma relação confirmada não tiver índice com as colunas da chave em posição inicial e na ordem dela, o arquivo SHALL emitir o `CREATE INDEX CONCURRENTLY` correspondente, **comentado**, precedido do aviso de que o comando não roda dentro de bloco de transação e de que uma falha deixa índice inválido para trás.

FK sem índice do lado filho transforma todo `DELETE` no pai em varredura sequencial da filha. Mas `CONCURRENTLY` falha sob `--single-transaction`, que é exatamente como um arquivo gerado tende a ser executado.

#### Scenario: Coluna sem índice recebe a sugestão com as duas armadilhas escritas

- **WHEN** uma relação confirmada tem lado filho sem índice em posição inicial
- **THEN** o `CREATE INDEX CONCURRENTLY` sai comentado com o aviso de bloco de transação e o de índice inválido

#### Scenario: Coluna já indexada não recebe sugestão

- **WHEN** o lado filho já tem índice com a chave em posição inicial, ainda que o índice seja mais largo
- **THEN** nenhum `CREATE INDEX` é emitido para ela

#### Scenario: Índice existente em ordem trocada não conta

- **WHEN** existe índice sobre as mesmas colunas da chave composta, em outra ordem
- **THEN** a sugestão é emitida, porque a ordem é o que torna o índice utilizável para a busca da chave

### Requirement: Relação quebrada emite a query de órfãos antes da DDL comentada

Para cada relação quebrada, o arquivo SHALL emitir primeiro a query que lista as linhas órfãs, e só depois a DDL correspondente, **comentada**, com o aviso de que ela não passa antes da limpeza.

A query de órfãos SHALL exigir todas as colunas da chave não nulas e SHALL comparar todas as posições no anti-join, refletindo a mesma semântica `MATCH SIMPLE` da constraint que ela precede e do número que a validação apurou. Ela SHALL ser produzida pelo mesmo gerador que monta o anti-join das constraints `NOT VALID` do `audit`: é a mesma query, e duas implementações da mesma regra divergem.

A contagem de órfãos apurada SHALL aparecer em comentário junto da relação, declarada como piso quando a validação tiver sido amostrada. Quando houver linhas isentas por nulidade parcial, o comentário SHALL dizer quantas são e que a constraint não as alcança.

Nenhum comando de remoção ou de correção de linha MUST ser emitido. O que fazer com linha órfã é decisão de domínio, e a ferramenta não tem o domínio.

#### Scenario: A ordem é olhar antes de alterar

- **WHEN** uma relação quebrada gera artefato
- **THEN** a query que lista os órfãos aparece antes da DDL, e a DDL está comentada

#### Scenario: Nenhum `DELETE` é sugerido

- **WHEN** o arquivo da categoria quebrada é inspecionado
- **THEN** ele não contém nenhum comando de remoção ou de atualização de linha, comentado ou não

#### Scenario: A query de órfãos composta concorda com a constraint

- **WHEN** a query de órfãos de uma relação composta é executada e depois a DDL correspondente é aplicada às mesmas linhas menos as órfãs
- **THEN** a constraint é criada e validada sem erro

#### Scenario: Isenção por nulidade parcial é declarada

- **WHEN** uma relação composta quebrada tem linhas com NULL em parte da chave
- **THEN** o comentário informa quantas linhas a constraint não alcança e por quê

### Requirement: Identificador é sempre citado e nome de constraint é determinístico

Todo identificador emitido SHALL ser citado pelo mesmo mecanismo de sanitização usado na camada de validação.

O nome de constraint gerado SHALL ser determinístico a partir dos nomes de tabela e coluna: a mesma entrada produz o mesmo nome em toda execução. Para chave de mais de uma coluna, as colunas SHALL entrar no nome na ordem da chave, de modo que duas chaves com as mesmas colunas em ordens diferentes produzam nomes diferentes. Quando o nome exceder o limite de identificador do servidor, ele SHALL ser truncado com sufixo derivado do nome completo, e o nome integral SHALL aparecer em comentário acima.

O orçamento SHALL ser contado em bytes, porque é assim que o servidor o conta, e o ponto de corte SHALL cair em fronteira de rune. Contar em caracteres estoura o limite com nome acentuado; cortar no meio de uma sequência UTF-8 produz identificador inválido. Truncar sem sufixo faz dois nomes longos distintos colidirem e o segundo `ADD CONSTRAINT` falhar por duplicata.

Chave composta estoura o orçamento com facilidade, então o truncamento deixa de ser caminho excepcional e passa a ser rotina — o que torna o sufixo de hash e o aviso no arquivo parte do fluxo normal, não do de exceção.

#### Scenario: Nome exótico sobrevive

- **WHEN** o artefato é gerado para tabela ou coluna com maiúscula, espaço ou palavra reservada
- **THEN** o SQL emitido executa sem erro de sintaxe contra um servidor real

#### Scenario: Execução repetida produz arquivo idêntico

- **WHEN** a mesma entrada gera artefatos duas vezes
- **THEN** os nomes de constraint e o conteúdo dos arquivos são idênticos, exceto pelo timestamp do cabeçalho

#### Scenario: Nome longo é truncado sem colidir

- **WHEN** dois pares distintos produziriam nomes de constraint que excedem o limite e coincidem após o corte
- **THEN** os nomes emitidos diferem, e o nome integral de cada um aparece em comentário

#### Scenario: Ordem das colunas muda o nome

- **WHEN** duas relações compostas ligam as mesmas colunas em ordens diferentes
- **THEN** os nomes de constraint gerados são diferentes
