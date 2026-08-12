## 1. As duas regimes

- [x] 1.1 Tipo de regime com os dois valores, e o rótulo que aparece no relatório
- [x] 1.2 Divisão determinística do gabarito por posições alternadas na ordem das relações
- [x] 1.3 Derrubar somente as chaves de uma das metades, na regime parcial
- [x] 1.4 Medir o recall da parcial sobre a metade removida, e não sobre o conjunto
- [x] 1.5 Greenfield partindo do estado deixado pela parcial: derruba o resto e mede sobre o conjunto inteiro
- [x] 1.6 Teste de que duas execuções dividem igual, sem contêiner

## 2. Relatório

- [x] 2.1 Recall por regime e por configuração, com o cenário nomeado em cada linha
- [x] 2.2 Texto dizendo qual regime descreve a primeira execução contra banco legado que declarou alguma integridade
- [x] 2.3 Custo por etapa cobrindo as seis execuções por schema, no arquivo que não é estável
- [x] 2.4 Manter estável o arquivo de recall: mesma entrada, mesmo byte

## 3. Medição

- [x] 3.1 Rodar as três entradas nas duas regimes e registrar o antes e o depois
- [x] 3.2 Conferir a hipótese central: no schema em português a parcial deve subir muito, no GitLab quase nada
- [x] 3.3 Ler o que a detecção aprende na parcial de cada schema

## 4. README

- [x] 4.1 Tabela com as duas regimes, incluindo a linha do schema em português
- [x] 4.2 Declarar que o schema em português não é redistribuível, e por que a linha existe mesmo assim
- [x] 4.3 Rever a seção sobre a detecção: o texto atual diz que ela é medida sem entrada, e passa a haver a medição com

## 5. Verificação

- [x] 5.1 `go test ./...` e lint nas três etiquetas
- [x] 5.2 Reexecutar o benchmark e conferir que o arquivo de recall não muda
- [x] 5.3 Rodar `openspec validate two-regimes --strict`
