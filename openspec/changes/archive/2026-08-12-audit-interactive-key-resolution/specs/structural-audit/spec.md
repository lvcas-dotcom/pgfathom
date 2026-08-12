## ADDED Requirements

### Requirement: A resolução interativa de chave ausente é gated por terminal, nunca por flag

O comando SHALL só oferecer resolução interativa de chave ausente quando stdin e stdout forem ambos um terminal interativo. O comando SHALL NOT introduzir uma flag para ligar ou desligar esse comportamento por si só — ele segue o ambiente de execução. `--no-probe-keys` SHALL desligar a resolução interativa junto com a sondagem automática, porque ambas leem dado da tabela.

#### Scenario: Saída redirecionada nunca pausa

- **WHEN** o comando roda com stdout redirecionado para um arquivo ou pipe
- **THEN** nenhuma tabela sem chave confirmada dispara um prompt, e a saída é idêntica à que o comando produziria sem esta capability

#### Scenario: --no-probe-keys desliga tudo

- **WHEN** o comando roda com `--no-probe-keys`, mesmo em terminal interativo
- **THEN** nenhuma leitura de dado acontece, incluindo a resolução interativa

### Requirement: A resolução de chave ausente é uma decisão única por execução, não por tabela

Quando houver ao menos uma tabela sem chave confirmada e o terminal for interativo, o comando SHALL primeiro avaliar todo o catálogo e só então relatar, uma única vez, quantas tabelas estão pendentes, quantas têm um candidato composto ainda não testado, e o nome de primary key mais comum entre as tabelas do escopo que já têm uma — cada convenção citada acompanhada de exemplos concretos dos objetos que a sustentam. O comando SHALL perguntar no máximo uma vez por execução o que fazer, e a resposta SHALL se aplicar a todas as tabelas pendentes de uma vez, nunca tabela a tabela.

#### Scenario: Resumo antes da pergunta

- **WHEN** três tabelas ficam sem chave confirmada, duas delas com candidato composto
- **THEN** o comando relata as três tabelas, os dois candidatos compostos, e a convenção de nome de PK — antes de perguntar qualquer coisa

### Requirement: A coluna sintética é sempre nomeada pela convenção do schema, nunca digitada

Quando o operador escolher a recomendação de coluna sintética, o comando SHALL nomeá-la com o nome de primary key mais comum entre as tabelas do escopo que já têm uma chave de coluna única — o mesmo nome que qualquer outra tabela do schema já usa. O comando SHALL NOT aceitar um nome de coluna digitado livremente. Quando nenhum nome puder ser determinado a partir do schema, a opção de coluna sintética SHALL NOT ser oferecida.

#### Scenario: Coluna sintética segue a convenção

- **WHEN** o operador escolhe a recomendação de coluna sintética e o schema majoritariamente nomeia a chave primária de um jeito
- **THEN** toda tabela pendente resolvida por essa escolha ganha uma coluna sintética com esse nome, sem sondagem de dado

#### Scenario: Sem convenção, sem opção de coluna sintética

- **WHEN** nenhuma tabela do escopo tem chave primária de coluna única o suficiente para tabular uma convenção
- **THEN** o menu não oferece a recomendação de coluna sintética, só o candidato composto (quando existir) e pular

### Requirement: Composto e sintético são recomendações globais, aplicadas a toda tabela pendente

Ao escolher a recomendação de chave composta, o comando SHALL testar, por contagem completa, o candidato de cada tabela pendente que tiver um — nunca afirmado sem essa prova. Ao escolher a recomendação de coluna sintética, o comando SHALL aplicá-la a toda tabela pendente, independentemente de ela ter ou não um candidato composto.

#### Scenario: Chave composta confirmada pela escolha global

- **WHEN** o operador escolhe a recomendação de chave composta e a sondagem por contagem completa confirma unicidade para uma das tabelas pendentes
- **THEN** o achado dessa tabela é resolvido como chave composta confirmada, com o mesmo veredito `confirmed` que o caminho automático produz

#### Scenario: Pular não resolve nada

- **WHEN** o operador responde vazio ou o stdin fecha (EOF) antes de uma resposta válida
- **THEN** todo achado pendente permanece exatamente como o caminho automático o deixou, sem sugestão adicional

### Requirement: Uma resposta não reconhecida é reportada como inválida e perguntada de novo

O comando SHALL NOT tratar uma resposta que não corresponda a nenhuma opção oferecida como um pedido para pular. Ele SHALL informar que a resposta foi inválida e perguntar novamente, até receber uma resposta reconhecida ou o stdin fechar.

#### Scenario: Resposta inválida não pula silenciosamente

- **WHEN** o operador digita algo que não corresponde a nenhuma opção do menu
- **THEN** o comando informa que a resposta foi inválida e pergunta de novo, em vez de tratar a tabela como pulada

### Requirement: Coluna sintética nunca é afirmada como confirmada por dado

Uma sugestão de coluna sintética SHALL NOT carregar um veredito de sondagem: sua correção não depende de nenhum dado existente, só da criação da coluna. O artefato `.sql` gerado para ela SHALL declarar em duas etapas — criação da coluna, depois promoção a chave primária — e SHALL observar que a criação de uma coluna `GENERATED ALWAYS AS IDENTITY` já reescreve a tabela.

#### Scenario: Artefato de coluna sintética

- **WHEN** um achado tem uma sugestão de coluna sintética
- **THEN** `suggested_keys.sql` emite a criação da coluna e a promoção a chave primária em duas etapas comentadas, com a ressalva sobre reescrita da tabela
