#!/usr/bin/env bash
#
# Extrai do CHANGELOG a seção de uma versão, para o goreleaser publicar como
# nota de release.
#
# Existe porque nota gerada de commits descreve o que foi tocado, não o que
# mudou para quem usa — e porque publicar é irreversível. Se a seção não existe,
# isto falha e o release inteiro para: melhor abortar antes de publicar do que
# sair com uma nota vazia que não dá para corrigir depois.
#
# Uso: scripts/release-notes.sh v0.1.1 [caminho-do-changelog]
set -euo pipefail

tag="${1:?uso: release-notes.sh <tag> [changelog]}"
changelog="${2:-CHANGELOG.md}"
version="${tag#v}"

if [ ! -f "$changelog" ]; then
	echo "release-notes: $changelog não existe" >&2
	exit 1
fi

# A seção vai do cabeçalho da versão até o próximo cabeçalho de mesmo nível.
notes=$(awk -v version="$version" '
	$0 ~ "^## \\[" version "\\]" { collecting = 1; next }
	collecting && /^## / { exit }
	# As definições de link ficam no rodapé do arquivo, depois da última seção,
	# e sem cabeçalho que as separe. Sem isto, a versão mais antiga as herda.
	collecting && /^\[[^]]+\]: / { next }
	collecting { print }
' "$changelog")

# Sem as linhas em branco das pontas, para a nota não abrir com um vão.
notes=$(printf '%s\n' "$notes" | sed -e '/./,$!d' -e :a -e '/^\n*$/{$d;N;ba' -e '}')

if [ -z "$notes" ]; then
	echo "release-notes: $changelog não tem seção para $version" >&2
	echo "  acrescente '## [$version] — <data>' antes de marcar a tag" >&2
	exit 1
fi

printf '%s\n' "$notes"
