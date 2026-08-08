## ADDED Requirements

### Requirement: Apresentação agrupa por veredito com quebradas primeiro

O relatório de `discover` SHALL agrupar os candidatos por veredito e SHALL apresentar os grupos nesta ordem: quebradas, confirmadas, fracas, não validadas. Rejeitadas MUST NOT aparecer a menos que a exibição de descartados tenha sido pedida explicitamente.

Cada grupo SHALL declarar sua contagem no cabeçalho, inclusive quando for zero — um grupo ausente é indistinguível de um grupo não avaliado.

A ordem contraria a confiança de propósito. Confirmada é higiene que espera a próxima janela; quebrada é integridade violada em produção, e é o achado que a ferramenta existe para produzir.

#### Scenario: Quebradas encabeçam o relatório

- **WHEN** uma execução produz candidatos quebrados e confirmados
- **THEN** o grupo de quebradas aparece antes do de confirmadas na saída

#### Scenario: Rejeitadas ficam fora por padrão

- **WHEN** uma execução produz candidatos rejeitados e a exibição de descartados não foi pedida
- **THEN** eles não aparecem individualmente, e o rodapé informa quantos foram e como vê-los

### Requirement: Cada linha de candidato carrega a métrica que sustenta seu veredito

Para cada candidato validado, a linha SHALL exibir a relação, a contenção por linha, a contenção por valor distinto, as contagens de órfãos, as linhas examinadas e o método de validação.

Candidato sem validação SHALL exibir os sinais que produziram seu score, e o motivo pelo qual não foi validado quando houver.

Nenhuma linha MUST afirmar um veredito sem a métrica ao lado. Veredito sem número é opinião.

#### Scenario: Candidato quebrado mostra as duas contenções e as duas contagens de órfão

- **WHEN** um candidato recebe veredito quebrada
- **THEN** a linha traz contenção por linha, contenção por valor, órfãos por linha, órfãos por valor, linhas examinadas e método

#### Scenario: Candidato não validado explica a ausência

- **WHEN** um candidato ficou sem validação por estouro de tempo
- **THEN** a linha registra o motivo, e não uma métrica em branco

### Requirement: Cabeçalho declara a procedência da execução

O cabeçalho SHALL declarar a versão da ferramenta, a versão do servidor analisado, o perfil de nomenclatura ativo, o limiar de score e o modo de validação executado.

Um relatório de inferência não é interpretável sem saber qual convenção o produziu. Quem discorda de um resultado precisa desses cinco valores para reproduzi-lo ou para contestá-lo.

#### Scenario: A procedência sai em toda execução

- **WHEN** `discover` renderiza em terminal
- **THEN** versão da ferramenta, versão do servidor, perfil, limiar e modo aparecem antes de qualquer achado

### Requirement: Rodapé fecha a conta e nunca omite o que não foi visto

O rodapé SHALL apresentar a contagem por veredito, o tempo total da execução e o bloco de cobertura completo — tabelas analisadas sobre o total, tabelas puladas com o motivo, funil do pré-filtro estatístico e candidatos que estouraram o tempo.

O bloco de cobertura SHALL aparecer mesmo quando não houve nenhum achado. Um relatório limpo precisa significar "olhei e está limpo", nunca "não consegui olhar".

#### Scenario: Cobertura sai mesmo sem achado

- **WHEN** uma execução não produz nenhum candidato acima do limiar
- **THEN** o rodapé ainda traz o bloco de cobertura e a contagem por veredito zerada

#### Scenario: Tabela pulada aparece com o motivo

- **WHEN** tabelas foram puladas por falta de privilégio ou por forma não suportada
- **THEN** o rodapé lista quantas e por quê

### Requirement: Modo amostrado sai com aviso destacado e inescapável

Quando qualquer validação da execução tiver sido amostrada, o relatório SHALL exibir, antes dos achados e novamente no rodapé, o aviso de que nenhum resultado ali é confirmado e de que as contagens de órfão são piso, não total.

O aviso SHALL ser realçado quando a saída aceitar realce.

Amostra limpa não é evidência de ausência: órfãos entram em lote e se agrupam nas mesmas páginas, que é o que amostragem por página é pior em encontrar. Um usuário que perde esse aviso lê triagem como conclusão.

#### Scenario: O aviso aparece duas vezes

- **WHEN** uma execução valida em modo amostrado
- **THEN** o aviso aparece no topo e no rodapé, e declara que só `--full` pode confirmar

#### Scenario: Modo completo não exibe o aviso

- **WHEN** uma execução valida em modo completo
- **THEN** o aviso de amostragem não aparece, e o cabeçalho declara que os vereditos são conclusivos

### Requirement: Realce respeita o destino e nunca chega a um pipe

O realce SHALL ser decidido uma única vez, na fronteira do processo, a partir da detecção de terminal já existente. O renderizador SHALL receber essa decisão como parâmetro e MUST NOT consultar ambiente por conta própria.

Quando o realce estiver desligado, a saída MUST NOT conter nenhuma sequência de escape ANSI.

#### Scenario: Saída canalizada é limpa

- **WHEN** a saída de `discover` é canalizada para arquivo ou para outro processo
- **THEN** nenhum byte de escape ANSI aparece no resultado

#### Scenario: O renderizador não decide sozinho

- **WHEN** o renderizador é chamado diretamente em teste com realce desligado
- **THEN** a saída é idêntica à do processo com a saída canalizada, independentemente do ambiente do teste

### Requirement: A formatação é fixada por golden file com entrada determinística

O conteúdo renderizado SHALL ser coberto por golden files, alimentados por um `Result` construído com versão, timestamp e duração fixos.

A normalização MUST estar na entrada do teste, nunca na saída do renderizador: o renderizador não pode ganhar caminho de código que exista apenas para tornar o teste possível.

Os cenários fixados SHALL cobrir, no mínimo: execução com os quatro vereditos presentes, execução sem nenhum achado, execução em modo amostrado, e execução com cobertura incompleta.

#### Scenario: Mudança de formatação vira diff revisável

- **WHEN** a formatação do relatório muda sem que o golden seja atualizado
- **THEN** o teste falha apontando a primeira linha divergente

#### Scenario: O renderizador não tem modo de teste

- **WHEN** o código de renderização é inspecionado
- **THEN** não existe ramo condicional cuja única razão de ser é o golden file
