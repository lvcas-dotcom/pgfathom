## Why

A penalidade de nome genérico (`penaltyGenericDomain`, `internal/infer/score.go`) existe,
pelo que o próprio spec diz, para **ordenar**:

> O sinal de nome genérico existe porque essas relações costumam ser reais e pouco
> interessantes, e sem penalidade elas dominariam o relatório por volume, **empurrando os
> achados valiosos para o fim**.
> — `openspec/specs/candidate-scoring/spec.md`

A implementação a fazia **filtrar**. `finalize` compara `MetaScore` — já com a penalidade
descontada — contra `MinScore`, e o candidato some antes de qualquer validação.

Medido contra um Postgres de verdade, schema em inglês com tabelas no plural, perfil `en`:

```
DISCARDED  public.products.category_id → public.categories.id
           score 0.35 below threshold 0.50
```

O detalhamento dos sinais mostra a aritmética inteira:

```
normalized_name=0.15  identical_type=0.25  unique_target=0.20  not_null=0.05
generic_domain_name=-0.30                                        → 0.35
```

Sem a penalidade seriam 0,65, bem acima do limiar. Com ela, 0,35 — e a relação, que a
validação teria confirmado com containment de 100%, nunca chegou a ser consultada. Vale
para `status`, `type`, `kind`, `state`, `level`, `category`, `origin` e `classification`:
a lista `generic_entities` inteira, em toda tabela de domínio pequena.

Por que passou despercebido: com casamento **exato** a conta fecha em
`0.30+0.25+0.20+0.05-0.30 = 0.50`, exatamente o limiar, e o candidato sobrevive por
aritmética. O corte só morde quando o nome da tabela é plural e o casamento é portanto
normalizado, a 0,15 — que é toda tabela de domínio de schema Rails, Django ou qualquer um
que pluralize. O teste que cobria a penalidade
(`TestGenericNameWithSmallTargetIsPenalized`) usava alvo no singular **e** baixava
`MinScore` para 0,01, então nunca exercitou o limiar de verdade.

Isso quebra duas regras invioláveis de `openspec/project.md` ao mesmo tempo. A regra 4,
porque o descarte aparece no relatório padrão como contagem sem nome — só
`--include-rejected` diz qual relação sumiu, e o README promete o contrário. E a regra 3 na
direção oposta à usual: a ferramenta afirmou "abaixo do limiar" sobre uma relação cuja
evidência ela se recusou a coletar.

O que não é motivo para mudar: "recall baixo". Recuperar mais nunca justifica sozinho
mexer no corte — a regra 5 diz que na dúvida entre recuperar mais e nunca errar, nunca
errar vence. O motivo é que este sinal específico não fala sobre verdade.

## What Changes

- `internal/infer/score.go`: `score` passa a delegar a saturação a `saturate`, dono único
  da regra de faixa. Novo `cutScore`, que soma os sinais do candidato **exceto**
  `SigGenericDomain` e satura pelo mesmo `saturate`.
- `internal/infer/generate.go`: `finalize` compara `cutScore(c)` com o limiar, e não
  `MetaScore`. `MetaScore` continua com a penalidade descontada, então a ordenação — a
  razão de a penalidade existir — não muda em nada.
- A isenção é de um sinal só. `penaltyAmbiguousTarget` continua cortando: alvo ambíguo é
  afirmação sobre probabilidade da hipótese — a ferramenta genuinamente não sabe qual
  tabela é —, e tipo apenas compatível também. Nome genérico é a única penalidade que diz
  "pouco interessante" em vez de "provavelmente falso".
- A isenção não resgata candidato fraco: quem já estava abaixo do limiar sem a penalidade
  continua descartado, com motivo registrado.

### Custo

Uma query de validação a mais por chave de domínio real, contra o banco do usuário. No
schema de medição: 4 candidatos validados antes, 5 depois. É o custo de validar uma
relação em vez de descartá-la sem olhar.

**O corpus público não consegue medir isto, e o relatório precisa dizer isso em vez de
publicar "custo zero".** `make benchmark` rodado nesta branch devolve recall e contagem de
candidatos **idênticos** aos publicados — GitLab 1066/1069/1754/1757, Discourse
403/396/408/400. O motivo não é que a mudança não faz nada: é que a penalidade exige
`rows < smallTableRows` **e** estimativa conhecida (`internal/infer/generate.go`), e um
`structure.sql` não tem linha nenhuma. No corpus a penalidade nunca é emitida, então não
há o que isentar. A medição desta change é contra banco com dado — fixtures de integração
e o schema do `docs/DEMO.md`, cuja saída publicada não muda.

## Alternatives rejected

**Tirar `category`, `status` e companhia de `generic_entities`.** Resolve o sintoma e
apaga o sinal: a ordenação que a penalidade entrega é boa e é o motivo de ela existir. Além
disso teria de ser repetido em `en`, `pt-br` e `es`, e no perfil que o próximo contribuidor
escrever.

**Baixar a penalidade de -0,30 para algo que não cruze o limiar.** Número escolhido para
caber no limiar padrão, que é configurável — `--min-score 0.7` traz o bug de volta. Troca
uma regra por uma coincidência aritmética.

**Deixar como está e mostrar os descartados por padrão.** Não resolve: a relação continua
sem ser validada, e o usuário lê "descartado por score" sobre uma chave que o dado
confirmaria. Mostrar mais sobre uma resposta errada não a torna certa.
