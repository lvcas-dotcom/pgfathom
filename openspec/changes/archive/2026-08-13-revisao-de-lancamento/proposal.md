## Why

Revisão completa do projeto antes da divulgação, em cinco fases: inventário, arquitetura e escala, comentários, documentação, correções. Ela achou três defeitos que valem mudança de comportamento ou de contrato, e um custo de escala que valia medir.

**O que mais importa: o exemplo principal do README mostrava uma saída que a ferramenta não produz.** Faltavam o cabeçalho de procedência, o aviso de amostragem, os nomes qualificados por schema, as colunas de métrica, as contagens por veredito e a seção `UNVALIDATED`. Havia um aviso no topo dizendo que aquilo era maquete — mas ninguém lê um exemplo de terminal como maquete, e a primeira coisa que um projeto open source mostra não pode ser ficção.

**O segundo: `--out` escrevia SQL em silêncio.** O manifesto — a lista do que foi escrito, com o lembrete de revisar antes de executar — só era impresso com `--format sql`. Na execução comum, relatório na tela e `--out` para os arquivos, o usuário terminava com SQL gerado em disco sem menção de onde foi parar nem de que era para ser lido primeiro. Isso contraria duas regras do próprio projeto: silêncio nunca é ausência, e nada é executado por conta de ninguém.

**O terceiro: "1 tables and 1 declared keys".** Os golden files só exercitavam contagens acima de um, onde singular e plural rendem os mesmos bytes.

## What Changes

- **O manifesto passa a aparecer em todo formato**: junto do relatório no terminal, e em stderr no modo JSON — onde stdout carrega um documento que um manifesto corromperia.
- **Contagens de um leem como singular.**
- **O exemplo do README passa a ser execução real**, contra um schema demo publicado em `docs/DEMO.md` que qualquer um sobe num contêiner e reproduz byte a byte.
- O passe composto da inferência deixa de derivar as formas do mesmo alvo uma vez por tabela do banco, e de alocar um mapa por par de tabelas.

## Capabilities

### Modified Capabilities

- `sql-artifacts`: escrever artefato passa a exigir dizer que foi escrito, em qualquer formato de saída.

## Impact

Nenhuma dependência nova. Nenhum golden file muda: o plural só difere em contagem um, que nenhum deles exercita, e o manifesto é escrito pela CLI, não pelo renderizador.

O passe composto ficou **35% mais rápido e usa 62% menos memória** num schema de 5.000 tabelas — de 2,9 GB alocados para 1,1 GB. Continua quadrático, e isso está medido e registrado em vez de escondido.
