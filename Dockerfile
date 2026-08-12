# O binário chega pronto: o goreleaser já compilou sem cgo e carimbou a versão.
# Compilar de novo aqui produziria um artefato diferente do que foi publicado e
# testado, que é a forma mais fácil de a imagem mentir sobre o que contém.
#
# O contexto é um diretório temporário que o goreleaser monta com um binário por
# plataforma, em subdiretórios — linux/amd64/pgfathom, linux/arm64/pgfathom. É
# daí que vem o TARGETPLATFORM: copiar da raiz do contexto funcionava no formato
# antigo de configuração e falha neste, com "/pgfathom: not found" durante o
# build multiplataforma.
#
# distroless/static, e não scratch. A ferramenta abre conexão TLS com o servidor
# do usuário, e a configuração recomendada para produção verifica o certificado.
# Sem autoridade certificadora dentro da imagem, isso falha com uma mensagem
# sobre certificado desconhecido que parece defeito do servidor — e a primeira
# execução de quem escolheu o contêiner vira um relatório de bug. A variante
# nonroot roda como usuário sem privilégio, que é o que uma ferramenta somente
# de leitura precisa e nada além.
FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM

COPY ${TARGETPLATFORM}/pgfathom /usr/local/bin/pgfathom

ENTRYPOINT ["/usr/local/bin/pgfathom"]
