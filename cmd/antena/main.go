// Command antena es Antena787: un solo proceso con la base, el resolver, la
// vigilancia del contenido y el servidor web (PRD §14.1). Escucha en
// 127.0.0.1:7870 y, si la máquina tiene Tailscale, también ahí — nunca en
// 0.0.0.0 por defecto (PRD §19).
//
// Uso:
//
//	antena                       arranca con datos/ junto al ejecutable
//	antena -datos /var/antena    dice dónde van los datos
//	antena -escucha 127.0.0.1:80 cambia dónde escucha
//	antena -web web/dist         sirve la interfaz del disco, no la embebida
//	antena -version              dice qué versión es y se va
//
// El proceso corre bajo el supervisor del sistema —servicio de Windows o
// systemd (F2.5)—: si el arranque falla, se sale con código 1 y un mensaje
// que dice qué pasa, y el supervisor lo vuelve a intentar.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"antena787/internal/api"
	"antena787/internal/app"
)

// Version es la versión del ejecutable. El release la fija con
// -ldflags "-X main.Version=…".
var Version = "F1-dev"

// DefaultListen es donde escucha si nadie dice otra cosa.
const DefaultListen = "127.0.0.1:7870"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Antena787 no pudo arrancar:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		datos   = flag.String("datos", "", "carpeta de datos (por defecto: datos/ junto al ejecutable)")
		escucha = flag.String("escucha", DefaultListen, "dónde escucha la interfaz")
		web     = flag.String("web", "", "carpeta de la interfaz a servir del disco (por defecto: la embebida)")
		version = flag.Bool("version", false, "dice la versión y se va")
	)
	flag.Parse()

	if *version {
		fmt.Println("Antena787", Version)
		return nil
	}

	dataDir, err := resolveDataDir(*datos)
	if err != nil {
		return err
	}

	a, err := app.Open(app.Options{DataDir: dataDir, Version: Version})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a.Start(ctx)

	ui, uiBuilt, err := interfaz(*web)
	if err != nil {
		_ = a.Close()
		return err
	}
	srv := api.New(a, api.Options{UI: ui})

	direcciones := listenAddresses(*escucha)
	var servers []*http.Server
	errs := make(chan error, len(direcciones))
	for _, addr := range direcciones {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			// Que Tailscale no esté no es motivo para no arrancar.
			if addr != direcciones[0] {
				fmt.Fprintf(os.Stderr, "aviso: no pude escuchar en %s (%v); sigo con las demás\n", addr, err)
				continue
			}
			_ = a.Close()
			return fmt.Errorf("no pude escuchar en %s: %w", addr, err)
		}
		s := &http.Server{
			Handler:           srv,
			ReadHeaderTimeout: 20 * time.Second,
			// Sin límite de escritura: /ws es una conexión larga por diseño.
		}
		servers = append(servers, s)
		go func() {
			if err := s.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errs <- err
			}
		}()
	}
	if len(servers) == 0 {
		_ = a.Close()
		return fmt.Errorf("no pude escuchar en ninguna dirección")
	}

	saludo(a, direcciones, dataDir, uiBuilt)

	select {
	case err := <-errs:
		stop()
		apagar(servers)
		_ = a.Close()
		return err
	case <-ctx.Done():
	}

	fmt.Println("\nApagando… (se espera a que todo cierre bien)")
	apagar(servers)
	if err := a.Close(); err != nil {
		return err
	}
	fmt.Println("Listo. El canal queda como estaba.")
	return nil
}

func apagar(servers []*http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, s := range servers {
		_ = s.Shutdown(ctx)
	}
}

// resolveDataDir decide dónde van los datos: lo que diga el flag, o datos/
// junto al ejecutable, que es lo que hace el instalador.
func resolveDataDir(flagValue string) (string, error) {
	if strings.TrimSpace(flagValue) != "" {
		return filepath.Abs(flagValue)
	}
	exe, err := os.Executable()
	if err != nil {
		return filepath.Abs("datos")
	}
	return filepath.Join(filepath.Dir(exe), "datos"), nil
}

// interfaz decide qué se sirve: una carpeta del disco si se pidió, y si no
// lo que quedó embebido en el binario.
func interfaz(dir string) (fs.FS, bool, error) {
	if strings.TrimSpace(dir) == "" {
		return api.UI(), api.UIBuilt(), nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, false, err
	}
	if _, err := os.Stat(filepath.Join(abs, "index.html")); err != nil {
		return nil, false, fmt.Errorf("en %q no hay ningún index.html que servir", abs)
	}
	return os.DirFS(abs), true, nil
}

// listenAddresses son las direcciones donde se escucha: la que se pidió y,
// si existe, la de Tailscale. Nunca 0.0.0.0 por su cuenta (PRD §19).
func listenAddresses(want string) []string {
	out := []string{want}
	_, port, err := net.SplitHostPort(want)
	if err != nil {
		return out
	}
	for _, ip := range tailscaleIPs() {
		addr := net.JoinHostPort(ip, port)
		if addr != want {
			out = append(out, addr)
		}
	}
	return out
}

// tailscaleIPs son las direcciones de la máquina dentro del rango de
// Tailscale (100.64.0.0/10, el CGNAT que usa la red).
func tailscaleIPs() []string {
	_, cgnat, err := net.ParseCIDR("100.64.0.0/10")
	if err != nil {
		return nil
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []string
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipnet.IP.To4()
		if ip != nil && cgnat.Contains(ip) {
			out = append(out, ip.String())
		}
	}
	return out
}

// saludo es lo que se lee en la consola al arrancar. En cristiano, porque lo
// puede estar leyendo alguien que instaló esto por primera vez.
func saludo(a *app.App, direcciones []string, dataDir string, uiBuilt bool) {
	fmt.Println("Antena787", a.Version)
	for i, addr := range direcciones {
		que := "en esta máquina"
		if i > 0 {
			que = "por Tailscale"
		}
		fmt.Printf("  Interfaz     http://%s   (%s)\n", addr, que)
	}
	fmt.Println("  Datos       ", dataDir)

	if a.FFmpegErr != nil {
		fmt.Println("  ffmpeg       NO ESTÁ —", a.FFmpegErr)
		fmt.Println("               Se puede entrar y configurar; para medir y normalizar hace falta.")
	} else {
		fmt.Println("  ffmpeg      ", a.FFmpeg)
	}
	if !uiBuilt {
		fmt.Println("  Interfaz web todavía no compilada: se sirve una página de aviso (make ui)")
	}
	if a.Restored {
		fmt.Println("  AVISO        la base estaba dañada y se restauró el respaldo", a.RestoredFrom)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if hay, err := a.Store.Settings.HasPIN(ctx); err == nil && !hay {
		fmt.Printf("\n  Todavía no hay clave de estación: abre http://%s y el asistente te lleva.\n", direcciones[0])
	}
	fmt.Println("\n  El canal está en modo sombra: arma el plan y publica la guía, todavía no emite.")
	fmt.Println("  Ctrl-C para parar.")
}
