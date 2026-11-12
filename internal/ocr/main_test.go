package ocr

import (
	"os"
	"testing"
	"time"
)

// TestMain deixa o próprio binário de teste fazer o papel de um processo
// externo preso: com EPROCIA_TESTE_DORMIR definida, ele dorme 1 minuto e sai
// sem rodar teste nenhum.
func TestMain(m *testing.M) {
	if os.Getenv("EPROCIA_TESTE_DORMIR") != "" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}
