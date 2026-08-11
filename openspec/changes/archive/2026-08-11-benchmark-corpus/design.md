## Context

O roadmap descreve o corpus como cinco schemas públicos completamente anotados com chave estrangeira: GitLab, Odoo, Discourse, Redmine e Mastodon. Medido, o pressuposto não se sustenta.

| Projeto | Schema publicado | Tabelas | FKs |
|---|---|---:|---:|
| GitLab | `db/structure.sql` | 1.426 | 1.857 |
| Discourse | `db/structure.sql` | 354 | 23 |
| Mastodon | só `schema.rb`, DSL Ruby | — | — |
| Redmine | só migrations | — | — |
| Odoo | schema criado pelo ORM na instalação | — | — |

Dois fatos saem daí. Só dois dos cinco publicam SQL carregável; os outros exigem subir a aplicação, rodar migrations e extrair o schema, que é outro mecanismo e outra ordem de custo. E o Discourse, com 354 tabelas, declara 23 chaves — Rails clássico põe a integridade na aplicação. Ele quase não serve para medir recall.

O que sobra é melhor do que parece. O GitLab sozinho tem quase o dobro das chaves do maior banco privado já medido, e o Discourse responde uma pergunta que o GitLab não responde: o que a ferramenta propõe num schema que quase não declara nada.

## Goals / Non-Goals

**Goals**

Um número reproduzível por terceiros, medido no mesmo caminho que o usuário executa. A decomposição que o README exige: quanto o nome recupera sozinho, quanto a detecção acrescenta, quanto a evidência de uso acrescenta. O custo por etapa, o funil de candidatos e o número de queries de validação, que nunca saíram de estimativa. E a calibração dos limiares que sete fases declararam como provisórios.

**Non-Goals**

Veredito no corpus público, que não tem dados. Geração de dados sintéticos. Os três schemas que exigem subir aplicação — o manifesto abre a porta, esta change não a atravessa. Empacotamento e release, que são a change seguinte.

## Decisions

### O harness é um teste, não um binário

Ele precisa de Docker, precisa das mesmas fixtures e helpers de contêiner que a suíte de integração já tem, e não pode existir no que se distribui. Um `cmd/` a mais apareceria em `go install ./...`, teria que ser excluído à mão no `goreleaser`, e uma etiqueta de build sobre o único arquivo de um pacote `main` quebra `go build ./...`.

Como teste sob `//go:build benchmark`, ele fica invisível para todo build normal sem nenhuma exceção configurada em lugar nenhum, e herda `-run`, o registro de saída e o ciclo de vida de contêiner de graça. O custo é uma impropriedade de vocabulário: `go test` roda algo que não é teste. Vale o preço.

### Buscar e medir são alvos separados

`make corpus` baixa e confere o checksum. `make benchmark` mede, e quando o corpus não está lá falha dizendo qual comando resolve.

Rede dentro da medição contaminaria duas coisas ao mesmo tempo: o tempo medido passaria a incluir o que a operadora fez naquela tarde, e uma queda de rede viraria falha de benchmark. Separados, a medição roda offline e quantas vezes se quiser sobre exatamente os mesmos bytes.

### O manifesto versiona a receita, nunca o dump

Cada entrada carrega nome, tipo de aquisição, URL, commit e `sha256`. O download vai para diretório ignorado.

Vendorizar os dumps custaria vários megabytes num repositório cujo argumento de venda é ser auditável numa sentada — o `structure.sql` do GitLab sozinho tem 2,8 MB. Com commit fixado e checksum, a reprodução é exata sem que o repositório engorde, e um upstream que reescreva a história é detectado em vez de silenciosamente medido.

O tipo de aquisição existe desde a primeira entrada, com um valor só usado. É o que faz um schema que exija subir a aplicação entrar depois como linha nova em vez de reescrita.

### O gabarito é o que foi derrubado, e o harness é quem derruba

O procedimento é: carregar o schema, ler as chaves declaradas, derrubar todas, medir.

Isso põe escrita no banco dentro de um projeto cuja primeira regra é não emitir nada que altere. As duas coisas não se tocam, e a separação precisa estar visível no código em vez de combinada num comentário: quem derruba é o harness, pela conexão dele, contra um contêiner descartável que ele mesmo subiu. A conexão que o `pgfathom` recebe continua sendo a de sempre, com as políticas de sessão de sempre, e o teste que prova que uma escrita por ela falha continua valendo.

### Recall é casamento exato, e precisão não é medida contra o gabarito

Um candidato conta como recuperado quando a chave filha e a chave pai batem com uma chave derrubada, coluna a coluna, na ordem. Sem crédito parcial: metade de uma chave não é meia recuperação, é outra proposta.

O caminho inverso não fecha. Um candidato que não bate com nenhuma chave derrubada **não é** um falso positivo — pode ser uma relação real que aquele schema nunca declarou, que é literalmente o produto desta ferramenta. Então o relatório publica a contagem dos não casados como o que ela é, uma contagem de propostas fora do gabarito, e nunca como erro. Chamar isso de precisão seria contar como defeito exatamente o que se está vendendo.

### O perfil de cada schema é declarado, não ajustado

O manifesto diz qual perfil embarcado cada entrada usa, e o relatório publica o valor junto do número.

