## Context

Esta camada é a única do programa que lê linhas de tabela do usuário, e a única cujo custo o dono do banco sente. As decisões abaixo existem para que o pior cenário — schema grande, tabela enorme, DBA desconfiado — degrade para "demorou e avisou", nunca para "derrubou" ou "mentiu".

Restrições herdadas que dominam o desenho: nenhum valor de dado sai da memória; amostra não prova ausência; candidato não resolvido nunca é silêncio. E a regra de ouro do projeto inteiro: na dúvida entre recuperar mais e nunca errar, nunca errar vence.

## Goals / Non-Goals

**Goals**

Responder, por candidato, o que os dados sustentam: confirmada, quebrada, fraca ou rejeitada, com as métricas que sustentam o veredito. Fazer isso a custo previsível e interrompível, num banco de produção de terceiros.

**Non-Goals**

Nenhuma emissão de DDL ou query de órfãos — os artefatos `.sql` são a fase 7. Nenhuma junção minerada de view ou função — fase 6. Nenhum veredito para chave composta ou par polimórfico, que continuam registrados como observação.

## Decisions

### Uma query por candidato, agregada por valor distinto antes do anti-join

A formulação ingênua — anti-join direto sobre as linhas — paga o custo da contagem de linhas da filha. Agregar por valor primeiro muda o anti-join para a cardinalidade da coluna, que em coluna de referência típica é ordens de magnitude menor, e entrega na mesma passada as duas contenções e o máximo de linhas por valor.

As duas contenções contam histórias diferentes: um único valor inválido repetido um milhão de vezes derruba a contenção por linha e mal arranha a por valor; um milhão de valores raros e inválidos faz o contrário. Divergência grande entre as duas é achado, não ruído — por isso ambas saem sempre.

### Amostrado por padrão, e declarado triagem

O modo padrão precisa ser executável num banco de centenas de milhões de linhas sem pedir permissão duas vezes. `TABLESAMPLE SYSTEM` calibrado para o alvo de linhas dá isso a custo proporcional ao alvo, não à tabela.

Mas órfãos não são aleatórios: entram em lote — uma carga malfeita, uma migração de fim de semana — e ficam fisicamente agrupados nas mesmas páginas, que é exatamente o que amostragem por página é pior em encontrar. O modo amostrado é portanto triagem para ordenar atenção, nunca evidência, e o relatório diz isso com todas as letras. Três consequências mecânicas: amostra nunca produz `confirmed`, sem exceção; órfão achado em amostra é real — a ausência é que não é provável — então `broken` continua válido e as contagens saem como piso, não como total; e o veredito limpo em amostra sai `weak` com o motivo apontando o `--full`.

`BERNOULLI` entra como fallback em tabela pequena, onde a amostragem por página distorce demais; e tabela que já cabe no alvo é lida direta, o que devolve o modo conclusivo de graça.

### Timeout por candidato, com transação somente-leitura

O `statement_timeout` da sessão protege tudo, mas a validação merece um teto próprio: é a única query cujo custo cresce com o tamanho da tabela do usuário. Cada validação roda numa transação com `SET LOCAL statement_timeout`, o que confina o ajuste àquela query sem tocar na política da sessão.

Timeout não é erro: o candidato sai `unvalidated` com o motivo, o contador de estourados sobe, e a execução segue. A alternativa — abortar a execução inteira porque uma tabela de 400 milhões de linhas não coube no teto — jogaria fora o trabalho das outras duzentas validações que couberam.

### Concorrência pelo teto da sessão

`errgroup.SetLimit` com o mesmo limite que dimensiona o pool. Não existe knob separado: o operador que autorizou N conexões autorizou N queries, e dois limites independentes criariam a combinação onde o pool bloqueia o grupo e o cancelamento fica esperando conexão.

### Vereditos com zona morta explícita

Contenção total nas duas dimensões e mais de um valor distinto: `confirmed`. Acima do limiar de quebra mas abaixo de total: `broken`, com órfãos por linha e por valor. Abaixo do limiar de rejeição: `rejected`. Entre os dois limiares, e nos casos em que a estatística não sustenta conclusão — um único valor distinto, nulos dominando a coluna, filha vazia — o veredito é `weak` com o motivo.

A zona morta é deliberada. Setenta por cento de contenção não é uma relação quebrada nem uma coincidência: é um caso que precisa de um humano, e fingir certeza para qualquer um dos lados é exatamente o tipo de erro que a ferramenta promete não cometer.

### Identificadores sempre citados

Nome de schema, tabela e coluna entra na query exclusivamente por citação de identificador (`pgx.Identifier`). A sessão read-only já impede escrita, mas identificador malformado quebrando query — ou pior, mudando o que ela mede — é bug de correção, não só de segurança.

## Risks / Trade-offs

**A query de validação é o maior custo que a ferramenta impõe** → limiar + pré-filtro encolhem o conjunto antes daqui; timeout por candidato limita o custo unitário; concorrência limita o agregado; e o modo padrão amostra. O pior caso vira lentidão anunciada.

**Amostra limpa induz falsa tranquilidade** → o veredito nunca passa de `weak` com apontamento explícito para `--full`, e o cabeçalho do relatório declara o modo. A armadilha está documentada na spec como comportamento, não como nota.

**`TABLESAMPLE` não aceita a fração como parâmetro em todo caminho de plano** → a fração é calculada de `reltuples` e interpolada como literal numérico formatado pelo próprio código, nunca de entrada do usuário; os identificadores continuam citados.

**Cancelamento no meio de dezenas de queries concorrentes** → o contexto do errgroup propaga para cada query, e a fase 2 já provou que cancelamento derruba a query no servidor. O grupo retorna no primeiro erro não-timeout; timeout não é erro de grupo.

## Migration Plan

Não aplicável. `Validation` e os vereditos já existem no modelo e no JSON desde a fase 1; esta fase começa a preenchê-los.

## Open Questions

Nenhuma bloqueante. Os limiares de quebra (0,90) e rejeição (0,50) e o alvo de amostra (100k linhas) são padrões declarados, revistos com o corpus da fase 8.
