## Why

Sete fases construíram a ferramenta e uma mediu. Falta a única coisa que separa isso de um repositório: uma forma de alguém obter o binário sem clonar e compilar.

O público é específico e a exigência dele vem junto. Quem autoriza rodar isto contra produção abre o `go.mod` antes de decidir — já é requisito de produto — e a mesma pessoa vai querer conferir o checksum do que baixou, saber de qual commit saiu, e não descobrir a versão perguntando ao binário e ouvindo `unknown`.

## What Changes

- `.goreleaser.yaml`: binário estático para linux, macOS e Windows, em amd64 e arm64, com o mesmo carimbo de versão que o Makefile já injeta. Checksums no release.
- Imagem de contêiner multiplataforma publicada no `ghcr.io`, sobre base mínima **com certificados raiz** — a ferramenta abre conexão TLS com o servidor do usuário, e uma imagem sem CA transforma `sslmode=verify-full` em erro incompreensível.
- Workflow de release disparado por tag, que roda a suíte antes de publicar qualquer coisa.
- Tap do Homebrew, com o pré-requisito declarado: é repositório separado e token próprio, e nada nesta change pode criá-los por você.
- `docs/RELEASING.md`: o procedimento, incluindo o que só existe fora deste repositório.
- Teste de que o binário publicado sabe a própria versão. Carimbo é duas configurações que precisam concordar — Makefile e goreleaser — e duas configurações que precisam concordar divergem.
- README e roadmap deixam de dizer que a fase 8 está planejada.

## Capabilities

### New Capabilities

- `distribution`: o que um release contém, o que ele promete sobre procedência, e o que a imagem de contêiner precisa carregar para a ferramenta funcionar dentro dela.

## Impact

Nenhuma mudança em código de produto. Nenhuma dependência nova no `go.mod`: o `goreleaser` é ferramenta de build, invocada por versão fixada, e nunca entra no binário.

O que esta change acrescenta é superfície de operação, não de execução: um arquivo de configuração, um workflow e um documento. O risco correspondente é publicar artefato errado — binário sem carimbo, imagem sem CA, release sem checksum —, e é contra cada um desses que entra uma verificação, porque nenhum deles falha em teste unitário e todos falham na mão do usuário.

Duas coisas dependem de você e não podem ser feitas daqui: criar o repositório `homebrew-tap` e gerar o token que o workflow usa para escrever nele. Enquanto não existirem, o release funciona sem Homebrew, e o documento diz isso em vez de deixar o workflow falhar sem explicação.

Esta é a quarta e última change da fase 8.
