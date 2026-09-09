package f0

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Stats muestrea CPU y RAM de todo lo que es ffmpeg más el propio proceso,
// cada tanto, a un CSV. F0-07 exige cifras medidas, no estimadas.
type Stats struct {
	f      *os.File
	encPID int
	// Windows reporta segundos de CPU acumulados, no porcentaje: se guarda
	// la lectura anterior por proceso y se saca el % del incremento.
	lastCPU map[int]float64
	lastAt  time.Time
}

func NewStats(path string, encPID int) *Stats {
	f, _ := os.Create(path)
	fmt.Fprintln(f, "t,cpu_pct_total,rss_mb_total,cpu_pct_encoder,rss_mb_encoder,procesos_ffmpeg")
	return &Stats{f: f, encPID: encPID, lastCPU: map[int]float64{}}
}

func (s *Stats) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			cpuT, rssT, cpuE, rssE, n := s.sample()
			fmt.Fprintf(s.f, "%s,%.1f,%.0f,%.1f,%.0f,%d\n", time.Now().Format(time.RFC3339), cpuT, rssT, cpuE, rssE, n)
		}
	}
}

func (s *Stats) Close() { s.f.Close() }

// sample usa ps (Unix) o PowerShell (Windows): sin dependencias, sin CGo.
func (s *Stats) sample() (cpuT, rssT, cpuE, rssE float64, n int) {
	encPID := s.encPID
	self := os.Getpid()
	if runtime.GOOS == "windows" {
		out, err := exec.Command("powershell", "-NoProfile", "-Command",
			"Get-Process ffmpeg,f0 -ErrorAction SilentlyContinue | ForEach-Object { \"$($_.Id) $($_.CPU) $($_.WorkingSet64)\" }").Output()
		if err != nil {
			return
		}
		now := time.Now()
		dt := now.Sub(s.lastAt).Seconds()
		seen := map[int]bool{}
		for _, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			fs := strings.Fields(ln)
			if len(fs) < 3 {
				continue
			}
			pid, _ := strconv.Atoi(fs[0])
			secs, _ := strconv.ParseFloat(strings.ReplaceAll(fs[1], ",", "."), 64)
			rss, _ := strconv.ParseFloat(fs[2], 64)
			seen[pid] = true
			var cpu float64
			if prev, ok := s.lastCPU[pid]; ok && dt > 0 {
				cpu = 100 * (secs - prev) / dt
			}
			s.lastCPU[pid] = secs
			rssT += rss / 1048576
			cpuT += cpu
			if pid != self {
				n++
			}
			if pid == encPID {
				rssE, cpuE = rss/1048576, cpu
			}
		}
		for pid := range s.lastCPU {
			if !seen[pid] {
				delete(s.lastCPU, pid)
			}
		}
		s.lastAt = now
		return
	}
	out, err := exec.Command("ps", "-axo", "pid=,%cpu=,rss=,comm=").Output()
	if err != nil {
		return
	}
	for _, ln := range strings.Split(string(out), "\n") {
		fs := strings.Fields(ln)
		if len(fs) < 4 {
			continue
		}
		pid, _ := strconv.Atoi(fs[0])
		comm := fs[3]
		if !strings.HasSuffix(comm, "ffmpeg") && pid != self {
			continue
		}
		cpu, _ := strconv.ParseFloat(fs[1], 64)
		rss, _ := strconv.ParseFloat(fs[2], 64)
		cpuT += cpu
		rssT += rss / 1024
		if strings.HasSuffix(comm, "ffmpeg") {
			n++
		}
		if pid == encPID {
			cpuE, rssE = cpu, rss/1024
		}
	}
	return
}
