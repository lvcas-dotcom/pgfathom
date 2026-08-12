## Context

O release atual entrega binário estático para quatro plataformas, imagem de contêiner e cask de Homebrew. É bastante, e não cobre o caminho mais curto do usuário-alvo: `apt install` ou `dnf install` de um arquivo baixado.

A distância entre "existe binário para Linux" e "existe pacote para Linux" não é técnica, é de expectativa. Quem administra servidor espera que o binário caia em `/usr/bin`, que a licença esteja em `/usr/share/doc`, e que `pgfathom <TAB>` complete. Um `.tar.gz` entrega o primeiro item e deixa os outros dois como tarefa de casa.

## Goals / Non-Goals

**Goals**

Pacote instalável nas duas famílias de distribuição e nas duas arquiteturas que já se compila. Conteúdo nos caminhos que a convenção define. Autocompletar que não pode divergir das flags.

**Non-Goals**

Repositório apt ou yum, que exige hospedagem e chave de assinatura — outra ordem de compromisso, e não é o que falta para o primeiro contato.

Página de manual. Ela exigiria uma dependência nova só para converter markdown em roff, e o `--help` do cobra já cobre o mesmo terreno. Fica registrado como ausência, não como esquecimento.

Nenhuma mudança em código de produto.

## Decisions

### O autocompletar é gerado do comando, não escrito à mão

O cobra já expõe `pgfathom completion <shell>`. O build roda esse comando por `go run` antes de empacotar, e o resultado entra no pacote.

Escrever os arquivos à mão os deixaria desatualizados no primeiro release que acrescentasse uma flag — e são 21 flags só no `discover`. Gerando, a divergência é impossível por construção: o autocompletar de um release descreve as flags daquele release.

`go run` em vez do binário recém-compilado porque o hook que prepara o contexto roda antes de existir binário. Compilar duas vezes custa segundos e evita uma ordenação frágil.

### Os caminhos são os da convenção, não os nossos

Binário em `/usr/bin/pgfathom`. Licença e README em `/usr/share/doc/pgfathom/`. Autocompletar em `/usr/share/bash-completion/completions/`, `/usr/share/zsh/site-functions/` e `/usr/share/fish/vendor_completions.d/`.

Nenhum deles é escolha nossa, e é isso que os torna certos: quem instala um pacote espera encontrar as coisas onde o sistema as guarda. Um pacote que inventa layout é um `.tar.gz` com metadados.

### Sem repositório, e dito em voz alta

Instalar por arquivo baixado é `dpkg -i` ou `rpm -i`, e não atualiza sozinho. Um repositório resolveria isso e custaria servidor mais chave de assinatura sob custódia — compromisso que não se assume de passagem, e que este projeto ainda não tem quem consuma.

O procedimento de release diz isso onde alguém vai procurar, junto das outras ausências declaradas.

## Risks / Trade-offs

**Pacote que instala em lugar errado** → os caminhos vêm da convenção, e a verificação lista o conteúdo do pacote em vez de supô-lo.

**Autocompletar que divergiu das flags** → impossível por construção: é gerado do comando no mesmo build.

**Pacote não atualiza sozinho** → verdade, e é o preço de não hospedar repositório. Declarado no procedimento de release.

**Mais quatro artefatos por release** → dois formatos por duas arquiteturas, montados pelo que já está embutido no goreleaser, sem etapa nova no workflow.

## Migration Plan

Não aplicável: canal novo, nada muda para quem já instalou por outro caminho.

## Open Questions

Nenhuma. Página de manual e repositório apt/yum ficam registrados como ausências conhecidas.
