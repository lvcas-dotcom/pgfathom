## Context

Esta change não entrega funcionalidade. Ela existe porque a próxima fase precisa de duas coisas que a estrutura atual não oferece: um lugar para medir cada etapa, e uma forma de rodar a execução completa sem passar pela linha de comando.

O código está saudável — sete fases sem reescrita, camadas com dependência em sentido único, cada uma testável isolada. O que faltou foi a peça de composição: alguém que junte as camadas e se chame de "uma execução". Esse papel foi ocupado por acidente pela função do comando, que era o lugar mais próximo quando a fase 2 tinha dois estágios e ainda parecia razoável quando tinha quatro.

## Goals / Non-Goals

**Goals**

Extrair a execução para uma unidade com fronteira e entrada programática. Dar nome a cada etapa, para que a fase 8 meça sem reabrir esta camada. Eliminar as duplicações que já divergem. Provar, pela suíte existente, que nada mudou.

**Non-Goals**

Nenhuma medição agora — esta change cria o lugar, a fase 8 põe os números. Nenhuma mudança em flag, saída, veredito ou artefato. Nenhuma reorganização das camadas existentes: `catalog`, `infer`, `stats`, `validate`, `sqlprobe` e `report` ficam exatamente onde estão, fazendo exatamente o que fazem.

## Decisions

### Uma unidade de composição, não um framework de pipeline

A tentação em refactor assim é generalizar: registro de estágios, interface comum, encadeamento configurável. Seria errado. Os estágios não são intercambiáveis — cada um tem assinatura própria, regime de erro próprio, e a ordem entre eles é semântica, não configuração. Um pipeline genérico esconderia isso atrás de uniformidade falsa e tornaria mais difícil ler a única coisa que importa: o que acontece com um candidato, em ordem.

A extração é literal. A sequência continua legível de cima para baixo, agora numa camada que pode ser chamada de um teste, de um harness de benchmark ou de um comando.

### Erro por etapa mantém o regime que cada fase escolheu

Três estágios já degradam em vez de falhar: evidência de uso, pré-filtro estatístico e verificação de privilégio. Isso é decisão de produto — sinal perdido não é resposta errada — e foi registrado no design da fase que o introduziu.

A extração preserva o regime de cada um, e o torna explícito: a unidade recebe como avisa, em vez de escrever em `stderr` por conta própria. Quem chama do CLI encaminha para o usuário; quem chama do benchmark contabiliza. Nenhum dos dois precisa saber que a decisão de degradar foi tomada três fases atrás.

### O dono da estimativa de linhas é o modelo

`tableRows` existe em `stats` e em `validate`, com a mesma regra e tipos de retorno diferentes. A regra não é trivial: desde o PostgreSQL 14, `reltuples` devolve `-1` para tabela nunca analisada, e tratar isso como contagem faz a tabela parecer vazia — o que silenciosamente muda pontuação em uma camada e amostragem na outra.

Uma regra dessas com duas implementações é uma divergência esperando acontecer. Ela passa a morar em `model`, ao lado da sentinela que interpreta, e as duas camadas consomem a mesma coisa.

### Uma lista de valores plantados, no lugar de quatro

A varredura de vazamento é o mecanismo que faz cumprir a regra mais dura do projeto, e hoje depende de quatro listas manuais que precisam crescer juntas a cada fixture nova. O modo de falha é o pior possível: a cópia desatualizada não acusa, ela **passa** — e um teste verde que não olhou é indistinguível de um teste verde que olhou.

A lista passa a ser única, em `testutil`, junto do helper que faz a varredura. Fixture nova acrescenta seus valores num lugar só, e toda varredura do projeto passa a conhecê-los no mesmo commit.

### Interface estreita nas quatro camadas de I/O

`sqlprobe` e `stats` já declaram a interface mínima que consomem; `catalog` e `validate` recebem `*db.Pool` concreto. Interface declarada no consumidor é idiomático em Go — o problema aqui é só a metade que não seguiu, e que por isso não pode ser exercitada sem um servidor.

Uniformizar não muda o que essas camadas fazem; abre a porta para testar seus caminhos de erro — privilégio negado, timeout, resultado malformado — sem subir contêiner. Esta change não escreve esses testes; ela remove o motivo de não existirem.

## Risks / Trade-offs

**Regressão silenciosa é o risco inteiro desta change** → por isso ela não ganha teste de comportamento próprio. A suíte existente é o critério: golden files fixam a saída byte a byte, e os testes de integração fixam veredito, contagem de órfãos e conteúdo dos artefatos. Se algo passar diferente, o teste que falha é o certo.

**Uma camada a mais entre comando e trabalho** → é a camada que já existia implicitamente; ela apenas deixa de ser invisível. O custo é um arquivo a mais para abrir ao seguir o fluxo; o ganho é o fluxo ser legível sem cobra em volta.

**A extração pode arrastar mudança de comportamento sem querer** — a ordem em que os avisos saem, por exemplo → os testes de CLI já verificam disciplina de streams, e os golden files cobrem o resto da saída.

## Migration Plan

Não aplicável. Nada muda para quem usa o binário.

## Open Questions

Nenhuma.
