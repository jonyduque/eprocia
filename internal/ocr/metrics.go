package ocr

import (
	"os/exec"
	"sync/atomic"
)

// spawned counts the child processes (pdftoppm, tesseract) prepared for OCR.
// The review harness benchmarks read it to measure process churn per ZIP.
var spawned atomic.Int64

// SpawnedProcesses returns how many OCR child processes were prepared since start.
func SpawnedProcesses() int64 {
	return spawned.Load()
}

// prepareCmd applies the platform process attributes and counts the process.
func prepareCmd(cmd *exec.Cmd) {
	hideConsole(cmd)
	spawned.Add(1)
}
