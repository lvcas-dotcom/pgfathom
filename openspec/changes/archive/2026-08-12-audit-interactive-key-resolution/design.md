## Context

`internal/validate` continua a única camada que lê dado de tabela do usuário — esta change não move essa fronteira, só lhe dá um segundo chamador dentro de `audit` (o primeiro veio da change anterior). A pergunta que já governava a fase 2 e a change de eficiência continua valendo: a ferramenta será apontada para o banco de produção de outra pessoa, e um prompt que trava um script em CI é tão ruim quanto uma query que trava o servidor.

## Goals / Non-Goals

**Goals**

Testar, como candidato de chave, a combinação de colunas que a heurística de catálogo de hoje não vê — as FKs de coluna única de uma tabela-ponte sem índice sobre o par. Oferecer coluna sintética como saída honesta quando não há chave natural, nomeada pela convenção que o próprio schema já declara em milhares de FKs, sem forçar quem revisa a inventar um nome. Perguntar uma vez por execução, não uma vez por tabela — a decisão é sobre o cenário do schema inteiro, e uma auditoria com centenas de tabelas sem PK não pode virar centenas de perguntas iguais. Fazer os dois sem exigir uma flag nova: o comportamento nasce do ambiente (terminal interativo), não de o usuário lembrar de pedir.

**Non-Goals**

Não perguntar o nome da coluna sintética. O nome é sempre o que o schema já usa — a mesma convenção que qualquer outra tabela do schema segue — nunca texto livre digitado no prompt; se não há convenção nenhuma para seguir, não há coluna sintética a oferecer, só a chave composta (quando existir) ou pular. Não perguntar tabela a tabela: a pergunta é uma só, feita depois que todo o catálogo já foi avaliado, e a resposta vale para todas as tabelas pendentes de uma vez. Não persistir estado entre execuções — não há "continuar de onde parou" entre pausas porque não há mais de uma pausa. Não sondar toda combinação de coluna possível interativamente: a única combinação nova oferecida é a das FKs de coluna única da própria tabela, porque essa é a que tem uma razão de catálogo para ser tentada; qualquer outra o usuário só alcança pelo caminho de coluna sintética. Não tornar `discover` interativo — o escopo é só `audit`, e só o caminho de chave ausente.

## Decisions

### O gate é TTY em ambas as pontas, nunca uma flag

`Streams.Interactive` é `true` só quando stdin e stdout são ambos um terminal real (`isTerminal`, já existente em `internal/cli/output.go`, sem dependência nova). As duas pontas importam: stdin sozinho não basta porque `--out` redireciona artefatos mas o terminal continua no stdout do resumo; stdout sozinho não basta porque um pipe no stdin (`echo | pgfathom audit`) não tem ninguém para responder. Isso é decidido uma vez, na construção de `Streams`, e nunca depende de uma flag — é assim que "comportamento do fluxo, não algo que o usuário precisa lembrar de ligar" fica garantido: rodar em CI, com saída redirecionada ou `--format json > file`, nunca aciona um prompt, porque `Interactive` já é falso antes de qualquer achado ser avaliado.

`--no-probe-keys` desliga o caminho inteiro mesmo em terminal interativo — pedir para não ler dado nenhum inclui não pausar perguntando sobre dado.

### A decisão é global: uma pergunta, não uma por tabela

`resolveUnconfirmedKeys` primeiro percorre todo o catálogo e junta toda tabela cujo achado de PK ausente `probeMissingKeys` deixou sem chave confirmada — nenhuma pergunta acontece nesse laço. Só depois de ter o cenário inteiro é que o comando fala com o operador: quantas tabelas estão pendentes, quantas têm candidato composto (FKs de coluna única não testadas), e o que a convenção de nome de PK do schema diz — cada número citando os exemplos concretos que o sustentam (`NamingEvidence.Examples`, ver adendo abaixo), não só uma porcentagem. A pergunta em si é uma só, e a resposta se aplica a todas as tabelas pendentes de uma vez: `[a]` roda `validate.ProbeUniqueness` no candidato composto de cada tabela que tiver um; `[b]` aplica a sugestão de coluna sintética, com o mesmo nome, a todas; `[enter]` não muda nada. Isso é diferente da primeira versão desta change, que pausava tabela a tabela — corrigido depois da revisão do usuário: numa auditoria com centenas de tabelas sem PK, a mesma pergunta repetida centenas de vezes é fadiga, não ajuda, e o nome da coluna sintética nunca deveria ter sido texto livre — o schema já diz qual é.

### O nome da coluna sintética nunca é digitado

`resolvePKName` devolve o topo do ranking que `Profile.Detect` já produz (`detection.PrimaryKeyNames[0].Affix`), sem limiar de confiança: seja qual for o nome mais comum, é o que o resto do schema já usa, e não há uma "escolha melhor" para o operador fazer sobre isso — só uma resposta a "usar esse nome, ou não". Se a detecção não achou nome nenhum (schema sem tabela de PK simples suficiente para tabular, ver `minPKNameCount`/`minPKNameShare` em `internal/profile`), a opção `[b]` simplesmente não aparece no menu — não há convenção nenhuma para seguir, e inventar uma seria exatamente a hipótese sem evidência que a regra 5 proíbe.

