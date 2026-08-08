//go:build integration

package testutil

import (
	"strings"
	"testing"
)

// PlantedValues is every recognizable value the fixtures write into user
// tables. They stand in for what a real target database holds — national ID
// numbers, names, addresses, equipment descriptions — and none of them may
// appear in any output, log, error or generated file.
//
// The list is one list on purpose. It used to be copied into each test package,
// and the copies drifted: one of them knew three of these values while the
// fixtures planted fourteen. That failure mode is the worst available, because
// a scan that does not know a value does not fail — it passes, and a green test
// that never looked is indistinguishable from a green test that did.
//
// A new fixture adds its values here, and every scan in the project learns them
// in the same commit.
var PlantedValues = []string{
	"145.892.663-04",
	"529.318.470-11",
	"Bomba Centrifuga",
	"CT-2019-0041",
	"Compressor Industrial",
	"Construtora Horizonte LTDA",
	"Conta Corrente",
	"Equipe de Campo",
	"Filial Leste",
	"Filial Oeste",
	"Fornecedor Municipal",
	"Joao Carlos Pereira",
	"Leitura Manual Bloco C",
	"Maria Aparecida Silva",
	"Matriz Central",
	"Ponto de Coleta",
	"Prefeitura de Sao Bernardo",
	"Rua das Acacias 42",
	"Sao Bernardo do Campo",
	"Secretaria de Obras",
	"Secretaria de Saude",
	"peca de reposicao",
	"servico de manutencao",
}

// AssertNoLeak fails when any planted value appears in the given texts, naming
// which stream carried which value. Keys name the output — "stdout", "json",
// "confirmed.sql" — so a failure says where to look.
func AssertNoLeak(t *testing.T, texts map[string]string) {
	t.Helper()

	for name, text := range texts {
		for _, planted := range PlantedValues {
			if strings.Contains(text, planted) {
				t.Errorf("%s leaked %q from a user table", name, planted)
			}
		}
	}
}
