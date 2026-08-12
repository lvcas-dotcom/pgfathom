## Why

Contra o schema de 226 tabelas medido esta semana, o `discover` levou nove segundos em silêncio. Contra o de 1.054 do GitLab, levou minutos — e o usuário não tem como saber se a ferramenta está trabalhando, esperando o servidor, ou travada. A primeira execução de alguém contra produção é justamente quando essa dúvida custa mais: a pessoa que autorizou está olhando por cima do ombro.

A informação já existe e já é medida. O pipeline nomeia cada estágio e devolve o tempo de cada um; a cobertura conta o funil de candidatos. Só não é dito enquanto acontece.

## What Changes

- A unidade de execução passa a relatar progresso a quem chamou: o estágio que começou e, na validação, quantos candidatos de quantos já terminaram.
- O comando desenha isso numa linha de stderr que se reescreve, e só quando stderr é terminal interativo — a mesma decisão tomada na fronteira do processo que já governa o realce.
- Em pipe, em arquivo, em CI, sob `NO_COLOR` ou `TERM=dumb`: nada. Nenhum byte a mais.
- Nenhuma dependência nova.

O progresso é **diagnóstico**, então vive em stderr, junto dos avisos. Em stdout ele corromperia o consumo programático, que é a regra que este projeto trata como inviolável.

## Capabilities

### Modified Capabilities

- `cli-foundation`: a execução passa a relatar progresso a quem a chama, e o comando passa a exibi-lo sob a mesma detecção de destino que governa o realce.

## Impact

Nenhuma dependência nova, nenhuma flag nova, nenhuma mudança de contrato. Nada em stdout muda, e por isso nenhum golden file muda.

A validação relata de dentro de um grupo concorrente, então o contador é atômico e o desenho é serializado — um progresso que embaralha a própria linha é pior do que progresso nenhum.

O risco é o progresso escapar para onde não deve: para stdout, ou para um destino que não é terminal. Contra isso, a decisão é tomada uma vez na fronteira e passada adiante, exatamente como o realce, e o teste verifica que um destino não interativo não recebe byte algum.
