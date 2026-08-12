## 1. Derivação ancorada

- [x] 1.1 Resolver cada posição por espelho ou por forma prefixada, mantendo a mesma forma em todas as prefixadas
- [x] 1.2 Exigir pelo menos uma âncora; conjunto sem âncora nenhuma continua sujeito à exigência de alvo único
- [x] 1.3 Conferir que casamento todo prefixado e casamento todo espelho seguem com o comportamento de antes
- [x] 1.4 Substituir a fixture de derivação misturada: o que era armadilha vira o padrão, e a armadilha nova é o caso sem âncora com mais de um alvo possível
- [x] 1.5 Atualizar os testes que codificavam a regra de uniformidade, com o motivo escrito onde ele possa ser lido depois

## 2. Medição da derivação ancorada

- [x] 2.1 Rodar o corpus e registrar quantas das 53 chaves compostas do GitLab voltaram: 1, e as 52 restantes têm causa nomeada
- [x] 2.2 Conferir o efeito colateral na aridade 1: 1.654 para 1.662 candidatos, os 8 vindos da geração composta
- [x] 2.3 Se não pagar, a regra sai e o motivo fica registrado

## 3. Alvo com namespace: medir antes de escrever

- [x] 3.1 Contar, sobre a lista real de tabelas, quantas das 53 chaves compostas teriam alvo alcançável por segmento final único
- [x] 3.2 Resultado: 0 de 53, com as 13 tabelas alvo todas em segmento disputado — a regra não é escrita
- [x] 3.3 Registrar a contagem no design, com a tabela por alvo, para que a decisão possa ser refeita por quem discordar
- [x] 3.4 Conferir que a detecção de prefixo de tabela segue intocada

## 5. Verificação

- [x] 5.1 `go test ./...` verde, com atenção aos golden files: qualquer um que mude precisa de justificativa própria
- [x] 5.2 Suíte de integração verde, com atenção ao cenário-armadilha e ao teste de que nada é confirmado indevidamente
- [x] 5.3 `golangci-lint run` limpo nas três etiquetas
- [x] 5.4 Relatório do benchmark regenerado, e README atualizado com os números novos
- [x] 5.5 Registrar a decomposição do ganho: 1 chave da âncora, 0 do namespace — que por isso não foi escrito
- [x] 5.6 Rodar `openspec validate anchored-matching --strict` e corrigir o que apontar
