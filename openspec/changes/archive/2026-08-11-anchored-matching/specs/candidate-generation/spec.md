## ADDED Requirements

### Requirement: Casamento de chave composta é total, ancorado e sem desempate

Para gerar candidato composto, o lado filho SHALL oferecer exatamente uma coluna correspondente a **cada** coluna da chave primária alvo, na mesma tabela, com tipo compatível em todas as posições.

Cada posição SHALL corresponder por uma de duas derivações: **espelho**, em que a coluna filha tem o nome da coluna da chave; e **prefixada**, em que uma forma do nome da entidade alvo, segundo o perfil ativo, precede o nome da coluna da chave. Todas as posições prefixadas SHALL usar a mesma forma.

O conjunto SHALL exigir **pelo menos uma** posição prefixada — a âncora. É ela que carrega o nome do alvo e responde por que aquela tabela e não outra; as demais posições são discriminadores, colunas como `partition_id` ou `empresa_id`, que atravessam o schema e sozinhas não apontam para nada.

Casamento sem nenhuma âncora — todas as posições em espelho — SHALL continuar sujeito à exigência de alvo único, precisamente porque nada nele diz para onde aponta.

Casamento parcial MUST NOT gerar candidato, e SHALL ser registrado como observação com quantas posições casaram sobre o total. Posição que case com mais de uma coluna filha SHALL pular o alvo com nota, e MUST NOT ser desempatada por posição, por tipo, por ordem de declaração ou por qualquer outro critério.

A regra anterior exigia derivação uniforme em todas as posições, sob o argumento de que misturar é o formato de uma coincidência. Medida contra o maior schema público do corpus, ela recuperou zero de 53 chaves compostas: todas as 53 são discriminador mais referência, que é a forma canônica de chave composta em schema particionado ou multi-tenant. O que restava de verdadeiro no argumento — colunas de nome comum casando por acaso — é o caso sem âncora, e esse continua barrado.

#### Scenario: Discriminador mais referência casa

- **WHEN** a filha oferece `partition_id` em espelho e `build_id` como âncora para a chave `(id, partition_id)` de uma tabela alcançável por `build`
- **THEN** o candidato composto é gerado

#### Scenario: Espelho puro continua exigindo alvo único

- **WHEN** todas as posições casam por espelho e mais de uma tabela carrega aquela assinatura de chave
- **THEN** nenhum candidato é gerado, e a nota registra quantas tabelas dividem a assinatura

#### Scenario: Prefixada em todas as posições continua casando

- **WHEN** cada coluna da chave alvo aparece na filha precedida pela mesma forma do nome do alvo
- **THEN** o candidato composto é gerado

#### Scenario: Formas diferentes entre posições não casam

- **WHEN** duas posições casam por prefixo, cada uma com uma forma diferente do nome do alvo
- **THEN** nenhum candidato é gerado

#### Scenario: Parcial vira observação, nunca candidato

- **WHEN** duas das três colunas da chave primária alvo têm correspondente na filha
- **THEN** nenhum candidato é gerado, e a observação registra que duas de três posições casaram

#### Scenario: Ambiguidade de posição pula o alvo

- **WHEN** uma coluna da chave primária alvo tem mais de uma correspondente possível na tabela filha
- **THEN** o alvo é pulado com nota, e nenhuma das correspondentes é escolhida

#### Scenario: Uma posição incompatível derruba o conjunto

- **WHEN** todas as posições casam por nome mas uma delas tem tipo incompatível
- **THEN** nenhum candidato é gerado

## REMOVED Requirements

### Requirement: Casamento de chave composta é total, uniforme e sem desempate

**Reason**: A exigência de derivação uniforme em todas as posições foi escrita antes de existir corpus para medi-la. Medida contra o maior schema público disponível, recuperou zero de 53 chaves compostas: todas são discriminador mais referência, que a uniformidade classifica como coincidência e que é, na verdade, a forma canônica de chave composta em schema particionado ou multi-tenant.

**Migration**: Substituída por "Casamento de chave composta é total, ancorado e sem desempate". O que muda é só o meio do intervalo: casamento todo prefixado continua valendo, casamento todo espelho continua exigindo alvo único, e o que passa a casar é o conjunto com pelo menos uma âncora e o resto em espelho. Todas as recusas — parcial, posição ambígua, tipo incompatível, assinatura compartilhada sem âncora — seguem valendo com o mesmo texto.
