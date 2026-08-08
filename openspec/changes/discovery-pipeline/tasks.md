## 1. Dono único da estimativa de linhas

- [x] 1.1 Mover a resolução de `reltuples` para `internal/model`, devolvendo o mapa de estimativas conhecidas por tabela
- [x] 1.2 `internal/stats` e `internal/validate` passam a consumir essa função, e suas cópias de `tableRows` deixam de existir
- [x] 1.3 Teste de que a sentinela de tabela nunca analisada não vira zero em nenhuma das duas camadas

## 2. Helpers de teste com um dono

- [x] 2.1 Mover para `internal/testutil` a lista única de valores plantados pelas fixtures
- [x] 2.2 Criar o helper de varredura de vazamento que recebe textos nomeados e falha apontando o valor encontrado
- [x] 2.3 Criar o helper que verifica que as consultas de um arquivo só referenciam catálogo e visões de estatística, parametrizado pelas relações permitidas
- [x] 2.4 Substituir as quatro cópias de valores plantados e as três de verificação de consulta pelos helpers
- [x] 2.5 Conferir que a lista única cobre todos os valores que cada cópia conhecia, sem perder nenhum

## 3. Interface estreita nas camadas de I/O

- [x] 3.1 `internal/catalog` passa a receber a interface de consulta em vez do pool concreto
- [x] 3.2 `internal/validate` idem, preservando o acesso à transação que o `SET LOCAL` exige
- [x] 3.3 Conferir que `sqlprobe` e `stats` seguem inalterados e que as quatro interfaces são a mesma

## 4. A execução como unidade

- [ ] 4.1 Criar `internal/discovery` com as opções da execução, sem dependência de cobra
- [ ] 4.2 Mover a sequência de estágios do comando para a unidade, preservando ordem e regime de erro de cada um
- [ ] 4.3 Estágio que degrada reporta o aviso a quem chamou, em vez de escrever em stream
- [ ] 4.4 Devolver o `model.Result` montado e o que o relatório precisa além dele
- [ ] 4.5 Identificar cada estágio, deixando o ponto de medição pronto para a fase 8 sem medir nada agora
- [ ] 4.6 `internal/cli` reduzido a validar flags, montar opções, chamar e renderizar
- [ ] 4.7 Conferir que a checagem de privilégio e a resolução de escopo seguem no mesmo ponto da sequência

## 5. Verificação

- [ ] 5.1 `go test ./...` verde sem Docker e sem rede, com os golden files intactos — nenhum arquivo `.golden` pode mudar
- [ ] 5.2 Suíte de integração verde pacote a pacote, com atenção a veredito, contagem de órfãos e conteúdo dos artefatos
- [ ] 5.3 Teste de que a execução programática produz o mesmo resultado do comando, com as mesmas opções
- [ ] 5.4 Conferir que a saída do binário é idêntica à de antes da change, nas fixtures cobertas
- [ ] 5.5 Rodar `golangci-lint run` e `--build-tags integration`, zerando os apontamentos
- [ ] 5.6 Revisar densidade de comentário: a extração não pode multiplicar comentário de trânsito
- [ ] 5.7 Rodar `openspec validate discovery-pipeline --strict` e corrigir o que apontar
