## ADDED Requirements

### Requirement: Todo arquivo gerado abre com cabeçalho de revisão obrigatória

Todo arquivo `.sql` emitido SHALL começar por um cabeçalho em comentário contendo a versão da ferramenta, o timestamp da geração, a versão do servidor analisado, o modo de validação executado, e a declaração explícita de que nada ali deve ser executado sem leitura.

Quando a execução tiver sido amostrada, o cabeçalho SHALL declarar que as contagens são piso e que nenhuma relação do arquivo foi confirmada.

Nenhum arquivo MUST ser gerado sem esse cabeçalho.

#### Scenario: O cabeçalho aparece em todo arquivo

- **WHEN** os artefatos são gerados
- **THEN** cada arquivo escrito começa pelo cabeçalho com os cinco campos e o aviso de revisão

#### Scenario: Modo amostrado é declarado no arquivo

- **WHEN** os artefatos são gerados a partir de uma execução amostrada
- **THEN** o cabeçalho declara que as contagens são piso e que nada foi confirmado

### Requirement: Um arquivo por categoria, sempre escrito

Os artefatos SHALL ser um arquivo por categoria de achado. Um arquivo SHALL ser escrito mesmo quando sua categoria estiver vazia, contendo o cabeçalho e a declaração de que a execução não produziu achados daquela categoria — e, quando aplicável, o motivo pelo qual não poderia tê-los produzido.

A ausência de um arquivo é ambígua entre "não achou nada", "a categoria não existe nesta versão" e "a escrita falhou". O arquivo vazio é uma afirmação; a ausência dele não é nada.

#### Scenario: Categoria vazia gera arquivo com a afirmação

- **WHEN** uma execução não produz nenhuma relação confirmada
- **THEN** o arquivo da categoria é escrito, com cabeçalho e a declaração de que nada foi confirmado

#### Scenario: Amostragem explica a categoria vazia

- **WHEN** uma execução amostrada gera o arquivo da categoria confirmada
- **THEN** o arquivo declara que o modo amostrado não pode confirmar, em vez de apenas informar contagem zero

### Requirement: Relação confirmada gera DDL executável em duas etapas

Para cada relação confirmada, o arquivo SHALL emitir `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY ... REFERENCES ... NOT VALID` executável, e SHALL emitir o `VALIDATE CONSTRAINT` correspondente **comentado**, em bloco separado, com a razão da separação escrita ao lado.

`NOT VALID` não varre linha alguma e segura o lock forte por um instante. O `VALIDATE` posterior varre com lock que não bloqueia leitura nem escrita. Emitir os dois juntos troca isso por uma varredura completa segurando o lock forte, que em tabela grande de produção é uma janela de manutenção.

#### Scenario: As duas etapas saem separadas

- **WHEN** uma relação confirmada gera DDL
- **THEN** o `ADD CONSTRAINT ... NOT VALID` sai executável e o `VALIDATE CONSTRAINT` sai comentado em bloco próprio

#### Scenario: Executar o arquivo inteiro roda só a parte barata

- **WHEN** o arquivo da categoria confirmada é executado por inteiro
- **THEN** as constraints são criadas como `NOT VALID` e nenhuma varredura de validação é disparada

### Requirement: Coluna filha sem índice gera `CREATE INDEX CONCURRENTLY` comentado

Quando a coluna filha de uma relação confirmada não tiver índice com ela em posição inicial, o arquivo SHALL emitir o `CREATE INDEX CONCURRENTLY` correspondente, **comentado**, precedido do aviso de que o comando não roda dentro de bloco de transação e de que uma falha deixa índice inválido para trás.

FK sem índice do lado filho transforma todo `DELETE` no pai em varredura sequencial da filha. Mas `CONCURRENTLY` falha sob `--single-transaction`, que é exatamente como um arquivo gerado tende a ser executado.

#### Scenario: Coluna sem índice recebe a sugestão com as duas armadilhas escritas

- **WHEN** uma relação confirmada tem coluna filha sem índice em posição inicial
- **THEN** o `CREATE INDEX CONCURRENTLY` sai comentado com o aviso de bloco de transação e o de índice inválido

#### Scenario: Coluna já indexada não recebe sugestão

- **WHEN** a coluna filha já tem índice com ela em posição inicial, ainda que composto
- **THEN** nenhum `CREATE INDEX` é emitido para ela

### Requirement: Relação quebrada emite a query de órfãos antes da DDL comentada

Para cada relação quebrada, o arquivo SHALL emitir primeiro a query que lista as linhas órfãs, e só depois a DDL correspondente, **comentada**, com o aviso de que ela não passa antes da limpeza.

A contagem de órfãos apurada SHALL aparecer em comentário junto da relação, declarada como piso quando a validação tiver sido amostrada.

Nenhum comando de remoção ou de correção de linha MUST ser emitido. O que fazer com linha órfã é decisão de domínio, e a ferramenta não tem o domínio.

