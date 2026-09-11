// secretos.go guarda lo que no puede ir en texto: la clave de un proveedor de
// streaming, el usuario de un servidor remoto, el testigo de una API.
//
// # Qué protege esto, y qué no
//
// Hay que decirlo claro porque la diferencia decide el diseño entero.
//
// El canal tiene que volver solo después de un apagón, sin que nadie escriba
// una contraseña a las tres de la mañana (PRD §18: arranque tras corte de luz).
// **Eso significa que la máquina tiene que poder descifrar sola**, y por tanto
// cualquiera con acceso a esa máquina y a esa cuenta puede hacerlo también. No
// hay forma de evitarlo: un secreto que la máquina puede usar desatendida es
// un secreto que la máquina puede revelar.
//
// Entonces, ¿para qué sirve?
//
//	Protege     — que la BASE DE DATOS sola no sirva de nada. Se respalda cada
//	              hora, esos respaldos se copian a otra máquina, y alguien puede
//	              abrirla con cualquier visor de SQLite. Ahí no hay claves.
//	Protege     — en Windows, que el archivo de la llave copiado a otra máquina
//	              o a otra cuenta tampoco sirva: DPAPI lo ata a esta cuenta.
//	NO protege  — de quien ya está dentro de la máquina como el usuario que
//	              corre Antena787. Para ése no hay nada que hacer, y prometerlo
//	              sería mentir.
//
// # Cómo
//
// La llave vive **fuera de la base**, en `<datos>/llave.bin`, con permisos
// cerrados. Lo guardado se cifra con AES-256-GCM de la biblioteca estándar: sin
// CGo, sin dependencias, igual en Windows, Linux y Mac.
//
// En Windows la llave se envuelve además con DPAPI antes de escribirla, así que
// el archivo copiado a otro sitio es basura. En Linux y Mac se apoya solo en
// los permisos del archivo, que es más flojo — el llavero del sistema pediría
// CGo o lanzar un programa externo, y el despliegue de referencia es Windows.
// Queda dicho para que nadie crea que las tres plataformas protegen igual.
package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// NombreDeLaLlave es el archivo donde vive la llave, dentro de la carpeta de
// datos. Va al lado de la base a propósito: quien copia la base sin querer no
// se lleva la llave, y quien hace una copia completa de la carpeta sabe lo que
// está copiando.
const NombreDeLaLlave = "llave.bin"

// Secretos cifra y descifra lo que se guarda en la columna `credenciales`.
type Secretos struct{ llave []byte }

// AbrirSecretos carga la llave de la carpeta de datos, y la crea la primera
// vez. Que falle no puede impedir que el canal arranque: quien llama decide
// qué hacer, y lo razonable es seguir sin poder leer las credenciales guardadas
// antes que quedarse sin aire.
func AbrirSecretos(dir string) (*Secretos, error) {
	ruta := filepath.Join(dir, NombreDeLaLlave)
	datos, err := os.ReadFile(ruta)
	switch {
	case err == nil:
		llave, err := desenvolver(datos)
		if err != nil {
			return nil, fmt.Errorf("la llave de %s no se pudo leer: %w", ruta, err)
		}
		if len(llave) != 32 {
			return nil, fmt.Errorf("la llave de %s no tiene el tamaño que debería", ruta)
		}
		return &Secretos{llave: llave}, nil
	case errors.Is(err, os.ErrNotExist):
		return nuevaLlave(ruta)
	default:
		return nil, fmt.Errorf("no se pudo leer la llave: %w", err)
	}
}

// nuevaLlave crea la llave la primera vez, con los permisos ya cerrados desde
// el momento en que se escribe: nunca existe un instante en que el archivo esté
// abierto a todo el mundo.
func nuevaLlave(ruta string) (*Secretos, error) {
	llave := make([]byte, 32)
	if _, err := rand.Read(llave); err != nil {
		return nil, fmt.Errorf("no se pudo generar la llave: %w", err)
	}
	envuelta, err := envolver(llave)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return nil, err
	}
	// O_EXCL: si dos procesos arrancan a la vez, uno gana y el otro relee, en
	// vez de pisarse la llave y dejar ilegible lo que el otro acaba de cifrar.
	f, err := os.OpenFile(ruta, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return AbrirSecretos(filepath.Dir(ruta))
		}
		return nil, fmt.Errorf("no se pudo crear la llave: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(envuelta); err != nil {
		return nil, fmt.Errorf("no se pudo escribir la llave: %w", err)
	}
	return &Secretos{llave: llave}, nil
}

// Cifrar devuelve lo que va a la columna `credenciales`. Vacío entra y sale
// vacío: no hay por qué guardar el cifrado de una cadena vacía.
func (s *Secretos) Cifrar(claro string) ([]byte, error) {
	if s == nil || len(s.llave) == 0 {
		return nil, errors.New("no hay llave para guardar credenciales")
	}
	if claro == "" {
		return nil, nil
	}
	gcm, err := s.gcm()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	// El nonce va delante del texto cifrado: se necesita para descifrar y no
	// es secreto. Lo que no puede repetirse nunca es el par llave+nonce, y por
	// eso sale de crypto/rand y no de un contador.
	return gcm.Seal(nonce, nonce, []byte(claro), nil), nil
}

// Descifrar lee lo que hay en `credenciales`.
func (s *Secretos) Descifrar(guardado []byte) (string, error) {
	if len(guardado) == 0 {
		return "", nil
	}
	if s == nil || len(s.llave) == 0 {
		return "", errors.New("no hay llave para leer las credenciales guardadas")
	}
	gcm, err := s.gcm()
	if err != nil {
		return "", err
	}
	if len(guardado) < gcm.NonceSize() {
		return "", errors.New("la credencial guardada está incompleta")
	}
	nonce, texto := guardado[:gcm.NonceSize()], guardado[gcm.NonceSize():]
	claro, err := gcm.Open(nil, nonce, texto, nil)
	if err != nil {
		// GCM comprueba que nadie lo tocó, así que esto también salta si
		// alguien editó la base a mano, no solo si la llave es otra.
		return "", errors.New("la credencial guardada no se pudo leer: o la llave no es la de esta instalación, o alguien tocó la base")
	}
	return string(claro), nil
}

func (s *Secretos) gcm() (cipher.AEAD, error) {
	bloque, err := aes.NewCipher(s.llave)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(bloque)
}
