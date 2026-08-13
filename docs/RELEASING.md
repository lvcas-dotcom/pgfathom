# Como sair um release

Publicar é irreversível. Tag apagada continua no cache de quem já baixou, e uma
versão republicada com conteúdo diferente é a coisa que mais rápido destrói a
confiança numa ferramenta que as pessoas apontam para produção. O procedimento
abaixo existe para que a etapa que menos perdoa — a tag — seja a última.

## Antes do primeiro release

Duas coisas moram fora deste repositório e ninguém as cria a partir daqui.

**O tap do Homebrew.** Precisa existir um repositório público
`lvcas-dotcom/homebrew-tap`, com pelo menos um commit — repositório sem branch
padrão não tem onde o goreleaser escrever, e ele falharia no último passo. O
prefixo `homebrew-` no nome é o que faz `brew` resolver `lvcas-dotcom/tap`.

A escrita vai por **deploy key**, não por token de usuário: uma chave SSH com
permissão de escrita naquele repositório e em nenhum outro, que não autentica
como ninguém e não vence. A metade pública é registrada como deploy key no
`homebrew-tap`; a privada vira o segredo `HOMEBREW_TAP_DEPLOY_KEY` deste
repositório. Enquanto não existirem, o release sai completo com um canal a
menos: a configuração pula o envio em vez de falhar depois de já ter publicado
binário.

O cask é **só macOS** — Homebrew no Linux não instala cask.

**O pacote no ghcr.** A primeira publicação cria o pacote como privado. Depois
dela, marque-o como público nas configurações do pacote, ou `docker pull` vai
pedir autenticação a quem só quer experimentar.

## O release

1. Confirme que a `main` está verde, incluindo a suíte de integração — o
   workflow de release não a executa, porque ela precisa de Docker no runner e
   o que ela protege já foi protegido no merge.
2. **Escreva a seção da versão no `CHANGELOG.md`**, com a data. É ela que vira
   a nota de release: o workflow a extrai com `scripts/release-notes.sh` e a
   passa ao goreleaser. Sem a seção, o release falha antes de publicar — de
   propósito, porque nota de release não se corrige depois de publicada.

   Confira o que vai sair antes de marcar a tag:

   ```console
   $ scripts/release-notes.sh v0.1.2
   ```

3. Reveja `docs/benchmark/recall.md`. Os números publicados no README saem
   dali, e um release que os contradiga é pior do que um release sem eles.
4. Rode `make release-check`. Ele valida a configuração e prova que o binário
   do caminho de release sabe a própria versão.
5. Rode `goreleaser release --snapshot --clean` e abra `dist/`. É a última
   chance de olhar o que vai ser publicado antes de existir uma tag.
6. Crie e empurre a tag:

   ```console
   $ git tag -a v0.1.0 -m 'v0.1.0'
   $ git push origin v0.1.0
   ```

7. O workflow roda suíte, lint e build cruzado, e só então publica. Acompanhe
   até o fim: uma falha depois da metade deixa artefatos parciais, e a resposta
   certa é corrigir e lançar a versão seguinte, nunca reescrever a tag.

Duas tags no mesmo commit é o caso normal quando o candidato passa e nada muda
depois — e o workflow passa `GORELEASER_CURRENT_TAG` por causa disso. Sem essa
variável, a versão sai de `git describe`, que escolhe entre as tags do commit e
pode escolher o candidato: foi assim que o release de `v0.1.0` tentou publicar
como `v0.1.0-rc.2` e morreu em asset já existente.

## O que o release não oferece

**Os artefatos não são assinados.** O que existe é `checksums.txt`, publicado
junto do release: confira o binário baixado contra ele antes de executar.
Assinatura e SBOM ficam de fora por decisão, não por esquecimento — ambos
acrescentam ferramenta externa e chave para custodiar, e entram quando houver
quem os consuma.

**A imagem não é reconstruída a partir do fonte durante o release.** Ela recebe
o mesmo binário que vai nos arquivos, o que é o ponto: compilar de novo
produziria um artefato diferente do que foi verificado.

**Não há repositório apt nem yum.** A instalação por `.deb` ou `.rpm` é de arquivo
baixado, então não atualiza sozinha. Hospedar repositório exigiria servidor e
chave de assinatura sob custódia, que é compromisso de outra ordem. Também não
há página de manual: ela custaria uma dependência só para converter markdown em
roff, e o `--help` cobre o mesmo terreno.

**O cask limpa a quarentena do macOS na instalação.** Binário baixado e não
assinado é recusado na primeira execução com uma mensagem sobre desenvolvedor
não verificado, e sem isso o cask instalaria sem funcionar. Remover o atributo
contorna uma verificação do Gatekeeper; o cask diz isso no próprio `caveats`,
para que quem instala saiba o que aceitou. A solução de raiz é assinar e
notarizar, que exige conta de desenvolvedor Apple.

## O que sai

| Canal | O quê |
|---|---|
| GitHub Releases | binário para linux, macOS e Windows, em amd64 e arm64, mais `checksums.txt` |
| `.deb` e `.rpm` | no release, para amd64 e arm64, com autocompletar de shell |
| `ghcr.io` | imagem multiplataforma sobre `distroless/static`, com certificados raiz e usuário sem privilégio |
| Homebrew | cask no tap, só macOS, quando a deploy key existe |
| `go install` | sempre disponível, e carimba a versão pelo que o toolchain grava |