#### Scenario: A ordem é olhar antes de alterar

- **WHEN** uma relação quebrada gera artefato
- **THEN** a query que lista os órfãos aparece antes da DDL, e a DDL está comentada

#### Scenario: Nenhum `DELETE` é sugerido

- **WHEN** o arquivo da categoria quebrada é inspecionado
- **THEN** ele não contém nenhum comando de remoção ou de atualização de linha, comentado ou não

### Requirement: Veredito sem conclusão não gera DDL

Candidatos com veredito fraco, rejeitado ou não validado MUST NOT gerar DDL, nem executável nem comentada.

Emitir DDL comentada para um veredito fraco sugere uma ação que a evidência não sustenta, e a ferramenta existe para não fazer isso.

#### Scenario: Fraco não vira sugestão

- **WHEN** uma execução produz candidatos fracos e não validados
- **THEN** nenhum artefato contém DDL referente a eles

### Requirement: `audit` gera a validação das constraints `NOT VALID`

Para cada chave estrangeira declarada `NOT VALID`, o artefato de `audit` SHALL emitir a query que verifica se a validação passaria, e o `VALIDATE CONSTRAINT` correspondente **comentado**.

Uma constraint `NOT VALID` aparece como FK normal em `\d` e em qualquer ferramenta de ERD, e não garante nada sobre as linhas que já estavam lá. Rodar o `VALIDATE` às cegas numa tabela com violação histórica falha depois de uma varredura completa.

#### Scenario: A verificação precede a validação

- **WHEN** o artefato de `audit` é gerado para uma constraint `NOT VALID`
- **THEN** a query de verificação aparece antes do `VALIDATE CONSTRAINT`, que está comentado

### Requirement: Identificador é sempre citado e nome de constraint é determinístico

Todo identificador emitido SHALL ser citado pelo mesmo mecanismo de sanitização usado na camada de validação.

O nome de constraint gerado SHALL ser determinístico a partir dos nomes de tabela e coluna: a mesma entrada produz o mesmo nome em toda execução. Quando o nome exceder o limite de identificador do servidor, ele SHALL ser truncado com sufixo derivado do nome completo, e o nome integral SHALL aparecer em comentário acima.

O orçamento SHALL ser contado em bytes, porque é assim que o servidor o conta, e o ponto de corte SHALL cair em fronteira de rune. Contar em caracteres estoura o limite com nome acentuado; cortar no meio de uma sequência UTF-8 produz identificador inválido. Truncar sem sufixo faz dois nomes longos distintos colidirem e o segundo `ADD CONSTRAINT` falhar por duplicata.

#### Scenario: Nome exótico sobrevive

- **WHEN** o artefato é gerado para tabela ou coluna com maiúscula, espaço ou palavra reservada
- **THEN** o SQL emitido executa sem erro de sintaxe contra um servidor real

#### Scenario: Execução repetida produz arquivo idêntico

- **WHEN** a mesma entrada gera artefatos duas vezes
- **THEN** os nomes de constraint e o conteúdo dos arquivos são idênticos, exceto pelo timestamp do cabeçalho

#### Scenario: Nome longo é truncado sem colidir

- **WHEN** dois pares distintos produziriam nomes de constraint que excedem o limite e coincidem após o corte
- **THEN** os nomes emitidos diferem, e o nome integral de cada um aparece em comentário

### Requirement: O SQL gerado executa sem edição manual

O conteúdo executável dos artefatos SHALL executar sem edição manual contra um banco de teste. Isso SHALL ser verificado por teste de integração que aplica o arquivo gerado a um servidor real, e não apenas por asserção de texto.

É o modo de falha mais embaraçoso possível para esta camada e o único que teste unitário não pega.

#### Scenario: O artefato aplica sobre a fixture que o originou

- **WHEN** os artefatos gerados a partir de uma fixture são aplicados ao banco dessa fixture
- **THEN** os comandos executáveis completam sem erro e as constraints passam a existir como `NOT VALID`

### Requirement: Nenhum valor de dado do usuário aparece nos artefatos

Nenhum arquivo gerado MUST conter valor lido de tabela do usuário. A varredura automatizada de vazamento SHALL cobrir o conteúdo dos artefatos, além de stdout e stderr.

A query de órfãos que o artefato emite é texto a ser executado pelo usuário, e MUST NOT embutir nenhum valor apurado durante a validação.

#### Scenario: A varredura cobre os arquivos

- **WHEN** o binário gera artefatos para cada fixture e os arquivos são varridos pelos valores plantados
- **THEN** nenhuma ocorrência é encontrada

#### Scenario: A query de órfãos não carrega valor

- **WHEN** a query que lista órfãos é emitida
- **THEN** ela é expressa como anti-join entre as duas tabelas, sem nenhum literal originado dos dados
