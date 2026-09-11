//go:build !windows

package store

// envolver y desenvolver fuera de Windows no hacen nada: la llave se apoya en
// los permisos del archivo (0600), y nada más.
//
// **Eso es más flojo que en Windows, y se dice en voz alta.** El llavero del
// sistema —Keychain en Mac, Secret Service en Linux— pediría CGo o lanzar un
// programa externo, y este proyecto no tiene ninguna de las dos cosas a
// propósito: un binario, sin CGo, que corre igual en las tres plataformas
// (ADR 0001). El despliegue de referencia es Windows, que es donde está la
// protección de verdad.
//
// Si algún día hace falta apretar esto en Mac o Linux, el sitio es este
// archivo y nada más: el resto del código no sabe que existe la diferencia.
func envolver(llave []byte) ([]byte, error) { return llave, nil }

func desenvolver(guardada []byte) ([]byte, error) { return guardada, nil }
