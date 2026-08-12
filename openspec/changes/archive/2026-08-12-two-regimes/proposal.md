## Why

O harness derruba todas as chaves declaradas e mede quantas voltam. Medido contra um schema real de gestão municipal em português — o tipo de banco que metade das decisões deste projeto existe para atender —, o resultado foi **1,8%**: 5 chaves de 277.

Esse número não descreve a ferramenta. Descreve o procedimento.

A detecção de nomenclatura aprende o afixo de referência lendo as chaves que o schema **já declara**. Contra o mesmo banco com as chaves no lugar, ela aprende sozinha o sufixo `_idkey` em 102 colunas e o prefixo `idkey_` em 130, e o recall vai para **82,2%** — que reconcilia com os 79,0% medidos à mão contra outro banco do mesmo fornecedor. Derrubar tudo apaga a evidência que a detecção lê, e o procedimento passa a medir uma ferramenta sem metade dos olhos.

O erro não é medir o cenário sem integridade declarada: esse cenário é real, é o mais difícil, e é onde a ferramenta se vende. O erro é publicar só ele, com um rótulo que diz "recall" e um leitor que entende "é isso que a ferramenta faz".

## What Changes

- Duas regimes de medição por schema, ambas publicadas:
  - **parcial** — metade das chaves é derrubada e é o gabarito; a metade que fica alimenta a detecção. É o banco que declarou alguma integridade e esqueceu o resto, que é o caso comum.
  - **greenfield** — o resto também cai, e o gabarito é o conjunto inteiro. É o banco que não declara nada, e é o que o harness mede hoje.
- A seleção da metade é determinística, para que duas execuções sobre o mesmo corpus produzam o mesmo número.
- As duas regimes rodam na mesma carga, em sequência: greenfield começa exatamente do estado em que a parcial termina, então a segunda medição não custa uma segunda carga.
- O relatório publica as duas lado a lado, e diz o que cada uma mede.
- O README passa a mostrar as duas, com a linha do schema em português — que é a primeira medição publicada contra o mercado-alvo do projeto.

## Capabilities

### Modified Capabilities

- `benchmark-harness`: a medição passa a ter duas regimes com significados distintos, e o relatório passa a nomear qual cenário cada número descreve.

## Impact

Nenhuma dependência nova. Nenhuma mudança em código de produto — o defeito está no que se mede, não no que a ferramenta faz.

O custo em tempo é próximo de zero, porque a segunda regime aproveita o estado deixado pela primeira. O que dobra é o número de linhas publicadas por schema, e é exatamente esse o ponto.

O risco é publicar duas taxas e deixar o leitor escolher a que lhe agrada. Contra isso, cada linha declara o cenário no próprio rótulo, e o texto diz qual das duas descreve a primeira execução de alguém contra um banco legado típico — que é a parcial, porque banco que não declara **nenhuma** chave é mais raro do que a nossa medição atual faz parecer.
