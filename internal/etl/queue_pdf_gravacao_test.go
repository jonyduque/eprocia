package etl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"eprocia/internal/models"

	"golang.org/x/sys/windows"
)

// recusarGravacao troca a DACL de pasta por uma que nega a criação de arquivo
// (FILE_ADD_FILE) a todos e deixa o resto: o PDF já gravado nela continua
// legível, e o TXT de saída não pode ser criado. O Cleanup devolve o acesso
// total para o t.TempDir() poder ser apagado.
func recusarGravacao(t *testing.T, pasta string) {
	t.Helper()
	aplicar := func(sddl string) error {
		sd, err := windows.SecurityDescriptorFromString(sddl)
		if err != nil {
			return err
		}
		dacl, _, err := sd.DACL()
		if err != nil {
			return err
		}
		return windows.SetNamedSecurityInfo(pasta, windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
			nil, nil, dacl, nil)
	}
	t.Cleanup(func() {
		if err := aplicar("D:P(A;OICI;FA;;;WD)"); err != nil {
			t.Errorf("devolver o acesso à pasta: %v", err)
		}
	})
	if err := aplicar("D:P(D;;0x2;;;WD)(A;OICI;FA;;;WD)"); err != nil {
		t.Fatalf("negar a gravação na pasta: %v", err)
	}
}

// A gravação do TXT de um item de PDF falha numa pasta que a recusa: o texto
// que vai para Error/Message (e daí para a linha e a notificação) é a frase
// fixa de disco, sem a pasta, que traz o nome do usuário do Windows.
func TestProcessItem_PDFGravacaoRecusadaSemCaminho(t *testing.T) {
	pasta := filepath.Join(t.TempDir(), "pasta")
	if err := os.Mkdir(pasta, 0o755); err != nil {
		t.Fatal(err)
	}
	pdfCaminho := gravarArquivo(t, filepath.Join(pasta, cnjTeste+" - Evento 12 - SENT1.pdf"),
		pdfComTexto("Sentenca ficticia de FULANO DE TAL"))
	recusarGravacao(t, pasta)

	q := NewQueue(NewPipeline())
	t.Cleanup(q.Shutdown)
	g := &gravadorEventos{}
	q.emitir = g.emitir
	q.cfg = &models.Config{CleaningLevel: "none", OcrMode: "none", OmitirTipos: []string{}}
	q.AddFiles([]string{pdfCaminho})
	fim := esperarTerminoID(t, q, itemComPath(t, q, pdfCaminho).ID)

	if fim.Status != "error" {
		t.Fatalf("Status = %q, esperava error (Message %q)", fim.Status, fim.Message)
	}
	for nome, texto := range map[string]string{"Error": fim.Error, "Message": fim.Message} {
		if texto != textoErroDisco {
			t.Errorf("%s = %q, esperava %q", nome, texto, textoErroDisco)
		}
		if strings.Contains(strings.ToLower(texto), strings.ToLower(filepath.Dir(pasta))) {
			t.Errorf("%s traz a pasta do teste: %q", nome, texto)
		}
	}
}
