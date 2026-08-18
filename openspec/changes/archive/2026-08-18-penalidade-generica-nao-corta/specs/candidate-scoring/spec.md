## MODIFIED Requirements

### Requirement: Sinais negativos e o que eles protegem

O sistema SHALL emitir sinais negativos para: nome de entidade genérico apontando para tabela pequena, tipicamente tabela de domínio como `status` ou `tipo`; múltiplas tabelas candidatas para o mesmo nome; e tipo compatível mas não idêntico.

O sinal de nome genérico existe porque essas relações costumam ser reais e pouco interessantes, e sem penalidade elas dominariam o relatório por volume, empurrando os achados valiosos para o fim.

Por isso ele SHALL afetar a ordenação e MUST NOT ser, sozinho, o que descarta um candidato. O limiar SHALL ser aplicado ao score do candidato calculado **sem** o sinal de nome genérico; o score reportado SHALL continuar contando a penalidade, para que a ordenação permaneça inalterada.

Os demais sinais negativos SHALL continuar entrando no corte. A distinção é o que cada um afirma: alvo ambíguo e tipo apenas compatível reduzem a probabilidade de a hipótese ser verdadeira, e um limiar de confiança é exatamente onde isso deve pesar. Nome genérico afirma que a relação é pouco interessante, não que seja provavelmente falsa — deixá-lo cortar transforma "chato" em "inexistente", e a relação descartada é justamente aquela cuja validação teria sido conclusiva.

Um candidato que já ficaria abaixo do limiar sem a penalidade SHALL continuar descartado, com o motivo registrado.

#### Scenario: Nome genérico com alvo pequeno é penalizado

- **WHEN** a coluna `status_id` casa com uma tabela `status` de poucas linhas
- **THEN** o candidato carrega o sinal de nome genérico e tem score reduzido

#### Scenario: Nome genérico não é motivo de descarte

- **WHEN** um candidato só fica abaixo do limiar por causa do sinal de nome genérico
- **THEN** ele segue para validação, e seu score reportado continua descontando a penalidade

#### Scenario: A penalidade continua ordenando

- **WHEN** um candidato penalizado por nome genérico e outro sem penalidade são reportados
- **THEN** o penalizado aparece depois, com score estritamente menor

#### Scenario: A isenção não resgata candidato fraco

- **WHEN** um candidato ficaria abaixo do limiar mesmo sem o sinal de nome genérico
- **THEN** ele é descartado antes da validação, com o motivo registrado

#### Scenario: Ambiguidade é penalizada

- **WHEN** um nome de entidade casa com mais de uma tabela
- **THEN** todos os candidatos resultantes carregam o sinal de alvo ambíguo e têm score reduzido

#### Scenario: Ambiguidade continua cortando

- **WHEN** um candidato fica abaixo do limiar por causa do sinal de alvo ambíguo
- **THEN** ele é descartado antes da validação, com o motivo registrado

### Requirement: Corte por limiar configurável com motivo registrado

Candidatos com score abaixo de um limiar configurável SHALL ser descartados antes de qualquer validação, e cada descarte SHALL registrar o motivo.

Este corte é o único mecanismo que impede a fase de validação de disparar milhares de anti-joins contra um banco de produção, e por isso o limiar precisa ser ajustável sem recompilar.

O padrão SHALL ser conservador o bastante para que uma execução em schema de centenas de tabelas produza um conjunto de candidatos que caiba numa revisão humana.

O score comparado ao limiar SHALL ser o score de corte definido no requisito de sinais negativos — o do candidato sem o sinal de nome genérico —, e não necessariamente o score reportado. Os dois SHALL usar a mesma regra de saturação, sob um único dono, para que continuem comparáveis contra o mesmo limiar.

#### Scenario: Abaixo do limiar é descartado

- **WHEN** um candidato pontua abaixo do limiar configurado
- **THEN** ele não segue para validação, e o motivo do descarte fica registrado

#### Scenario: Descartes são reportáveis

- **WHEN** o usuário pede para ver os descartados
- **THEN** eles aparecem com score e motivo, para que ninguém se pergunte por que uma coluna óbvia foi ignorada

#### Scenario: Limiar é ajustável

- **WHEN** o usuário fornece um limiar diferente do padrão
- **THEN** o conjunto de candidatos sobreviventes muda de acordo
