## ADDED Requirements

### Requirement: O extrator reconhece o operador de um predicado de coluna qualificada

Além da igualdade entre duas colunas qualificadas, o extrator SHALL reconhecer um predicado cujo lado esquerdo é uma referência de coluna qualificada e classificar o operador — igualdade, comparação de faixa, `LIKE`/`ILIKE`, contenção (`@>`, `<@`, `?`, `?|`, `?&`), full-text (`@@`) e distância de vetor. O lado direito SHALL ser classificado em literal, parâmetro ou referência, e um `LIKE` sobre literal constante SHALL distinguir prefixo de infixo.

Essa evidência alimenta a recomendação de tipo de índice. A extração de junção existente MUST permanecer inalterada, e predicado que o extrator não reconhece MUST ser ignorado sem erro: a degradação da fase 6 vale igual aqui.

#### Scenario: Predicado de contenção é classificado

- **WHEN** uma função contém `t.dados @> '...'::jsonb` sobre uma coluna qualificada
- **THEN** a evidência de predicado registra a coluna resolvida e o operador de contenção

#### Scenario: LIKE de infixo é distinguido do de prefixo

- **WHEN** o SQL contém `t.nome LIKE '%x%'` e, em outro ponto, `t.cod LIKE 'x%'`
- **THEN** o primeiro é classificado como infixo e o segundo como prefixo

#### Scenario: Operador desconhecido não derruba a extração

- **WHEN** um predicado usa um operador que o extrator não classifica
- **THEN** aquele predicado é ignorado sem erro e a extração das demais fontes segue

#### Scenario: A junção existente segue intacta

- **WHEN** uma view cruza duas colunas por igualdade com aliases
- **THEN** a evidência de junção continua sendo extraída como antes, ao lado da nova evidência de predicado
