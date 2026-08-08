## Contexto

Esta fase não descobre nada. Ela pega o `model.Result` que as fases 1 a 5 produzem e o transforma em três coisas que um humano ou uma máquina consegue usar. O risco aqui não é errar a inferência — é errar a **entrega**: gerar um SQL que não compila, vazar um valor de dado num campo de texto livre, ou fixar um formato de saída que quebra a cada build.

As decisões abaixo são todas sobre isso.

## Decisões

### `--format sql` exige `--out`, e não escreve em stdout

A alternativa óbvia seria emitir tudo em stdout separado por comentários de seção, o que permitiria `pgfathom discover --format sql | psql`. É justamente por permitir isso que foi descartada.

A especificação diz que nenhum arquivo gerado deve ser executável sem revisão humana. Um formato que cabe num pipe convida ao pipe, e o cabeçalho de revisão vira decoração que ninguém lê porque ninguém abriu o arquivo. Exigir um diretório força o artefato a existir como arquivo, que é o objeto que se abre num editor.

O custo é real: quem quer o SQL num script tem que ler o arquivo depois. Aceito. A fricção está do lado certo.

`--out` é ortogonal ao formato: pode ser passado junto com `--format table` ou `--format json`, e nesse caso os artefatos são escritos **e** o relatório sai normalmente. `--format sql` só muda o que vai para stdout — o manifesto dos arquivos escritos, com contagem por categoria, em vez do relatório.

### Todo arquivo é escrito, inclusive o vazio

Se não houve nenhuma relação confirmada, `confirmed.sql` ainda é criado, com cabeçalho e uma linha dizendo que esta execução não confirmou nenhuma relação — e, em modo amostrado, dizendo **por quê** não poderia ter confirmado.

A alternativa é não criar o arquivo, e ela falha na regra 4. Um diretório sem `confirmed.sql` é ambíguo entre "não achou nada", "a categoria não existe nesta versão" e "a escrita falhou". O arquivo vazio é uma afirmação; a ausência dele não é nada.

### DDL de confirmada sai executável, DDL de quebrada sai comentada

A assimetria é deliberada e é a única decisão do arquivo que codifica a regra 5 diretamente.

Uma relação `confirmed` tem contenção total verificada linha a linha em modo completo. O `ADD CONSTRAINT ... NOT VALID` correspondente é seguro por construção: ele não valida nada, então nem sequer depende da conclusão estar certa. Sai executável.

Uma relação `broken` tem órfãos. Qualquer DDL ali só passa depois de uma decisão humana sobre o que fazer com as linhas órfãs — apagar, corrigir, ou apontar para um registro sentinela — e essa decisão a ferramenta não tem informação para tomar. Sai comentada, precedida da query que lista os órfãos, porque a ordem correta de trabalho é olhar antes de alterar.

`weak`, `rejected` e `unvalidated` não geram DDL de espécie alguma. Um veredito fraco é literalmente "não sei"; emitir DDL comentada para ele seria sugerir uma ação que a evidência não sustenta.

### `NOT VALID` primeiro, `VALIDATE` depois e separado

`ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY ... NOT VALID` pega `SHARE ROW EXCLUSIVE` na tabela filha por um instante e não varre linha nenhuma. O `VALIDATE CONSTRAINT` posterior varre, mas com um lock mais fraco que não bloqueia leitura nem escrita.

Emitir os dois num statement só — que é o que `ADD CONSTRAINT` sem `NOT VALID` faz — troca isso por uma varredura completa segurando o lock forte. Em tabela grande de produção isso é uma janela de manutenção; em duas etapas é um comando instantâneo e uma varredura que pode rodar em horário escolhido.

Por isso o `VALIDATE` sai comentado num bloco próprio, com o motivo escrito ali. Quem cola o arquivo inteiro executa só a parte barata.

### `CREATE INDEX CONCURRENTLY` sai comentado e sozinho

Quando a coluna filha não tem índice com ela em posição inicial, o arquivo emite o `CREATE INDEX CONCURRENTLY` — FK sem índice do lado filho transforma todo `DELETE` no pai em varredura sequencial da filha.

Mas `CONCURRENTLY` **não roda dentro de bloco de transação**. Se o arquivo for executado com `psql --single-transaction`, ou colado dentro de um `BEGIN`, ele falha. E se falhar no meio, deixa um índice `INVALID` para trás que precisa ser derrubado à mão.

Duas armadilhas que o usuário não tem como adivinhar lendo a linha. O comando sai comentado, com as duas escritas acima dele.

### Nome de constraint é determinístico e cabe em 63 bytes

`fk_<tabela>_<coluna>`, minúsculo. Determinístico porque rodar a ferramenta duas vezes tem que dar o mesmo arquivo — golden file exige, e diff entre execuções é o que torna o `check --baseline` possível.

`NAMEDATALEN` corta identificador em 63 bytes silenciosamente, o que transformaria dois nomes longos distintos em um único nome e faria o segundo `ADD CONSTRAINT` falhar por duplicata. Quando o nome estoura, ele é truncado com um sufixo derivado do nome completo, e o nome original aparece num comentário acima.

O orçamento é contado em **bytes**, porque é assim que o servidor conta, mas o corte cai em **fronteira de rune**: contar em caracteres estoura o limite com nome acentuado, e cortar no meio de uma sequência UTF-8 produz identificador inválido. As duas coisas precisam ser verdade ao mesmo tempo.

Colisão residual entre nomes truncados diferentes continua possível em teoria. Não é tratada — a ferramenta não conhece os nomes de constraint que o usuário vai criar entre a geração e a execução, então nenhuma garantia aqui seria honesta. O arquivo é revisado antes de rodar; é lá que isso aparece.

