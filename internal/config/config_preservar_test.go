package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveToFile_PreservaChavesDesconhecidas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"futuro": 1, "ocr_mode": "fast"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.OcrMode = "best"
	if err := SaveToFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if string(m["futuro"]) != "1" {
		t.Errorf("futuro = %s, quer 1", m["futuro"])
	}
	if string(m["ocr_mode"]) != `"best"` {
		t.Errorf("ocr_mode = %s, quer \"best\"", m["ocr_mode"])
	}
	if string(m["omitir_tipos"]) != "[]" {
		t.Errorf("omitir_tipos = %s, quer []", m["omitir_tipos"])
	}
	got, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.OcrMode != "best" {
		t.Errorf("OcrMode lido = %q", got.OcrMode)
	}
}

func TestSaveToFile_ArquivoIlegivelGravaConhecidos(t *testing.T) {
	for _, conteudo := range []string{`{quebrado`, `[1,2]`} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, _ := DefaultConfig()
		if err := SaveToFile(path, cfg); err != nil {
			t.Fatalf("%s: %v", conteudo, err)
		}
		data, _ := os.ReadFile(path)
		var m map[string]json.RawMessage
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("%s: JSON inválido: %v", conteudo, err)
		}
		if _, ok := m["ocr_mode"]; !ok {
			t.Errorf("%s: sem ocr_mode", conteudo)
		}
	}
}

func TestSaveToFile_SemArquivo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	cfg, _ := DefaultConfig()
	cfg.OcrMode = "fast"
	if err := SaveToFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.OcrMode != "fast" {
		t.Errorf("OcrMode = %q", got.OcrMode)
	}
}
