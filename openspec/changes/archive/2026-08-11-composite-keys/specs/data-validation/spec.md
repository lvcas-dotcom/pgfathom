## MODIFIED Requirements

### Requirement: A validação lê contagens, nunca linhas

A query de validação SHALL agregar por valor distinto antes do anti-join e SHALL retornar exclusivamente contagens: valores distintos, linhas não nulas, órfãos por valor, órfãos por linha, máximo de linhas por valor e linhas isentas por nulidade parcial. Nenhum valor de tabela do usuário MUST sair da query para o programa.

Para chave de mais de uma coluna, a agregação SHALL ser por **tupla** distinta e o anti-join SHALL comparar todas as posições, na ordem da chave. A chave SHALL entrar na query por lista de colunas citadas uma a uma, nunca por concatenação de valores: concatenar produziria colisão entre tuplas distintas e faria valor do usuário atravessar uma expressão de texto.

O filtro de nulidade SHALL exigir **todas** as colunas da chave não nulas, e as linhas com NULL em parte da chave SHALL ser contadas em separado. É o que `MATCH SIMPLE` — o padrão do SQL, e o que a DDL emitida usa — significa: NULL em qualquer posição isenta a linha da verificação. Contá-las como órfãs produziria veredito de quebrada que a própria constraint gerada não corroboraria.

Contenção por linha e por valor contam histórias diferentes — um valor inválido repetido um milhão de vezes e um milhão de valores raros inválidos são problemas distintos — e ambas SHALL ser reportadas.

#### Scenario: Só contagens atravessam a fronteira

- **WHEN** uma validação executa
- **THEN** o resultado carrega apenas contagens e durações, e a varredura de vazamento sobre a saída completa do binário não encontra valor plantado

#### Scenario: As duas contenções saem sempre

- **WHEN** um candidato é validado
- **THEN** a contenção por linha e a por valor distinto aparecem nas métricas, junto com as contagens de órfãos nas duas unidades

#### Scenario: Chave composta agrega por tupla

- **WHEN** um candidato de chave composta é validado
- **THEN** os valores distintos contados são tuplas distintas, e o anti-join compara todas as posições

#### Scenario: Nulidade parcial isenta e é contada

- **WHEN** a tabela filha tem linhas com NULL em parte da chave composta
- **THEN** elas não entram em nenhuma contagem de órfãos, e aparecem na contagem de isentas por `MATCH SIMPLE`

#### Scenario: A medição concorda com a constraint emitida

- **WHEN** a DDL gerada para um candidato composto confirmado é aplicada a um banco de teste com as mesmas linhas
- **THEN** ela é criada e validada sem erro, e nenhuma linha que a validação contou como isenta a impede
