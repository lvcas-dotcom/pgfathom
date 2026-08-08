## 1. Modelo

- [x] 1.1 Adicionar ao modelo o tipo de evidência de junção: par de colunas resolvido, origem (view, função, statements) e objeto de origem
- [x] 1.2 Pesos dos três sinais de junção em `internal/infer`, acima de qualquer sinal de nome, com teste de ordenação relativa

## 2. Tokenizador

- [x] 2.1 Criar `internal/sqlprobe` com o tokenizador: identificador nu (minúsculas) e citado (caixa e espaços preservados), pontuação relevante, número
- [x] 2.2 Atravessar comentário de linha e de bloco (aninhado), string com aspas simples e escape, string `E''`
- [x] 2.3 Atravessar dollar quoting com e sem tag, incluindo tag aninhada distinta
- [x] 2.4 Tabela de casos do tokenizador cobrindo cada forma e as combinações traiçoeiras

## 3. Extrator

- [x] 3.1 Construir o mapa de alias por statement a partir de `FROM` e `JOIN`, com e sem `AS`, com schema qualificado
- [x] 3.2 Reconhecer igualdades `referência = referência` em `ON` e `WHERE`, resolvendo ambos os lados
- [x] 3.3 Deduplicar evidências por par resolvido e origem
- [x] 3.4 Ignorar sem erro: SQL truncado, sintaxe desconhecida, referência não resolvida
- [x] 3.5 Tabela de casos do extrator: junção simples, múltiplas junções, alias conflitante, `WHERE` implícito, SQL malformado

## 4. Fontes

- [x] 4.1 Ler definições de view dos schemas em escopo
- [x] 4.2 Ler corpos de função `sql` e `plpgsql` dos schemas em escopo
- [x] 4.3 Ler `pg_stat_statements` quando disponível, marcando a disponibilidade na cobertura
- [x] 4.4 Teste que falha se alguma consulta da camada referenciar relação fora do catálogo e das visões de estatística

## 5. Integração na geração e no discover

- [x] 5.1 `infer.Generate` aceita evidências: sinal no candidato existente, candidato novo com âncora de chave, tipos compatíveis, sem FK declarada
- [x] 5.2 Par sem âncora de chave não vira candidato
- [x] 5.3 Ambos os lados com chave geram as duas direções
- [x] 5.4 Ligar o probe no `discover` entre catálogo e geração
- [x] 5.5 Teste de determinismo da geração com evidência

## 6. Verificação

- [x] 6.1 Fixture `usage_evidence`: relação descobrível apenas via view (nome sem semelhança com o alvo), relação via função PL/pgSQL, função com SQL dinâmico em string, view que reforça candidato de nome, dados íntegros para confirmação
- [x] 6.2 Teste de integração: a relação invisível ao nome passa de inexistente a confirmada com o probe ligado
- [x] 6.3 Teste de integração: o candidato reforçado sobe de score em relação à execução sem probe
- [x] 6.4 Teste de integração: função com SQL dinâmico não produz evidência nem erro
- [x] 6.5 Varredura de vazamento ponta a ponta incluindo a fixture nova
- [x] 6.6 Rodar `golangci-lint run` e zerar os apontamentos
- [x] 6.7 Confirmar que `go test ./...` segue sem Docker e sem rede
- [x] 6.8 Rodar `openspec validate sql-join-probe` e corrigir o que apontar
