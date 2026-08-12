# O binário chega pronto: o goreleaser já compilou sem cgo e carimbou a versão.
# Compilar de novo aqui produziria um artefato diferente do que foi publicado e
# testado, que é a forma mais fácil de a imagem mentir sobre o que contém.
#
# distroless/static, e não scratch. A ferramenta abre conexão TLS com o servidor
# do usuário, e a configuração recomendada para produção verifica o certificado.
# Sem autoridade certificadora dentro da imagem, isso falha com uma mensagem
# sobre certificado desconhecido que parece defeito do servidor — e a primeira
# execução de quem escolheu o contêiner vira um relatório de bug. A variante
# nonroot roda como usuário sem privilégio, que é o que uma ferramenta somente
# de leitura precisa e nada além.
FROM gcr.io/distroless/static-debian12:nonroot

COPY pgfathom /usr/local/bin/pgfathom

ENTRYPOINT ["/usr/local/bin/pgfathom"]
