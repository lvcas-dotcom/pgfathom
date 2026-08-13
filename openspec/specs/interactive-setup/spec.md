# interactive-setup Specification

## Purpose
A definir - criado ao arquivar alteração setup-guide. Atualize o Purpose após o arquivamento.
## Requirements
### Requirement: O guia compõe um comando e o imprime

O subcomando de guia SHALL perguntar conexão, escopo, modo de validação e destino dos artefatos, e SHALL imprimir o comando `discover` que as respostas compuseram, com as flags explícitas.

O comando impresso SHALL ser reproduzível: executá-lo diretamente SHALL produzir a mesma execução que o guia produziria.

O objetivo não é poupar o usuário das vinte e uma flags, é ensiná-las. Quem vê o comando que as próprias respostas montaram copia, guarda e não precisa do guia de novo. É também o que mantém o guia honesto: nada acontece que a pessoa não possa reproduzir sozinha, e nenhuma opção fica escondida atrás de uma pergunta amigável.

#### Scenario: O comando sai completo

- **WHEN** o guia termina de coletar as respostas
- **THEN** ele imprime o comando `discover` correspondente, com as flags que as respostas determinaram

#### Scenario: O comando impresso é o que foi executado

- **WHEN** o guia executa ao final
- **THEN** o que ele executa é exatamente o comando que imprimiu

### Requirement: O guia mostra antes de agir, e pergunta

O guia SHALL exibir o comando composto e SHALL pedir confirmação explícita antes de executá-lo. Sem confirmação, ele MUST NOT executar nada.

Este projeto emite DDL comentada para revisão mesmo quando a relação foi confirmada linha a linha. Um guia que executasse por conta própria contra um banco de produção contradiria a única postura que a ferramenta mantém em todo o resto.

#### Scenario: Recusa não executa

- **WHEN** o usuário não confirma
- **THEN** nada é executado, e o comando impresso permanece disponível para uso posterior

#### Scenario: Confirmação executa o que foi mostrado

- **WHEN** o usuário confirma
- **THEN** a execução é a do comando exibido, sem opção adicional que não tenha sido mostrada

### Requirement: O escopo é escolhido a partir do que o servidor tem

O guia SHALL listar os schemas visíveis ao papel conectado, com o número de tabelas de cada um, e SHALL permitir escolher um ou mais.

A leitura SHALL usar a mesma conexão somente leitura de sempre.

O modo de falha mais comum de uma primeira execução não é erro, é apontar para o schema errado e concluir que o banco não tinha nada a descobrir. Um servidor com dezenas de schemas e poucas tabelas no padrão é o normal em sistema de gestão pública, e a informação que desfaz o engano é uma linha por schema com a contagem.

#### Scenario: Os schemas aparecem com tamanho

- **WHEN** o guia pergunta o escopo
- **THEN** ele lista os schemas visíveis com o número de tabelas de cada um

#### Scenario: O padrão não é imposto

- **WHEN** o schema padrão tem menos tabelas que outro visível
- **THEN** a escolha continua com o usuário, e a lista mostra a diferença

### Requirement: O guia exige terminal interativo

O guia SHALL verificar, antes de perguntar qualquer coisa, que entrada e saída são terminal interativo, e SHALL recusar com mensagem clara quando não forem.

Um guia interativo em pipe ou em CI espera entrada que nunca chega e trava o processo. Diferente do realce e do progresso, aqui não há modo degradado: sem terminal, não há guia.

#### Scenario: Sem terminal, recusa explicando

- **WHEN** o guia é invocado com entrada ou saída que não são terminal
- **THEN** ele falha com mensagem dizendo que o comando exige terminal, e sugere o `discover` direto

### Requirement: O guia veste a mesma convenção de cor do relatório

O guia SHALL usar a paleta do projeto com os mesmos papéis que o relatório: `#e2483d` onde a atenção vai, `#7ec9b8` para o que ficou assentado, `#8c7d73` para o que apoia sem competir.

Diferente do relatório, o guia SHALL adaptar os tons claros ao fundo do terminal, porque tem alguém olhando e nada nele é fixado por golden file.

#### Scenario: Uma escolha marcada se lê como assentada

- **WHEN** um item é marcado numa lista de múltipla escolha
- **THEN** a marca aparece na mesma cor que o relatório usa para relação confirmada

#### Scenario: Fundo claro não engole o texto

- **WHEN** o guia roda em terminal de fundo claro
- **THEN** os títulos usam a contraparte escura da cor clara da paleta, e permanecem legíveis

