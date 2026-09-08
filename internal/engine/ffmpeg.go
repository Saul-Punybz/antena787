package engine

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// FFmpeg localiza el binario. El release lo coloca junto al ejecutable
// (ADR 0003); en desarrollo vale el del PATH o ANTENA_FFMPEG.
func FFmpeg() (string, error) { return findTool("ffmpeg") }

// FFprobe igual que FFmpeg.
func FFprobe() (string, error) { return findTool("ffprobe") }

func findTool(name string) (string, error) {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if env := os.Getenv("ANTENA_FFMPEG"); env != "" {
		p := filepath.Join(env, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", errors.New(name + " no está: ponlo junto al ejecutable, en el PATH, o en ANTENA_FFMPEG")
}
