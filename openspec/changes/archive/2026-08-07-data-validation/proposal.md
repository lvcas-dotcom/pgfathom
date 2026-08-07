## Why

Tudo até aqui produziu perguntas. As fases 1 a 4 leem catálogo, geram hipóteses, pontuam por fatos e matam as impossíveis por estatística — e nenhuma delas consegue dizer se `pedido.cliente_id` de fato aponta para `cliente.id`. Só os dados podem, e esta fase é onde a ferramenta finalmente os consulta.

É o núcleo do produto e o motivo dos vereditos existirem: **confirmada** é a FK que alguém esqueceu de declarar; **quebrada** é o bug de dados que está em produção há anos sem ninguém saber — o achado que justifica a ferramenta. É também onde mora todo o risco operacional: query pesada, timeout, carga em banco de produção. Chegar aqui com modelo, perfis, catálogo, inferência e pré-filtro já testados significa que, quando algo quebrar, dá para saber que quebrou aqui.

## What Changes

- `internal/validate`: a query de agregação por valor distinto — uma por candidato, contagens apenas, nunca linhas — que entrega contenção por linha e por valor na mesma passada e produz a métrica de cardinalidade de graça.
- Modo amostrado por padrão com `TABLESAMPLE SYSTEM`, fallback `BERNOULLI` em tabela pequena, e leitura direta quando a tabela cabe no alvo. `--full` para o modo conclusivo.
- `statement_timeout` por validação; candidato que estoura vira `unvalidated` com motivo e a execução continua.
- Concorrência limitada por `errgroup.SetLimit`, respeitando o teto configurado da sessão.
- Atribuição de veredito: confirmada, quebrada, fraca, rejeitada — com a regra sem exceção de que amostra nunca confirma.
- `discover` passa a reportar vereditos com as métricas de validação, e a cobertura ganha os contadores de validados e estourados que já existem no modelo.

## Capabilities

### New Capabilities

- `data-validation`: a formulação da query, os modos de execução e seus limites, o regime de timeout e concorrência, e as regras de veredito — incluindo o que amostra pode e não pode afirmar.

### Modified Capabilities

Nenhuma. `candidate-generation`, `candidate-scoring` e `stats-prefilter` alimentam esta camada sem mudança de requisito; o contrato de `Validation` e dos vereditos já existe em `domain-model` desde a fase 1.

## Impact

Nenhuma dependência nova no binário: `errgroup` já está no módulo via pgx. `internal/validate` importa `internal/db` e `internal/model`.

Esta é a única camada do programa autorizada a ler linhas de tabela do usuário. A leitura produz contagens e morre ali — o teste de vazamento de ponta a ponta já varre a saída inteira do binário e passa a cobrir também os caminhos novos.

O custo de uma execução muda de natureza: `discover` deixa de ser só catálogo e passa a disparar até um anti-join por candidato sobrevivente. Tudo que as fases anteriores fizeram para encolher esse conjunto — limiar, pré-filtro — existe para este momento.
