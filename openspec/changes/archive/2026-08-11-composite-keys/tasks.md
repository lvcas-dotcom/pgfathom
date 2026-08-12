## 1. A chave no modelo, com a suíte atual como rede

- [x] 1.1 Criar `model.KeyRef` com schema, tabela e lista ordenada de colunas, mais as formas de renderização que `ColumnRef` já oferece — referência de tabela, texto legível e chave de comparação estável
- [x] 1.2 Trocar `Candidate.Child` e `Candidate.Parent` para `KeyRef`, mantendo `ColumnRef` onde o assunto continua sendo uma coluna: estatística de planner e igualdade extraída de SQL
- [x] 1.3 Atravessar mecanicamente as camadas consumidoras com aridade 1, sem mudar comportamento: `infer`, `stats`, `validate`, `report`, `discovery`
- [x] 1.4 Rodar a suíte inteira: só o golden do JSON pode mudar, e só na forma de `child` e `parent`. Qualquer outro golden diferente é bug desta tarefa, não atualização
- [x] 1.5 Conferir que `schema_version` permanece `"1"` e que o teste de contrato do JSON foi atualizado junto, não depois

## 2. Geração para alvo de chave composta

- [x] 2.1 Remover o corte por aridade do índice de alvos; alvo sem chave primária continua pulado com o motivo de sempre
- [x] 2.2 Implementar as duas derivações de correspondência — espelho e prefixada — resolvendo cada coluna da chave contra as colunas da tabela filha pelas formas do perfil ativo
- [x] 2.3 Exigir casamento total e derivação uniforme em todas as posições; ordenar as colunas do candidato pela ordem da chave primária
- [x] 2.4 Posição com mais de uma correspondente pula o alvo com nota, sem desempate de nenhuma espécie
- [x] 2.10 Casamento espelho exige alvo único: assinatura de chave compartilhada por mais de uma tabela é pulada com nota, porque mais de um dos candidatos pode confirmar e no máximo um é real
- [x] 2.5 Casamento parcial vira observação com a fração que casou, e nunca candidato
- [x] 2.6 Elegibilidade da coluna filha vale para o conjunto: uma posição já coberta por FK declarada desqualifica o candidato inteiro
- [x] 2.9 Relaxar a elegibilidade: inelegível é a coluna que **é** a chave primária de coluna única, não a que participa de uma composta. Sem isso o relacionamento identificador — o motivo de a maioria das chaves compostas existir — continua invisível
- [x] 2.7 Compatibilidade de tipo avaliada posição a posição, com uma incompatibilidade derrubando o conjunto
- [x] 2.8 Auto-referência composta continua permitida, pelas mesmas razões da aridade 1

## 3. Pontuação da aridade

- [x] 3.1 Acrescentar o sinal de concordância por aridade, emitido uma vez, com peso sublinear e declarado como estimativa a revisar com o corpus
- [x] 3.2 Avaliar os sinais existentes sobre a chave inteira — tipo, índice, nulidade — e garantir emissão única de cada um
- [x] 3.3 O sinal de índice passa a exigir índice cujas colunas iniciais sejam a chave filha na ordem dela
- [x] 3.4 Teste de que uma chave de quatro colunas não produz vinte sinais dizendo quatro fatos, e de que o score continua explicável pela lista

## 4. Pré-filtro sobre tupla

- [x] 4.1 Estimar a cardinalidade de tupla pelo limite inferior `máx distinto(coluna)`, com o motivo da rejeição nomeando a coluna que produziu o limite
- [x] 4.2 Checar faixa posição a posição, emitindo o sinal no máximo uma vez por candidato
- [x] 4.3 Ausência de estatística em todas as posições mantém o marcador de indisponível, sem penalidade e sem rejeição
- [x] 4.4 Estender a varredura que prova que nenhum valor de `pg_stats` sobrevive à camada para o caminho composto

## 5. Validação por tupla

