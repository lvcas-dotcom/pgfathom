## 1. Realce vira nível

- [x] 1.1 `report.Emphasis` passa de `bool` para nível: nenhum, 4 bits, 24 bits
- [x] 1.2 `truecolor(hex)` e a queda por papel, sem dependência
- [x] 1.3 A fronteira lê `COLORTERM` e resolve o nível uma vez
- [x] 1.4 Todos os chamadores atualizados; golden files inalterados

## 2. Papéis

- [x] 2.1 `Alert` na cor da marca, `Dim` em `#8c7d73`
- [x] 2.2 `Confirm` em `#7ec9b8`, o papel que faltava, aplicado ao veredito confirmado

## 3. A marca

- [x] 3.1 Glifo de 14 linhas com gradiente que aprofunda sem sair do vermelho
- [x] 3.2 `Banner` só escreve em stderr, e só em terminal
- [x] 3.3 Ligada ao binário sem argumento e à abertura do `setup`

## 4. O guia

- [x] 4.1 Estilos do `lipgloss` derivados das constantes da marca
- [x] 4.2 `BrandInk` como contraparte de `#ddd5d0` em fundo claro
- [x] 4.3 Tique, cursor, resposta digitada e falha de conexão nos papéis certos

## 5. Verificação

- [x] 5.1 `go test ./...` verde, golden files inalterados
- [x] 5.2 `golangci-lint run` limpo
- [x] 5.3 Saída canalizada sem nenhum byte de marca ou de escape