### Identificador sempre citado, via `pgx.Identifier.Sanitize`

Mesmo mecanismo que `internal/validate` já usa para montar a query de validação. Não é sobre injeção — os nomes vêm do catálogo, não do usuário — é sobre nome com maiúscula, espaço, hífen ou palavra reservada, que a fixture de nome exótico já cobre e que quebraria SQL não citado.

### Golden file exige entrada determinística

Um relatório carrega versão da ferramenta, timestamp e versão do servidor. Fixar isso num golden file quebraria o teste em todo build.

A solução é não normalizar a saída, e sim controlar a entrada: os testes de golden constroem o `model.Result` com versão, `GeneratedAt` e duração fixos. O renderizador continua ingênuo e o golden fixa exatamente o que se quer fixar — a formatação — sem nenhuma máquina de sanitização entre o código e o teste.

Consequência aceita: quando a fase 6 acrescentar sinais de junção, alguns golden files mudam e precisam de `-update`. É o comportamento desejado de um golden file, não uma regressão.

### ANSI mínimo, sem biblioteca

O aviso de amostragem precisa ser "destacado" segundo a especificação. Realce aqui é negrito e uma cor, aplicados a três lugares: o aviso de amostragem, o cabeçalho do grupo de quebradas, e a linha de cobertura quando há tabela pulada.

Isso é meia dúzia de constantes de escape e uma função que devolve string vazia quando a cor está desligada. Uma dependência de coloração custaria uma linha no `go.mod` que o DBA lê antes de autorizar a execução, para resolver um problema de seis constantes.

A cor é decidida uma vez em `Streams.Color()`, que já existe e já respeita `NO_COLOR`, `TERM=dumb` e detecção de TTY. O renderizador recebe o booleano; não consulta ambiente.

### Descartado ganha campo próprio, e não é um candidato com veredito ruim

Descoberto ao reestruturar o agrupamento: hoje, com `--include-rejected`, os descartados são anexados a `Result.Candidates` **e** passados à view como lista separada. O renderizador atual desenha as duas listas, então o candidato descartado aparece duas vezes. Ninguém notou porque a lista atual é plana e a duplicata some no meio.

Agrupar por veredito faria isso virar erro visível, e a correção óbvia — anexar depois de montar a view — depende de o `append` não realocar, o que é aliasing acidental esperando para quebrar.

A correção certa é estrutural: `Result` ganha `discarded` como campo de topo, e `candidates` passa a conter só os sobreviventes. É o mesmo princípio que já separa relação declarada de inferida no modelo — um consumidor não deveria precisar inspecionar score para saber se um candidato passou pela triagem. Feito agora porque congelar o contrato com as duas coisas no mesmo array tornaria isso permanente.

### O JSON é congelado pela forma, não pelo conteúdo

O teste de contrato serializa um `Result` preenchido e compara o **conjunto de caminhos de chave** contra um golden. Comparar o JSON inteiro faria o teste quebrar por mudança de valor, que é ruído; comparar as chaves quebra exatamente quando o contrato muda, que é o sinal.

Campo novo também quebra o teste. É intencional: acrescentar campo é compatível, mas deve ser um ato consciente que passa por revisão, não algo que entra de carona numa struct.

`Result` ganha `duration_ns` nesta fase, antes do congelamento, porque o rodapé do terminal precisa do tempo total e porque acrescentar campo depois do primeiro release é mais caro que agora.

### Ordem de apresentação: quebradas primeiro

Contraria a ordem de confiança — o intuitivo seria confirmadas no topo. Mas confirmada é uma tarefa de higiene que pode esperar a próxima janela; quebrada é integridade violada em produção, provavelmente há anos. É o achado que justifica a ferramenta existir e é o que a pessoa precisa ver antes de fechar o terminal.

Depois de quebradas: confirmadas, fracas, e por fim não validadas. Rejeitadas só aparecem com `--include-rejected`, porque a lista de rejeitados é longa por construção e afogaria o resto.

## Riscos

**O SQL gerado não compilar.** É o modo de falha mais embaraçoso possível para esta fase e o único que teste unitário não pega. Mitigado por teste de integração que pega o arquivo gerado e o executa contra um Postgres real das fixtures — incluindo a de nome exótico e a de FK sem índice.

**Vazamento por campo de texto livre.** `Reason` e `Signal.Detail` são texto livre e agora aparecem em três saídas em vez de uma. A varredura de vazamento passa a cobrir também o conteúdo dos arquivos `.sql` gerados, não só stdout e stderr.

**Sobrescrita silenciosa em `--out`.** Os nomes de arquivo são fixos, então rodar duas vezes no mesmo diretório sobrescreve. Aceito e declarado: o manifesto em stdout lista cada caminho escrito, e a alternativa — sufixo por timestamp — produziria lixo acumulado e quebraria a comparação entre execuções que o `check --baseline` vai querer.

## Fora de escopo

**Formato de saída para ERD** — DBML, Mermaid, PlantUML estão no pós-v0.1 e não competem com estes três.

**`DROP CONSTRAINT` ou qualquer DDL de remoção.** A ferramenta só propõe acréscimo. Sugerir remoção exigiria concluir que algo está errado no schema declarado, que não é uma conclusão que ela alcança.

**Script de limpeza de órfãos.** A query que os lista sai; o `DELETE` não. Escolher o que fazer com linha órfã é decisão de domínio, e a ferramenta não tem o domínio.

**Cor configurável por tema.** Três realces não justificam superfície de configuração.
