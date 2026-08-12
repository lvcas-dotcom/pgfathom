## Why

O usuário mais provável desta ferramenta é DBA num servidor Linux, e nenhum dos canais atuais serve bem a ele. `go install` exige Go instalado e deixa o binário fora do `PATH` na configuração padrão de muita gente. Homebrew é cask, e cask é só macOS. Contêiner para uma ferramenta de linha de comando é fricção que ninguém aceita para experimentar. O que sobra é baixar `.tar.gz` do GitHub, descompactar, mover à mão e conferir checksum — que funciona e é exatamente o atrito que faz a pessoa desistir antes da primeira execução.

Pacote de distribuição resolve isso com o comando que essa pessoa já digita, e ainda traz o que um pacote deve trazer: binário no lugar certo, licença e documentação onde o sistema espera, e autocompletar de shell funcionando sem configuração.

## What Changes

- `.deb` e `.rpm` entre os artefatos do release, para `amd64` e `arm64`.
- Binário em `/usr/bin`, licença e README em `/usr/share/doc/pgfathom/`.
- Autocompletar para bash, zsh e fish, gerado a partir do próprio comando e instalado onde cada shell procura. Vale também para quem baixa o `.tar.gz`.
- README e `docs/RELEASING.md` passam a listar o canal.

**Nenhum repositório apt ou yum.** Hospedar repositório exige servidor e chave de assinatura para custodiar, e é decisão de outra ordem — o arquivo baixado e instalado com `dpkg -i` ou `rpm -i` cobre o caso sem nada disso.

## Capabilities

### Modified Capabilities

- `distribution`: o release passa a cobrir instalação por gerenciador de pacotes de distribuição, e o que um pacote precisa conter para ser um pacote e não um binário num diretório.

## Impact

Nenhuma dependência nova no binário. O `nfpm` que monta os pacotes está embutido no `goreleaser` e nunca entra no que se distribui.

O autocompletar é gerado por `go run` do próprio comando durante o build, o que significa que ele nunca fica desatualizado em relação às flags — uma flag nova aparece no autocompletar do mesmo release que a introduz, sem ninguém lembrar de regenerar nada.

O risco é o pacote instalar arquivo em lugar errado e sujar a máquina de quem confiou. Contra isso, os caminhos são os que a convenção de cada distribuição define, e a verificação lista o conteúdo do pacote em vez de supô-lo.
