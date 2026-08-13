## ADDED Requirements

### Requirement: A execução relata progresso a quem a chama

A unidade de execução SHALL relatar, para quem a chamou, o estágio que começou. Na validação, SHALL relatar também quantos candidatos de quantos já terminaram.

O relato SHALL ser uma função recebida por opção, e a unidade MUST NOT escrever progresso em stream algum por conta própria. Quem executa a mesma unidade para medir — o harness de benchmark — precisa contar sem que nada seja desenhado.

Estágio sem denominador conhecido antes de terminar SHALL relatar apenas que começou. Progresso com denominador inventado afirma saber quanto falta.

#### Scenario: Cada estágio anuncia o início

- **WHEN** uma execução atravessa os estágios
- **THEN** quem chamou recebe um relato por estágio, na ordem de execução

#### Scenario: A validação relata contagem

- **WHEN** a validação atravessa os candidatos
- **THEN** os relatos trazem quantos terminaram e quantos são no total

#### Scenario: Nada é escrito pela unidade

- **WHEN** a unidade é executada sem função de progresso
- **THEN** nenhum byte é escrito em stream algum por causa de progresso

### Requirement: O progresso é diagnóstico, e só aparece em terminal interativo

O comando SHALL exibir o progresso em stderr, nunca em stdout.

A exibição SHALL ocorrer somente quando stderr for terminal interativo, e SHALL ser suprimida quando `NO_COLOR` estiver definido ou o terminal for `dumb`. A decisão SHALL ser tomada uma vez, na fronteira do processo, e passada adiante como valor.

Progresso em stdout corromperia o consumo programático. Progresso num destino que não é terminal encheria log e arquivo de linhas que existiam para ser sobrescritas.

`NO_COLOR` desliga o progresso apesar de a linha não ser cor: quem define a variável está pedindo um stream sem sequências de escape, e reescrever linha é sequência de escape.

#### Scenario: Pipe não recebe progresso

- **WHEN** stderr é redirecionado para arquivo ou canalizado
- **THEN** nenhuma linha de progresso é escrita

#### Scenario: `NO_COLOR` suprime

- **WHEN** `NO_COLOR` está definido e stderr é terminal
- **THEN** nenhuma linha de progresso é escrita

#### Scenario: Aviso não é atropelado

- **WHEN** um estágio emite aviso durante a execução com progresso visível
- **THEN** o aviso aparece em linha própria, sem resto da linha de progresso
