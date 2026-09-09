package ingest

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// ThumbnailWidth es el ancho de la miniatura. La altura sale sola para no
// deformar la imagen. 320 px alcanza para la pared de carátulas de
// Biblioteca (PRD §13) y no ocupa nada en disco.
const ThumbnailWidth = 320

// Thumbnail saca un cuadro del archivo y lo guarda como PNG en dst. Si el
// instante pedido cae más allá del final —un clip más corto de lo que se
// creía— vuelve a intentarlo desde el principio antes de rendirse.
func Thumbnail(ctx context.Context, ffmpeg, path string, at time.Duration, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return Plainf(err, "no se puede crear la carpeta de las miniaturas")
	}
	if at < 0 {
		at = 0
	}
	err := grabFrame(ctx, ffmpeg, path, at, dst)
	if err == nil && fileHasBytes(dst) {
		return nil
	}
	if at > 0 {
		if err2 := grabFrame(ctx, ffmpeg, path, 0, dst); err2 == nil && fileHasBytes(dst) {
			return nil
		}
	}
	if err == nil {
		err = Plainf(nil, "ffmpeg no devolvió ningún cuadro")
	}
	return Plainf(err, "no se pudo sacar una imagen del archivo")
}

// ThumbnailAt es el instante del que conviene sacar la miniatura: el 10 % de
// la duración, y nunca antes de los tres segundos ni después del minuto. Al
// principio de un clip casi siempre hay negro, barras o un slate.
func ThumbnailAt(durationMs int64) time.Duration {
	if durationMs <= 0 {
		return 0
	}
	at := time.Duration(durationMs/10) * time.Millisecond
	if at < 3*time.Second {
		at = 3 * time.Second
	}
	if at > time.Minute {
		at = time.Minute
	}
	if half := time.Duration(durationMs/2) * time.Millisecond; at > half {
		at = half
	}
	return at
}

func grabFrame(ctx context.Context, ffmpeg, path string, at time.Duration, dst string) error {
	cctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()
	return runFFmpeg(cctx, ffmpeg, []string{
		"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-ss", secs(at.Milliseconds()),
		"-i", path,
		"-frames:v", "1",
		"-vf", "scale=" + itoa(ThumbnailWidth) + ":-2:flags=bicubic",
		"-c:v", "png", "-f", "image2", dst,
	})
}

func fileHasBytes(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Size() > 0
}
