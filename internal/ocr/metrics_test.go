package ocr

import (
	"os/exec"
	"testing"
)

func TestPrepareCmd_CountsSpawnedProcesses(t *testing.T) {
	before := SpawnedProcesses()
	prepareCmd(exec.Command("go", "version"))
	prepareCmd(exec.Command("go", "env"))
	if got := SpawnedProcesses() - before; got != 2 {
		t.Fatalf("SpawnedProcesses delta = %d, want 2", got)
	}
}
