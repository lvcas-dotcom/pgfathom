## Why

A taxa de recuperação é a métrica principal do projeto e vai no README. Hoje ela existe em três medições contra bancos privados, feitas à mão, reproduzíveis apenas pelos donos deles. Isso é o suficiente para orientar o desenvolvimento e não é o suficiente para lançar: um número que ninguém de fora pode conferir é uma alegação, não uma medida.

Falta também o que ainda não foi medido nenhuma vez. Tempo por etapa, funil de candidatos e custo em escala nunca saíram de estimativa — a change anterior nomeou os estágios exatamente para que esta pudesse pôr números neles.

E há a calibração pendente. Limiar de score, margem do pré-filtro, limiares de veredito e os pesos de aridade recém-introduzidos entraram todos como estimativa declarada como tal, com a mesma promessa: revistos com o corpus. Sem o corpus, essa promessa vence sozinha no release.

## What Changes

- `internal/bench`: o harness, atrás de `//go:build benchmark`. Carrega um schema, lê as chaves declaradas como gabarito, derruba todas, executa `discovery.Run` **em processo** e compara o que voltou com o que foi derrubado.
- Manifesto de corpus versionado, com URL, commit e checksum por schema. O download vai para diretório ignorado e é conferido contra o checksum; o que entra no git é a receita, não o dump.
- Duas entradas agora — GitLab, com 1.426 tabelas e 1.857 chaves declaradas, e Discourse, com 354 tabelas e 23. O manifesto já nasce com o tipo de aquisição declarado, para que um schema que exija subir a aplicação entre depois como linha nova em vez de reescrita do harness.
- Entrada local opcional, para o dump real em português: presente na máquina, é medido; ausente, o relatório diz que não foi.
- `discovery.Result` passa a devolver o custo por etapa. Os estágios já têm nome desde a change anterior; o que falta é o relógio.
- `make corpus` busca e confere. `make benchmark` mede, e falha dizendo o que fazer quando o corpus não está lá. A rede fica fora da medição.
- Tabela de resultados gerada em `docs/benchmark/`, com versão da ferramenta, servidor, perfil e a decomposição entre nome, detecção e evidência de uso.
- Calibração dos limiares e pesos contra o que o corpus mostrar, com a mudança de cada valor justificada pelo número que a motivou.

**O que esta change não mede.** Veredito, no corpus público. Os dois schemas são DDL sem uma linha de dado, então nenhum candidato pode ser confirmado ou quebrado ali — o que se mede é se o candidato certo foi **gerado**. O critério de "nenhum falso positivo confirmado" continua onde é decidível: nas fixtures, cujo gabarito foi construído junto do cenário. Num banco real sem FK declarada, uma confirmação não declarada é indistinguível do achado que a ferramenta existe para produzir, e um critério que não pode ser falsificado não é critério.

## Capabilities

### New Capabilities

- `benchmark-harness`: o procedimento de medição, o que ele publica e o que ele se recusa a afirmar.

### Modified Capabilities

- `cli-foundation`: a execução programática passa a devolver o custo por etapa, que é o que torna a medição possível sem instrumentar de fora.

## Impact

Nenhuma dependência nova no binário. O harness usa `net/http` da biblioteca padrão para buscar, o `go-toml` que já está no `go.mod` para ler o manifesto, e os `testcontainers` que já servem a suíte de integração — cuja etiqueta de build passa a valer também para `benchmark`.

O harness **escreve** no banco: derruba as chaves declaradas antes de medir. Isso não fere a regra de leitura absoluta, e a distância entre as duas coisas precisa ficar explícita no código — quem derruba é o harness, pela conexão dele, contra um contêiner descartável que ele mesmo subiu. Nenhuma linha de `pgfathom` emite DDL, e o teste que prova isso continua valendo.

O risco é publicar um número que mede outra coisa. Contra ele, o harness executa o mesmo caminho do usuário, em processo, sem reimplementar a orquestração; e todo recorte que ele aplica — schema fora do escopo, chave que aponta para fora do corpus, tabela sem privilégio — aparece no relatório em vez de sumir na conta.

A alternativa era medir pelo binário e reparsear a saída. Foi recusada porque o funil por etapa não existe na saída, e reconstruí-lo de fora significaria inventar do lado de fora um número que só existe do lado de dentro.