A fase 3.5 estabeleceu a regra: um corpus rodado com perfil ajustado à mão não representa a primeira execução de ninguém. Escolher `en` para um schema em inglês não é ajuste, é a flag que o usuário desse schema passaria; inventar afixo para o corpus casar seria. A diferença é que a primeira escolha cabe numa coluna da tabela publicada e a segunda não caberia em lugar nenhum.

### A calibração vem depois da primeira medição, e cada valor muda com o número ao lado

Limiar de score, margem do pré-filtro, limiares de veredito e pesos de aridade entraram como estimativa. A ordem aqui importa: medir primeiro com os valores atuais, publicar essa linha de base, e só então mexer — cada mudança justificada pelo que o corpus mostrou, e com o antes e o depois no registro.

Calibrar antes de publicar a linha de base transformaria o corpus num ajuste de curva sobre si mesmo, e o número resultante não diria mais nada sobre a primeira execução de ninguém.

## Risks / Trade-offs

**Publicar um número que mede outra coisa** — o risco central → o harness executa `discovery.Run` em processo, sem reimplementar a orquestração, e todo recorte que aplica aparece no relatório: schema fora de escopo, chave apontando para fora do corpus, tabela sem privilégio, candidato que estourou o teto de tempo.

**Um schema do corpus deixa de carregar num Postgres limpo** — o `structure.sql` do GitLab quer `btree_gist` e `pg_trgm`, cria dois schemas próprios e tem 101 tabelas particionadas → a versão do servidor é declarada por entrada no manifesto, acima do piso da suíte de integração, e uma carga que falhe é falha de benchmark com a mensagem do servidor, nunca uma linha faltando na tabela em silêncio.

**O recall do Discourse vai parecer péssimo** — 23 chaves em 354 tabelas dão um denominador minúsculo, onde cada acerto vale quatro pontos percentuais → publicar assim mesmo, com o denominador ao lado, e dizer na tabela o que aquela linha mede. Um schema que quase não declara integridade é um dado sobre o ecossistema, não um constrangimento a esconder.

**O tempo por etapa mede a máquina de quem rodou** → a tabela publica servidor, imagem e versão da ferramenta junto dos números, e eles servem para comparar etapas entre si e execuções da mesma máquina entre si, não para prometer desempenho.

**O corpus vira um alvo** — otimizar contra dois schemas em inglês enviesaria a ferramenta para eles → a decomposição por origem de sinal é o que torna o enviesamento visível, e as medições contra bancos privados em português continuam publicadas ao lado.

## Migration Plan

Não aplicável. Nada muda para quem usa o binário: o harness não existe em nenhum build que não passe `-tags=benchmark`.

## O que a linha de base disse sobre os limiares

Registrado depois da primeira medição, que é a ordem que esta change se impôs.

**Nenhum limiar muda, e o número diz por quê.** No GitLab, de 1.655 candidatos gerados, exatamente **um** ficou abaixo do corte de score. O limiar não é o que limita o recall ali: o teto é a geração, que não produz 727 das 1.857 chaves do gabarito. Baixar o corte não pode recuperar o que nunca foi proposto, e subi-lo só cortaria acertos.

**O pré-filtro não opina num corpus sem dados**, e não poderia: rejeitou 0 e registrou "sem estatística" para os 1.654 candidatos, porque tabela vazia nunca analisada não tem `n_distinct`. Mede-se aqui que a camada se abstém corretamente, não a margem dela.

**Os limiares de veredito continuam sem medição**, pela mesma razão que nenhum veredito é medido: não há linhas. Eles seguem como estimativa declarada, agora com o lugar onde seriam calibrados nomeado — o dump real, quando existir.

**Os pesos de aridade também seguem sem medição**, porque nenhuma chave composta foi recuperada. Calibrar um peso a partir de zero acerto seria ajustar um número contra ruído.

## Open Questions

**A detecção de nomenclatura é medida sem a entrada dela, e isso é estrutural.** Ela deriva o afixo de referência lendo as chaves que o schema já declara, e o procedimento derruba todas antes de medir. A linha `+ detecção` do corpus descreve, então, um schema que não declara integridade nenhuma — o caso mais difícil e um caso real, mas não o mesmo em que a detecção mediu +78 pontos contra bancos privados que ainda tinham 470 chaves. Não há como o corpus fazer as duas perguntas: quem derruba o gabarito remove a evidência. O relatório declara isso; o que fica em aberto é se vale acrescentar uma configuração que alimente a detecção com o conjunto pré-derrubada, sabendo que ela mediria uma execução que ninguém faz.

**A detecção chegou a custar recall.** No Discourse ela baixou de 11 para 10 recuperadas. Sem chave declarada para aprender, sobra a detecção de prefixo de tabela por frequência, e um prefixo removido a mais estraga o casamento. É achado do corpus, é pequeno em valor absoluto, e a resposta certa provavelmente é a detecção se abster quando não há chave declarada de onde aprender — o que é mudança de comportamento e não cabe nesta change.

Nenhuma outra bloqueante. Se a primeira medição mostrar que o custo de subir o GitLab a cada execução domina o tempo, a resposta é cachear o contêiner carregado entre configurações — mas isso é otimização, e otimizar antes de ter o número seria decidir sem saber.
