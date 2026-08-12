## MODIFIED Requirements

### Requirement: Realce respeita o destino e nunca chega a um pipe

O realce SHALL ser decidido uma única vez, na fronteira do processo, a partir da detecção de terminal já existente. O renderizador SHALL receber essa decisão como parâmetro e MUST NOT consultar ambiente por conta própria.

A decisão SHALL ser um nível, não um interruptor: sem realce, realce com as cores de 4 bits, ou realce com as cores da marca em 24 bits. O nível de 24 bits SHALL exigir `COLORTERM` valendo `truecolor` ou `24bit`; qualquer outro terminal recebe as cores de 4 bits.

Quando o realce estiver desligado, a saída MUST NOT conter nenhuma sequência de escape ANSI.

#### Scenario: Saída canalizada é limpa

- **WHEN** a saída de `discover` é canalizada para arquivo ou para outro processo
- **THEN** nenhum byte de escape ANSI aparece no resultado

#### Scenario: O renderizador não decide sozinho

- **WHEN** o renderizador é chamado diretamente em teste com realce desligado
- **THEN** a saída é idêntica à do processo com a saída canalizada, independentemente do ambiente do teste

#### Scenario: Terminal sem cor verdadeira recebe a queda

- **WHEN** a saída é um terminal e `COLORTERM` não anuncia cor de 24 bits
- **THEN** o realce usa apenas as cores de 4 bits, e nenhuma sequência de 24 bits é emitida

## ADDED Requirements

### Requirement: A paleta tem papéis, e cada veredito tem o seu

O realce SHALL usar as cores da marca por papel, e MUST NOT atribuir a mesma cor a papéis diferentes:

| Papel | Cor | Queda em 4 bits |
| --- | --- | --- |
| Quebrada, e qualquer erro | `#e2483d` | vermelho |
| Confirmada | `#7ec9b8` | ciano |
| Atenuado, secundário | `#8c7d73` | esmaecido |

O veredito confirmado SHALL ter cor própria, distinta da de qualquer cabeçalho.

Nenhum papel do relatório SHALL usar `#ddd5d0`: ele desaparece em terminal de fundo claro, e o relatório não detecta o fundo.

#### Scenario: Os dois vereditos se distinguem sem ler o texto

- **WHEN** um relatório com relações quebradas e confirmadas é renderizado em terminal com cor de 24 bits
- **THEN** o cabeçalho de quebradas e o de confirmadas saem em cores diferentes uma da outra e do resto do relatório

#### Scenario: A queda preserva a distinção

- **WHEN** o mesmo relatório é renderizado no nível de 4 bits
- **THEN** os dois vereditos continuam em cores diferentes uma da outra
