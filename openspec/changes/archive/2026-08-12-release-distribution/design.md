## Context

A ferramenta já compila para as quatro plataformas alvo sem cgo — `make crosscheck` prova isso desde a fase 1, e a ausência de cgo foi requisito de produto justamente para que este momento fosse barato. O que falta é empacotar, publicar e dizer de onde veio.

O leitor deste release não é o usuário médio de CLI. É alguém que vai apontar um binário desconhecido para um banco de produção, depois de convencer outra pessoa a autorizar. Tudo que reduza o que ele precisa aceitar em confiança vale mais aqui do que valeria em outro projeto.

## Goals / Non-Goals

**Goals**

Binário para as quatro plataformas, com checksum e com carimbo de procedência que o próprio binário sabe dizer. Imagem de contêiner que funcione contra servidor com TLS. Procedimento escrito, incluindo as partes que não moram neste repositório.

**Non-Goals**

Assinatura criptográfica de artefato e SBOM. Ambos são defensáveis para este público e ambos acrescentam ferramenta externa e chave para custodiar; ficam para quando houver quem os consuma, e a ausência é declarada em vez de silenciosa.

Publicação no Docker Hub, que exigiria credencial nova. `ghcr.io` usa o token que o workflow já tem.

Nenhuma mudança de código de produto. Um release que precise mexer no que está sendo lançado não é um release.

## Decisions

### O carimbo tem duas fontes, então tem um teste

O Makefile injeta `Version`, `Commit` e `Date` em `internal/buildinfo` por `-ldflags`. O goreleaser vai injetar os mesmos três, pelo seu próprio caminho. São duas configurações que precisam concordar sobre três nomes de variável, e duas configurações que precisam concordar sobre nomes divergem quando alguém renomeia um deles.

O modo de falha é discreto e chega inteiro no usuário: o binário publicado responde `unknown` quando perguntado, e o relatório JSON sai com `tool_version` vazio — o campo que o `check --baseline` de amanhã vai usar para saber o que produziu a linha de base.

Contra isso, a verificação óbvia: construir por snapshot no CI, executar o binário resultante e falhar se ele não souber quem é. É barato, roda em toda mudança e pega exatamente o erro que ninguém revisa.

### A imagem carrega certificados raiz, e isso não é detalhe

A ferramenta abre conexão com o servidor do usuário, e a recomendação para produção é `sslmode=verify-full`. Num contêiner sem CA raiz, isso falha com uma mensagem sobre certificado desconhecido que parece problema do servidor.

A base é `distroless/static`, que traz os certificados e roda como usuário sem privilégio. Um `scratch` seria menor e transformaria a primeira execução com TLS num relatório de bug.

### O tap é dependência externa, e o documento diz isso antes do workflow tentar

Homebrew exige repositório separado — `homebrew-tap` — e um token com permissão de escrita nele. Nenhuma das duas coisas se cria a partir deste repositório, e um workflow que falhe na última etapa depois de já ter publicado binários deixa um release pela metade.

Então a configuração fica escrita e o pré-requisito fica no `docs/RELEASING.md`, na frente e não no rodapé. Sem o token, o release sai sem Homebrew, o que é um release completo com um canal a menos.

### A suíte roda antes de publicar, e publicar é irreversível

Tag apagada continua no cache de quem já baixou, e versão republicada com conteúdo diferente é a coisa que mais rápido destrói confiança em ferramenta de infraestrutura.

O workflow roda a suíte unitária e o lint antes de o goreleaser tocar em qualquer coisa. A suíte de integração não entra nesse caminho: ela depende de Docker dentro do runner e leva minutos, e o que ela protege já foi protegido no merge que produziu o commit sendo lançado.

## Risks / Trade-offs

**Publicar artefato sem carimbo** → snapshot construído e executado no CI, falhando quando o binário não sabe a própria versão.

**Imagem que não conecta com TLS** → base com CA raiz, e a razão escrita ao lado da escolha para que ninguém a "otimize" para `scratch` depois.

**Release pela metade por dependência externa faltando** → o Homebrew é o último passo e é opcional; a ausência do token custa um canal, não o release.

**Ausência de assinatura** → declarada no documento de release, com checksum publicado como o que existe hoje. Quem exigir mais tem onde ler que ainda não há.

**Tag errada** → o procedimento é escrito, e a etapa que menos perdoa — a tag — é a que o documento cobre com mais cuidado.

## Migration Plan

Não aplicável: é o primeiro release. O contrato JSON já nasceu na versão 1 e a change de chave composta foi feita antes exatamente para que ele nascesse na forma definitiva.

## Open Questions

Nenhuma bloqueante. Assinatura e SBOM ficam registrados como ausências conhecidas, não como esquecimentos.
