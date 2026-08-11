## Why

Um quarto do escopo fica de fora. No banco municipal medido, 86 tabelas em 338 têm chave primária composta, e a versão atual as pula com nota — corretamente, mas o que sobra é um recall calculado sobre três quartos do schema. A fase 8 existe para publicar esse número; publicá-lo com o recorte de fora seria enganoso por omissão, e é por isso que as chaves compostas sobem do "fora de escopo" original.

A segunda razão é de contrato. O JSON é API pública a partir do primeiro release, e a fase 8 é esse release. Um candidato hoje carrega uma coluna de cada lado; suportar chave composta muda a forma de `child` e `parent`. Feito agora, a versão 1 do contrato já nasce na forma definitiva. Feito depois, o primeiro consumidor a existir é também o primeiro a ser quebrado.

E a estrutura já espera por isso: o modelo carrega chave de várias colunas desde a fase 1, a cobertura registra o motivo do descarte, o gerador de `NOT VALID` do `audit` já monta anti-join por tupla, e a validação por anti-join generaliza para tupla sem mudança conceitual. O que falta não é mecanismo novo, é atravessar a aridade pelas camadas.

## What Changes

- **BREAKING (pré-release)**: `model.Candidate` passa a referenciar chaves, não colunas. Entra `model.KeyRef` com lista ordenada de colunas; coluna única vira o caso de aridade 1. Um caminho de código só — representação dupla seria a divergência da fase seguinte.
- **Geração**: alvo com chave composta deixa de ser pulado. O lado filho precisa casar **todas** as colunas da chave, na mesma tabela, com tipos compatíveis e com a mesma regra de derivação em todas as partes. Casamento parcial não gera candidato: vira observação com quantas partes casaram, que é onde o recall perdido fica visível.
- **Pontuação**: sinal novo de concordância por aridade. `n` colunas casando de uma vez é evidência de outra ordem de grandeza, e é o que impede que a aridade seja tratada como se fosse `n` coincidências independentes.
- **Pré-filtro**: a checagem de cardinalidade passa a usar o limite inferior seguro `distinto(tupla) ≥ máx distinto(coluna)`. Rejeita só quando o próprio limite inferior já estoura a margem — a camada continua sem inventar rejeição.
- **Validação**: agregação por tupla distinta. Uma linha com NULL em **qualquer** coluna da chave fica isenta, que é o que `MATCH SIMPLE` — o padrão, e o que a DDL emitida usa — significa. Essas linhas passam a ser contadas e reportadas: escapam da constraint por regra, o que é achado, não detalhe.
- **Artefatos**: DDL, query de órfãos, sugestão de índice e nome de constraint passam a lidar com lista de colunas. O gerador de anti-join por tupla, que hoje existe só para o `audit`, passa a ter um dono só.
- **Evidência de uso**: `n` igualdades do mesmo objeto entre o mesmo par de tabelas viram um candidato composto quando o lado pai é exatamente a chave primária. Sem isso, uma relação composta visível só em view continuaria invisível.
- **Cobertura**: `composite_primary_key` deixa de ser motivo de tabela não suportada. Os outros motivos — sem chave, particionada, herança — ficam.

## Capabilities

### Modified Capabilities

- `domain-model`: um candidato referencia chave de uma ou mais colunas, e a validação passa a contar as linhas que `MATCH SIMPLE` isenta.
- `candidate-generation`: alvo com chave composta é alvo legítimo, sob a regra de casamento total.
- `candidate-scoring`: entra o sinal de concordância por aridade.
- `stats-prefilter`: a cardinalidade de tupla é estimada por limite inferior, e a faixa é checada por par de colunas.
- `data-validation`: a agregação é por tupla, com a semântica de NULL declarada.
- `sql-artifacts`: DDL, órfãos, índice e nome de constraint sobre lista de colunas.
- `usage-evidence`: junção composta ancora candidato composto.
- `report-json`: a forma congelada de `child` e `parent` passa a ser a de chave.
- `catalog-inspection`: chave composta sai da lista de motivos de tabela pulada.

## Impact

Nenhuma dependência nova. Nenhuma flag nova. Nenhum comando novo.

A mudança atravessa oito pacotes porque a aridade atravessa a ferramenta inteira — é o preço de não ter um caminho de código para coluna única e outro para tupla. O que a contém é que a aridade 1 continua sendo a mesma execução de sempre: se um único golden file de cenário só-coluna-única mudar, isso é bug e não atualização.

`schema_version` permanece `"1"`. Incrementar para `"2"` antes de existir consumidor de `"1"` publicaria uma história que ninguém viveu; a forma composta é a que o primeiro release congela.

O risco desta change é falso positivo confirmado por casamento de nome barato — `empresa_id, filial_id` casa com meia dúzia de tabelas num ERP. É contra isso que o casamento é total e uniforme, que a ambiguidade continua sem ser resolvida por palpite, e que o parcial não gera nada.

Esta é a primeira de três changes da fase 8. As outras duas — o harness do corpus e a distribuição — dependem desta, e nenhum número é publicado antes das três fecharem: um recall medido sem chave composta mediria a ferramenta anterior.