### O candidato composto novo são as FKs de coluna única, não força bruta

`candidateKeys` (change anterior) já cobre "colunas de um índice não-único existente". O gap que esta change fecha é diferente: uma tabela de associação cujas duas FKs (`idkey_a`, `idkey_b`) não têm índice nenhum sobre o par — o motivo mais comum de ausência de PK numa tabela-ponte de schema legado. O candidato novo é "todas as colunas de FK de coluna única declaradas na tabela", oferecido só quando são duas ou mais e ainda não fazem parte de nenhum candidato já testado pelo caminho automático. Continua sendo só um nome de candidato: a confirmação é sempre `validate.ProbeUniqueness`, contagem completa, nunca afirmada sem prova — a regra 5 não abre exceção para o caminho interativo.

### Resposta não reconhecida pergunta de novo, nunca vira pular por engano

`promptKeyResolution` recontrói o prompt e lê de novo sempre que a resposta não bate com nenhuma opção disponível — imprime "invalid answer" e tenta outra vez, em vez de tratar qualquer coisa não reconhecida como `[enter]`. Um `[enter]` de verdade (linha vazia) é sempre válido e sempre significa pular; a diferença entre "o operador quis pular" e "o operador digitou errado" importa, e só a primeira deveria produzir esse resultado. Isso inclui o EOF de stdin fechado: `bufio.Reader.ReadString` devolve linha vazia junto com `io.EOF`, que já bate na regra de linha vazia — não precisa de tratamento especial além de não tentar ler de novo depois.

### Exemplos concretos por trás de cada convenção citada (adendo)

Toda vez que o `audit` ou o `discover` citam uma convenção detectada — nome de PK, prefixo de referência, prefixo de tabela — a citação passa a incluir de 1 a `model.MaxNamingExamples` (3) objetos reais que a sustentam, não só a contagem e a porcentagem. `internal/profile/detect.go` acumula os exemplos durante a mesma passada que já conta ocorrências, capando na acumulação (um `namingAccumulator` por candidato) em vez de recortar depois — um schema com milhares de tabelas nunca guarda mais que um punhado de strings curtas por convenção. A motivação é a mesma de todo achado do `audit`: uma afirmação sem como ser checada contra o schema real não é melhor que um palpite.

### Coluna sintética é seu próprio artefato de duas etapas

`writeConfirmedPrimaryKey` (change anterior) já usa o padrão de duas etapas — `CREATE UNIQUE INDEX CONCURRENTLY`, depois `ADD PRIMARY KEY USING INDEX` — para não pagar o lock de `ADD PRIMARY KEY` direto. A coluna sintética herda o padrão, com uma ressalva honesta a mais: `ADD COLUMN ... GENERATED ALWAYS AS IDENTITY` já reescreve a tabela para popular a sequência em toda linha existente — não há como evitar esse custo, ele é do ato de criar a coluna, não da promoção a PK. O comentário do artefato diz isso, em vez de sugerir que o caminho de duas etapas o evita.

## Risks / Trade-offs

- **Um script que herda um terminal interativo por engano** (ex.: `pgfathom audit` chamado de dentro de outro programa que aloca um pty) pausaria sem que ninguém leia o prompt. Mitigado pelo mesmo padrão que `resolveColor` já usa (`TERM=dumb` também desliga cor); não é um risco novo desta change, é o risco que qualquer detecção de TTY carrega, e o padrão já é aceito no projeto.
- **A mesma resposta se aplica a tabelas bem diferentes entre si** — uma tabela-ponte de duas linhas e uma de dois milhões, ambas resolvidas por `[a]` ou `[b]` sem distinção. Aceito porque a alternativa (perguntar por tabela) é o problema que esta versão corrigiu; o operador que quiser tratamento diferente por tabela ainda tem `--no-probe-keys` mais uma edição manual do achado catálogo-only como saída.
- **Coluna sintética escolhida pela convenção do schema pode colidir** com uma coluna existente numa tabela específica. O `audit` não valida o nome contra o catálogo de cada tabela antes de sugerir; a colisão só aparece quando o `.sql` gerado falhar ao rodar naquela tabela, porque a ferramenta nunca executa DDL — o mesmo raciocínio de "artefato revisável, nunca executado" que já cobre todo o resto do produto.

## Migration

Nenhuma. Comportamento aditivo: sem terminal interativo, ou com `--no-probe-keys`, a saída é byte a byte a mesma da change anterior. `schema_version` não muda — ver proposal.md.

## Open Questions

Nenhuma pendente nesta revisão — o limiar de confiança que ainda estava em aberto (`autoApplyPKNameShare`) saiu do desenho junto com o auto-apply silencioso.