- [x] 5.1 Gerar a agregação com lista de colunas citadas uma a uma, sem concatenação de valores em nenhum ponto
- [x] 5.2 Filtro de nulidade exigindo todas as colunas não nulas, com a contagem separada das linhas isentas por nulidade parcial
- [x] 5.3 Acrescentar a contagem de isentas ao modelo de validação, zero por construção em aridade 1
- [x] 5.4 Conferir que amostragem, teto de tempo, concorrência e classificação de erro seguem idênticos — a aridade não é motivo para tocar em nenhum deles
- [x] 5.5 Teste de que a DDL emitida para um composto confirmado cria e valida sem erro sobre as mesmas linhas que a validação examinou

## 6. Evidência de uso composta

- [x] 6.1 Agrupar as igualdades por objeto de origem e par de tabelas antes de ancorar
- [x] 6.2 Grupo cujo lado pai é exatamente a chave primária composta produz um candidato composto, na ordem da chave
- [x] 6.3 Grupo que cobre parte da chave não ancora, e não cai de volta em candidatos de coluna única para aquele par
- [x] 6.4 Um sinal por origem no candidato composto, mantendo a regra de que três views provando a mesma junção são um fato

## 7. Saídas

- [x] 7.1 Unificar o gerador de anti-join por tupla: a query de órfãos do `discover` e a de violação do `audit` passam a ter um dono só
- [x] 7.2 DDL com as duas listas na ordem da chave, sem cláusula `MATCH`
- [x] 7.3 Nome de constraint incluindo as colunas na ordem da chave, com o truncamento existente virando caminho rotineiro
- [x] 7.4 Sugestão de índice sobre as colunas da chave na ordem dela, com índice existente em ordem trocada não contando como cobertura
- [x] 7.5 Comentário de órfãos declarando as linhas isentas por `MATCH SIMPLE` quando houver
- [x] 7.6 Terminal renderizando lista de colunas sem quebrar o alinhamento das colunas da tabela
- [x] 7.7 Golden files novos para os cenários compostos, e conferência de que os antigos permanecem intactos exceto dois: o do JSON, pela forma da chave, e o de quebradas, porque a query de órfãos perdeu o alias `orphan_value` ao virar uniforme entre aridades

## 8. Cobertura e observações

- [x] 8.1 Remover `composite_primary_key` dos motivos de tabela não suportada, na leitura de catálogo e no modelo
- [x] 8.2 Conferir que as tabelas de chave composta passam a contar como analisadas, e que a proporção de cobertura sobe de acordo
- [x] 8.3 Observação de casamento parcial e de ambiguidade de posição entrando nos achados, com objeto e detalhe carregando só nome de catálogo

## 9. Fixtures e verificação

- [x] 9.1 Fixture de chave composta cobrindo: casamento espelho, casamento prefixado, derivação misturada que não deve casar, parcial de duas em três, ambiguidade de posição, órfãos de tupla plantados e linhas de nulidade parcial
- [x] 9.2 Registrar os valores plantados na lista única de `testutil`, no mesmo commit da fixture
- [x] 9.3 Cenário-armadilha composto: duas colunas de nome comum que existem juntas em várias tabelas e não formam relação — nenhuma delas pode sair confirmada
- [x] 9.4 Varredura de vazamento sobre a saída completa dos cenários compostos, nos três formatos
- [x] 9.5 Teste de integração aplicando o SQL gerado dos cenários compostos a um servidor real, sem edição manual
- [x] 9.6 Determinismo: duas execuções sobre a mesma fixture produzem JSON byte a byte idêntico, descontado o timestamp
- [x] 9.7 `go test ./...` sem Docker e sem rede, suíte de integração pacote a pacote, `golangci-lint run` também com `--build-tags integration`
- [x] 9.8 Atualizar o recorte de cobertura do README que hoje diz que um quarto das tabelas está fora de alcance
- [x] 9.9 Rodar `openspec validate composite-keys --strict` e corrigir o que apontar
