## Why

O casamento de nome tem um teto estrutural: `os_servico.resp_tecnico → funcionario.id` é invisível para qualquer heurística de nomenclatura, porque `resp_tecnico` não se parece com `funcionario` em idioma nenhum. Mas se uma view do próprio banco faz `JOIN funcionario f ON o.resp_tecnico = f.id`, o código real já declarou a relação — só não no lugar onde o catálogo de constraints olha.

Esta fase minera essa evidência: predicados de junção extraídos de definições de view e corpos de função. É puro catálogo — nenhum dado de usuário, custo desprezível — e encontra relações que nome estruturalmente não alcança. Vindo depois da validação, o ganho é mensurável: existe uma linha de base de recall só com nome, e ligar o probe produz a decomposição que o README promete publicar.

## What Changes

- `internal/sqlprobe`: tokenizador que atravessa dollar quoting (`$$` e `$tag$`), comentário de linha e de bloco, string e identificador entre aspas; extração de igualdades entre referências de coluna qualificadas, com resolução de alias de `FROM`/`JOIN`.
- Leitura das definições de view e dos corpos de função SQL/PL/pgSQL dos schemas em escopo, e — quando a extensão existir — das queries normalizadas de `pg_stat_statements`.
- Cada junção extraída vira sinal de peso alto no candidato correspondente e **também cria candidato** que a heurística de nome jamais geraria, desde que um dos lados seja chave única de coluna única e os tipos sejam compatíveis.
- O extrator degrada, nunca falha: SQL que ele não entende é ignorado. Junção perdida é sinal perdido; em nenhum caso o extrator produz resposta errada — a validação da fase 5 é quem julga.
- A cobertura declara se `pg_stat_statements` estava disponível, pelo campo que o modelo já tem.

## Capabilities

### New Capabilities

- `usage-evidence`: o que o extrator reconhece e o que tem permissão de ignorar, de onde a evidência vem, como junção vira sinal e vira candidato, e o teto de segurança — evidência de uso nunca confirma nada sozinha.

### Modified Capabilities

- `candidate-generation`: a geração passa a aceitar evidência de junção como segunda origem de candidato, com as mesmas regras duras de tipo e de alvo.

## Impact

Nenhuma dependência nova — e essa é a decisão central da fase: um parser SQL completo exigiria `pg_query_go`, que depende de cgo e quebraria o cross-compile trivial. O extrator estreito com permissão explícita de falhar é o que mantém o `go.mod` curto.

`internal/sqlprobe` é determinístico no que extrai (`Extract` é puro); a leitura das fontes importa `internal/db`. `internal/infer` ganha uma entrada nova e continua sem I/O.

O risco de candidato ruim vindo de extração errada é limitado por construção: ele nasce `unvalidated` como qualquer outro e morre na validação contra dados.
