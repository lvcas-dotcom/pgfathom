## MODIFIED Requirements

### Requirement: Cardinalidade impossível penaliza, e rejeita só além da margem

Quando o número estimado de valores distintos da coluna filha excede o número estimado de linhas da tabela pai, o candidato SHALL receber o sinal negativo de violação de cardinalidade e ter o score recomposto. A rejeição direta SHALL ocorrer apenas quando a violação excede uma margem de tolerância larga, e SHALL registrar o motivo com as duas estimativas declaradas como estimativas.

Para chave de mais de uma coluna, a cardinalidade da tupla SHALL ser estimada pelo **limite inferior** `distinto(tupla) ≥ máx distinto(coluna)`, e a checagem SHALL usar esse limite. `pg_stats` não carrega cardinalidade de tupla — obtê-la exigiria `CREATE STATISTICS`, que é DDL, e a ferramenta não emite DDL.

O limite inferior é suficiente para a única coisa que esta camada faz: se ele já excede a margem, nenhuma estatística melhor salvaria o candidato. O que se perde é sensibilidade — combinação impossível que só se revela na tupla passa —, e isso custa um anti-join a mais. Estimar a tupla por multiplicação de cardinalidades custaria a regra de nunca rejeitar sem base, porque a independência entre as colunas de uma chave é justamente o que não se pode supor.

#### Scenario: Violação dentro da margem penaliza

- **WHEN** a filha tem mais valores distintos estimados que as linhas do pai, mas menos que o dobro
- **THEN** o candidato carrega o sinal de violação de cardinalidade, tem score reduzido e não é rejeitado

#### Scenario: Violação além da margem rejeita

- **WHEN** os valores distintos estimados da filha excedem o dobro das linhas estimadas do pai
- **THEN** o candidato é rejeitado com motivo que nomeia as estimativas envolvidas

#### Scenario: Rejeição estatística é reportável

- **WHEN** o usuário pede para ver os descartados
- **THEN** os rejeitados pela estatística aparecem com o motivo

#### Scenario: Chave composta usa o limite inferior

- **WHEN** um candidato de chave composta tem uma coluna cuja cardinalidade estimada já excede a margem sobre as linhas do pai
- **THEN** ele é rejeitado, e o motivo nomeia a coluna que produziu o limite inferior

#### Scenario: Sem estimativa em nenhuma posição, sem opinião

- **WHEN** nenhuma coluna da chave filha tem estatística utilizável
- **THEN** o candidato passa com o marcador de estatística indisponível, sem penalidade e sem rejeição

### Requirement: Faixa penaliza e nunca rejeita sozinha

Para colunas da família numérica, quando os limites do histograma da filha caem fora dos limites da chave do pai, o candidato SHALL receber o sinal negativo de violação de faixa. Este sinal MUST NOT causar rejeição por si só, qualquer que seja a magnitude.

Em chave de mais de uma coluna, a checagem SHALL ser feita posição a posição, e o sinal SHALL ser emitido no máximo uma vez por candidato, ainda que mais de uma posição viole. Uma chave disjunta em três posições é um fato, não três, e emitir três penalidades deixaria a aridade decidir o score por volume.

Limites de histograma são amostra, e tabela que cresceu após o `ANALYZE` tem valores fora dos limites antigos por construção. Para tipos fora da família numérica a checagem de faixa SHALL ser omitida.

#### Scenario: Faixa deslocada penaliza

- **WHEN** os limites da filha caem inteiramente fora dos limites da chave do pai
- **THEN** o candidato carrega o sinal de violação de faixa e tem score reduzido, sem ser rejeitado

#### Scenario: Tipo não numérico não opina sobre faixa

- **WHEN** o candidato liga colunas de tipo textual, `uuid` ou data
- **THEN** nenhum sinal de faixa é emitido, em nenhuma direção

#### Scenario: Duas posições disjuntas penalizam uma vez

- **WHEN** duas posições de uma chave composta têm faixas disjuntas das correspondentes no pai
- **THEN** o candidato carrega um único sinal de violação de faixa
