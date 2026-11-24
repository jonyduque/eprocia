package ocr

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
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
	if os.Getenv("EPROCIA_TESTE_SENHA") != "" {
		fmt.Fprintln(os.Stderr, "Command Line Error: Incorrect password")
		os.Exit(1)
	}
	if n := os.Getenv("EPROCIA_TESTE_JORRAR"); n != "" {
		q, err := strconv.Atoi(n)
		if err != nil {
			os.Exit(3)
		}
		os.Stdout.Write(bytes.Repeat([]byte("x"), q))
		os.Exit(0)
	}
	// Defesa contra cascata: um filho sem modo conhecido não roda a suíte
	// (que copiaria o binário de novo).
	if os.Getenv("EPROCIA_TESTE_FILHO") != "" {
		fmt.Fprintln(os.Stderr, "binário de teste chamado como filho sem modo")
		os.Exit(2)
	}
	os.Exit(m.Run())
}
