## 1. O custo por etapa volta com o resultado

- [x] 1.1 `discovery.Result` passa a carregar o tempo de cada estágio executado, na ordem, identificado pelo nome que a change anterior deu a ele
- [x] 1.2 Estágio desligado por opção não aparece, em vez de aparecer com duração zero
- [x] 1.3 Contabilizar o funil de candidatos por ponto — gerados, sobreviventes do limiar, checados pelo pré-filtro, rejeitados, validados, estourados — reusando o que a cobertura já conta e acrescentando só o que falta
- [x] 1.4 Teste de que os tempos somam aproximadamente o total da execução e que a ordem é a de execução

## 2. Manifesto e aquisição

- [x] 2.1 Formato do manifesto em TOML versionado: nome, tipo de aquisição, origem, commit, `sha256`, imagem de servidor e perfil
- [x] 2.2 Entradas de GitLab e Discourse, com commit fixado e checksum conferido na máquina
- [x] 2.3 Entrada local opcional para o dump real em português, ausente por padrão
- [x] 2.4 Busca com verificação de checksum, para diretório ignorado pelo git; divergência aborta nomeando esperado e obtido
- [x] 2.5 Alvo `make corpus` para buscar e conferir, separado do alvo que mede
- [x] 2.6 `.gitignore` cobrindo o diretório de cache do corpus

## 3. O harness

- [x] 3.1 `internal/bench` atrás de `//go:build benchmark`, com os helpers de contêiner de `testutil` passando a valer para as duas etiquetas
- [x] 3.2 Carregar um schema num servidor descartável, com a imagem que o manifesto declara
- [x] 3.3 Ler as chaves declaradas como gabarito, antes de qualquer modificação
- [x] 3.4 Derrubar todas as chaves pela conexão do harness, com a separação em relação à conexão da ferramenta explícita no código
- [x] 3.5 Executar `discovery.Run` em processo nas três configurações — perfil sozinho, com detecção, com evidência de uso
- [x] 3.6 Casamento exato de chave contra o gabarito, coluna a coluna e na ordem, sem crédito parcial
- [x] 3.7 Contar os candidatos fora do gabarito em campo próprio, que nenhuma taxa de erro consome
- [x] 3.8 Registrar o recorte: tabelas fora da análise com motivo, chaves apontando para fora do escopo medido, candidatos que estouraram o teto
- [x] 3.9 `make benchmark` mede sem rede e, sem corpus, falha nomeando o comando que resolve
- [x] 3.10 Saída documentada para medir contra servidor já carregado, para máquina sem Docker; o harness continua lendo o gabarito e derrubando as chaves

## 4. Relatório

- [x] 4.1 Tabela por schema com as três configurações, denominador declarado, versão da ferramenta, servidor e perfil
- [x] 4.2 Bloco de custo por schema: tempo por etapa, funil e número de queries de validação
- [x] 4.3 Declarar que o corpus público não tem dados e que nenhum veredito é medido nele
- [x] 4.4 Declarar entrada opcional ausente, em vez de omiti-la
- [x] 4.5 Escrever `docs/benchmark/` de forma determinística, para que reexecutar sem mudança não produza diff
- [x] 4.6 README passa a apontar para o relatório e a trazer a linha de cabeçalho, com as medições privadas ao lado e identificadas como tal

## 5. Primeira medição e calibração

- [x] 5.1 Rodar a linha de base com os valores vigentes e registrá-la antes de qualquer ajuste
- [x] 5.2 Ler a linha de base contra o esperado: recall do GitLab por configuração, e o que o Discourse mostra com 23 chaves em 354 tabelas
- [x] 5.3 Rever limiar de score, margem do pré-filtro, limiares de veredito e pesos de aridade, cada mudança justificada pelo número que a motivou, com antes e depois
- [x] 5.4 Conferir que nenhuma calibração foi feita antes da linha de base existir
- [x] 5.5 Reexecutar a suíte inteira após a calibração: golden file que mudar precisa de justificativa própria

## 6. Verificação

- [x] 6.1 `go test ./...` sem Docker e sem rede continua verde, e nada do harness entra em build sem a etiqueta
- [x] 6.2 Suíte de integração verde, com atenção ao teste que prova que a ferramenta não escreve
- [x] 6.3 `golangci-lint run` limpo nas três etiquetas: sem etiqueta, `integration` e `benchmark`
- [x] 6.4 `make corpus && make benchmark` a partir de repositório limpo, com o tempo total registrado
- [x] 6.5 Reexecutar o benchmark sobre o mesmo corpus e conferir que o relatório não muda, descontado o que é tempo
- [x] 6.6 Conferir que `go.mod` não ganhou dependência
- [x] 6.7 Rodar `openspec validate benchmark-corpus --strict` e corrigir o que apontar
