## 1. Modelo

- [ ] 1.1 Adicionar ao modelo o tipo de evidência de junção: par de colunas resolvido, origem (view, função, statements) e objeto de origem
- [ ] 1.2 Pesos dos três sinais de junção em `internal/infer`, acima de qualquer sinal de nome, com teste de ordenação relativa

## 2. Tokenizador

- [ ] 2.1 Criar `internal/sqlprobe` com o tokenizador: identificador nu (minúsculas) e citado (caixa e espaços preservados), pontuação relevante, número
- [ ] 2.2 Atravessar comentário de linha e de bloco (aninhado), string com aspas simples e escape, string `E''`
- [ ] 2.3 Atravessar dollar quoting com e sem tag, incluindo tag aninhada distinta
- [ ] 2.4 Tabela de casos do tokenizador cobrindo cada forma e as combinações traiçoeiras

## 3. Extrator

- [ ] 3.1 Construir o mapa de alias por statement a partir de `FROM` e `JOIN`, com e sem `AS`, com schema qualificado
- [ ] 3.2 Reconhecer igualdades `referência = referência` em `ON` e `WHERE`, resolvendo ambos os lados
- [ ] 3.3 Deduplicar evidências por par resolvido e origem
- [ ] 3.4 Ignorar sem erro: SQL truncado, sintaxe desconhecida, referência não resolvida
- [ ] 3.5 Tabela de casos do extrator: junção simples, múltiplas junções, alias conflitante, `WHERE` implícito, SQL malformado

## 4. Fontes

- [ ] 4.1 Ler definições de view dos schemas em escopo
- [ ] 4.2 Ler corpos de função `sql` e `plpgsql` dos schemas em escopo
- [ ] 4.3 Ler `pg_stat_statements` quando disponível, marcando a disponibilidade na cobertura
- [ ] 4.4 Teste que falha se alguma consulta da camada referenciar relação fora do catálogo e das visões de estatística

## 5. Integração na geração e no discover

- [ ] 5.1 `infer.Generate` aceita evidências: sinal no candidato existente, candidato novo com âncora de chave, tipos compatíveis, sem FK declarada
- [ ] 5.2 Par sem âncora de chave não vira candidato
- [ ] 5.3 Ambos os lados com chave geram as duas direções
- [ ] 5.4 Ligar o probe no `discover` entre catálogo e geração
- [ ] 5.5 Teste de determinismo da geração com evidência

## 6. Verificação

- [ ] 6.1 Fixture `usage_evidence`: relação descobrível apenas via view (nome sem semelhança com o alvo), relação via função PL/pgSQL, função com SQL dinâmico em string, view que reforça candidato de nome, dados íntegros para confirmação
- [ ] 6.2 Teste de integração: a relação invisível ao nome passa de inexistente a confirmada com o probe ligado
- [ ] 6.3 Teste de integração: o candidato reforçado sobe de score em relação à execução sem probe
- [ ] 6.4 Teste de integração: função com SQL dinâmico não produz evidência nem erro
- [ ] 6.5 Varredura de vazamento ponta a ponta incluindo a fixture nova
- [ ] 6.6 Rodar `golangci-lint run` e zerar os apontamentos
- [ ] 6.7 Confirmar que `go test ./...` segue sem Docker e sem rede
- [ ] 6.8 Rodar `openspec validate sql-join-probe` e corrigir o que apontar
