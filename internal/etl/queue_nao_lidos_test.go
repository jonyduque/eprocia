package etl

import (
	"testing"
	"time"
)

func esperarStatus(t *testing.T, q *Queue, status string) *QueueItem {
	t.Helper()
	prazo := time.Now().Add(30 * time.Second)
	for time.Now().Before(prazo) {
		for _, it := range q.GetItems() {
			if it.Status == status {
				return it
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("nenhum item chegou ao status %q", status)
	return nil
}

func TestQueue_ContaNaoLidos(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfComTexto("Texto sadio ficticio um")},
		{"Evento 2 - DOC2.pdf", []byte("não é PDF")},
		{"Evento 3 - DOC3.pdf", pdfComTexto("Texto sadio ficticio tres")},
		{"Evento 4 - DOC4.pdf", []byte("não é PDF")},
		{"Evento 5 - DOC5.pdf", pdfComTexto("Texto sadio ficticio cinco")},
	})
	q := NewQueue(NewPipeline())
	defer q.Shutdown()
	q.cfg = cfgSemOCR()

	q.AddFiles([]string{zipPath})
	item := esperarStatus(t, q, "success")
	if item.Message != "Concluído" {
		t.Errorf("Message = %q, esperava Concluído", item.Message)
	}
	if item.UnreadDocs != 2 {
		t.Errorf("UnreadDocs = %d, esperava 2", item.UnreadDocs)
	}
}

func TestQueue_ReprocessarZeraNaoLidos(t *testing.T) {
	q := &Queue{items: []*QueueItem{{ID: "1", Name: "a.zip", Path: "a.zip", Status: "success", UnreadDocs: 2}}}
	q.encerrada = true // startProcessing não inicia nada
	if err := q.Reprocessar("1", "none", "none"); err != nil {
		t.Fatalf("Reprocessar: %v", err)
	}
	it := q.items[0]
	if it.Status != "pending" || it.UnreadDocs != 0 {
		t.Errorf("status %q, UnreadDocs %d; esperava pending e 0", it.Status, it.UnreadDocs)
	}
}
