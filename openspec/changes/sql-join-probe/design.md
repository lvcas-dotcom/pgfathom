## Context

A evidência mais confiável sobre relações não declaradas não está nos nomes: está no SQL que o próprio banco guarda — views, funções e, quando disponível, o log de queries normalizado. Uma junção ali é prova de que o código trata duas colunas como relacionadas, sejam quais forem seus nomes.

O problema é ler SQL sem pagar o preço de um parser. `pg_query_go` traria o parser real do PostgreSQL — e traria cgo, quebrando a regra de cross-compile trivial que é requisito de produto. Todo o desenho decorre de recusar essa dependência.

## Goals / Non-Goals

**Goals**

Extrair igualdades entre referências de coluna qualificadas, com resolução de alias, de qualquer SQL bem-comportado. Transformar cada uma em sinal e, quando o par não existe por nome, em candidato novo. Ignorar sem erro tudo que o extrator não entende.

**Non-Goals**

Nenhum parse completo: subquery, CTE, `USING`, junção lateral e SQL dinâmico ficam explicitamente fora do reconhecimento garantido — se o extrator os pega, é bônus; se perde, é sinal perdido. Nenhum veredito: evidência de uso pontua e gera hipótese, quem conclui é a validação. `pg_stat_statements` é oportunista, nunca requisito.

## Decisions

### Um extrator estreito com permissão explícita de falhar

A alternativa a um parser não é um parser pior — é mudar a pergunta. O extrator não entende SQL; ele procura uma única forma sintática: `referência = referência`, onde ambos os lados são `alias.coluna` (ou `tabela.coluna`), e resolve os aliases olhando as cláusulas `FROM`/`JOIN` do mesmo statement.

Essa permissão de falhar é o que torna a abordagem segura. A saída é *sinal*, nunca veredito. Junção não extraída devolve o candidato ao caminho normal de inferência por nome. Junção extraída errada gera um candidato ruim, que nasce `unvalidated` e morre na validação contra dados. Em nenhum dos dois casos existe resposta errada — só mais ou menos ajuda.

### O tokenizador trata o que faria o extrator mentir

Comentários, strings, dollar quoting e identificadores entre aspas não são casos de borda aqui — corpo de função PL/pgSQL vem inteiro dentro de `$$`, e um `=` dentro de string ou comentário viraria predicado fantasma. O tokenizador existe para que o extrator só veja código, nunca texto.

Identificador entre aspas preserva caixa e espaços; identificador nu normaliza para minúsculas, como o servidor faz.

### Alias por statement, sem escopo aninhado

Subqueries têm escopos próprios de alias; modelá-los é metade de um parser. O extrator mantém um mapa plano por statement: em conflito de alias entre escopos, a resolução pode errar — e o erro produz no máximo um candidato que a validação derruba. O custo de acertar sempre seria a dependência que a proposta recusa.

### De onde vem o SQL

Views vêm de `pg_views.definition`, que o servidor devolve normalizado e reconstruído — o caso mais limpo. Funções vêm de `pg_proc.prosrc` para as linguagens `sql` e `plpgsql`, restritas aos schemas em escopo. `pg_stat_statements` entra quando a extensão responde, e a cobertura registra se estava disponível — ausência de evidência de uso não pode parecer ausência de uso.

### Junção vira candidato apenas com âncora de chave

`a.x = b.y` não diz quem referencia quem. O lado que é chave única de coluna única é o pai; se nenhum dos lados é, o par não vira candidato — vira nada, porque sem âncora qualquer direção seria chute. Se ambos os lados são chaves, os dois sentidos são hipóteses legítimas e ambos nascem. Coluna filha com FK já declarada não gera candidato, como em toda a geração.

Compatibilidade de tipo continua sendo filtro duro: o servidor aceita juntar `int` com `text` via cast, mas um candidato desses não sobreviveria à criação de constraint, que é o produto final.

### O peso da evidência de uso supera qualquer sinal de nome

Junção no código real é o sinal mais forte que existe — mais que casamento exato de nome — porque não é convenção, é uso. Os três tipos (view, função, statements) têm pesos próprios em `internal/infer`, junto dos demais, porque a régua relativa entre todos os sinais precisa morar num único lugar auditável.

## Risks / Trade-offs

**O extrator vai perder junções reais** → aceito por construção; o caso fica registrado na decomposição de recall da fase 8, que existe para medir exatamente esse gap. A resposta a um gap grande é melhorar o extrator, nunca afrouxar a validação.

**Alias plano pode resolver errado em SQL aninhado** → o candidato errado nasce `unvalidated` e morre na validação. Pior caso real: um anti-join desperdiçado.

**`prosrc` contém SQL dinâmico montado em string** → o tokenizador o vê como string e o ignora inteiro, que é o comportamento correto: extrair de SQL montado seria adivinhar.

**Views materializadas e regras** ficam de fora nesta versão → registrado como limitação conhecida; `pg_views` cobre o caso dominante.

## Migration Plan

Não aplicável. Os sinais `join_in_*` existem no modelo desde a fase 1.

## Open Questions

Nenhuma bloqueante. O peso relativo entre os três tipos de evidência é estimativa declarada, revista com o corpus da fase 8.
