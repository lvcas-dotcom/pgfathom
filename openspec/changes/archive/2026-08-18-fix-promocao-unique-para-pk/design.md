## Por que três passos, não dois

Os dois vizinhos de `writePromoteUnique` no mesmo arquivo já resolvem "como
promover uma chave sem lock pesado" com dois passos: constrói o índice
`CONCURRENTLY`, promove. Funciona ali porque não existe nenhuma constraint
no caminho — o índice novo nasce livre.

O nosso caso começa de um lugar diferente: já existe uma `UNIQUE` constraint,
com um índice que já é dela. `USING INDEX` não aceita esse índice porque ele
já está "ocupado". A saída é construir um índice **novo**, autônomo, promovê-lo
(esse sim aceito, porque nasceu livre), e só then descartar a `UNIQUE`
antiga — que nesse ponto já é redundante com a PK nova. O terceiro passo
(`DROP CONSTRAINT`) é o que diferencia este caminho dos outros dois; não é
invenção nova, é a consequência direta de já existir uma constraint prévia.

## Nome do índice novo

Mesma convenção que `writeConfirmedPrimaryKey`/`writeSyntheticPrimaryKey` já
usam: `truncateIdent("ux_" + t.Name + "_" + strings.Join(columns, "_"))`.
Reaproveitar em vez de inventar uma segunda convenção — o arquivo já tem uma,
e ela já trata truncamento em nomes acima de `NAMEDATALEN`.

## Por que as três linhas ficam comentadas

`CREATE INDEX CONCURRENTLY` não roda dentro de bloco de transação, e o
arquivo `.sql` inteiro é pensado para ser revisado antes de rodar (README,
seção Safety: "Nothing generated is meant to be executed unreviewed"). Os
dois vizinhos já comentam suas três/duas linhas por esse motivo exato — a
única razão de `writePromoteUnique` hoje gerar SQL executável direto é que
ninguém tinha notado que esse caminho também precisa do mesmo cuidado, porque
o bug de `USING INDEX` mascarava o problema antes mesmo de chegar a essa
questão.

## Texto corrigido, não removido

Onde hoje diz "at no cost" / "scans nothing" / "without rewriting a row", a
correção não apaga a alegação — troca pela parte que continua verdadeira: o
catálogo já prova a unicidade e a nulidade, então **nenhuma sondagem de
dado** é necessária (isso nunca deixou de ser verdade). O que deixa de ser
dito é que a *execução* da DDL é gratuita — ela builda um índice novo, que
tem custo de I/O mesmo sendo `CONCURRENTLY` (sem lock longo, mas não sem
leitura da tabela inteira).

## Teste: por que rodar as três linhas separadas, nunca juntas

`CONCURRENTLY` não pode estar na mesma transação que outro comando — e o
driver (`pgx`) trata múltiplos statements separados por `;` numa única
chamada `Exec` como um bloco implícito em alguns modos. `TestSuggestedIndexesArtifactParsesUnderExplain`
já resolve isso rodando uma linha comentada por vez, com `conn.Exec`
individual; o teste desta change estende a mesma técnica para as três linhas
em sequência, na mesma conexão, terminando com a verificação de estado que já
existia (`pg_constraint` com `contype = 'p'`).

## Sem delta de spec

`openspec/specs/structural-audit/spec.md:96` já diz "custo baixo", nunca
"custo zero" — o requisito nunca prometeu o que a implementação prometia.
Não há requisito a adicionar ou modificar; é bug fix puro contra um spec que
já estava certo. Arquivar com `--skip-specs`.
