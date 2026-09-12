package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
	"antena787/internal/store"
)

// Las pruebas del control manual (T5). El plan de F2 decía que la de
// concurrencia era «la parte que más vale la pena escribir primero», así que
// está primero.

// F2-77 — nunca hay dos tenedores del aire a la vez. Diez personas pulsando
// el botón al mismo tiempo y **solo una** se lleva el control; las otras
// nueve reciben quién lo tiene y desde cuándo, que es lo que la pantalla
// enseña junto al botón de quitárselo.
func TestF2_77SoloUnoSeLlevaElAire(t *testing.T) {
	a := conReloj(t, alasDosEnPunto)
	ctx := context.Background()

	const cuantos = 10
	var (
		listos  sync.WaitGroup
		salida  sync.WaitGroup
		arrancá = make(chan struct{})
		mu      sync.Mutex
		ganaron int
		ocupado int
		otros   []error
	)
	listos.Add(cuantos)
	salida.Add(cuantos)
	for i := 0; i < cuantos; i++ {
		go func(n int) {
			defer salida.Done()
			listos.Done()
			<-arrancá
			_, err := a.TomarElControl(ctx, nombreDeOperador(n))
			mu.Lock()
			defer mu.Unlock()
			var ocup *store.ErrOtroTieneElControl
			switch {
			case err == nil:
				ganaron++
			case errors.As(err, &ocup):
				ocupado++
			default:
				otros = append(otros, err)
			}
		}(i)
	}
	listos.Wait()
	close(arrancá)
	salida.Wait()

	if len(otros) > 0 {
		t.Fatalf("alguien falló por algo que no era tener el aire ocupado: %v", otros)
	}
	if ganaron != 1 {
		t.Fatalf("el aire se lo llevaron %d personas a la vez; tiene que ser exactamente una", ganaron)
	}
	if ocupado != cuantos-1 {
		t.Fatalf("%d de %d recibieron «lo tiene otro»; tenían que ser %d", ocupado, cuantos, cuantos-1)
	}

	// Y en la base hay una sola retención abierta, que es donde de verdad
	// importa: la memoria se puede reconstruir, la base no.
	_, hay, err := a.Store.Manual.Abierta(ctx, a.ChannelID)
	if err != nil || !hay {
		t.Fatalf("no quedó ninguna retención abierta en la base (err=%v)", err)
	}
	abiertas := 0
	todas, err := a.Store.Manual.Ultimas(ctx, a.ChannelID, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range todas {
		if h.Abierta() {
			abiertas++
		}
	}
	if abiertas != 1 {
		t.Fatalf("hay %d retenciones abiertas a la vez en la base", abiertas)
	}
}

func nombreDeOperador(n int) string {
	return string(rune('A'+n)) + "-operador"
}

// F2-77, la otra mitad — quitárselo a quien lo tiene es una puerta aparte,
// deja constancia de a quién se lo quitaron, y sigue habiendo uno solo.
func TestF2_77QuitarleElAireDejaDichoAQuien(t *testing.T) {
	a := conReloj(t, alasDosEnPunto)
	ctx := context.Background()

	if _, err := a.TomarElControl(ctx, "Rolando"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.TomarElControl(ctx, "Saul"); err == nil {
		t.Fatal("Saul tomó un aire que ya tenía Rolando")
	}

	_, aQuien, err := a.QuitarleElControl(ctx, "Saul")
	if err != nil {
		t.Fatalf("no se pudo quitar el control: %v", err)
	}
	if aQuien != "Rolando" {
		t.Fatalf("se lo quitó a %q y era a Rolando", aQuien)
	}
	quien, _, enManual := a.QuienTieneElControl()
	if !enManual || quien != "Saul" {
		t.Fatalf("el aire lo tiene %q (manual=%v) y tenía que tenerlo Saul", quien, enManual)
	}

	// La de Rolando quedó cerrada con «quitado», no con «soltado»: él no
	// soltó nada.
	todas, err := a.Store.Manual.Ultimas(ctx, a.ChannelID, 10)
	if err != nil {
		t.Fatal(err)
	}
	var deRolando *model.ManualHold
	for i := range todas {
		if todas[i].User == "Rolando" {
			deRolando = &todas[i]
		}
	}
	if deRolando == nil {
		t.Fatal("no quedó registro de la retención de Rolando")
	}
	if deRolando.EndReason != model.FinQuitado {
		t.Fatalf("la retención de Rolando se cerró con %q y tenía que ser %q",
			deRolando.EndReason, model.FinQuitado)
	}
}

// F2-27 — un canal recién arrancado está en automático. Nunca arranca en
// manual, ni siquiera si la última vez se quedó alguien al mando.
//
// F2-79 — y esa retención que se quedó abierta se cierra con
// `caida_del_sistema`: ni «soltado», ni «fin de bloque», ni «timeout».
// Sin ese motivo el registro miente sobre lo que pasó esa noche.
func TestF2_27y79AlArrancarSiempreEnAutomaticoYLoQueQuedoAbiertoSeCierraBien(t *testing.T) {
	a := conReloj(t, alasDosEnPunto)
	ctx := context.Background()

	// Alguien tenía el aire y el servicio se cayó: la fila queda abierta.
	if _, err := a.Store.Manual.Tomar(ctx, a.ChannelID, "Rolando", alasDosEnPunto.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	// Arranca otra vez.
	a.CargarElManual(ctx)

	if a.EnManual() {
		t.Fatal("el canal arrancó en manual: tiene que arrancar siempre en automático")
	}
	todas, err := a.Store.Manual.Ultimas(ctx, a.ChannelID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(todas) != 1 {
		t.Fatalf("se esperaba una retención y hay %d", len(todas))
	}
	if todas[0].Abierta() {
		t.Fatal("la retención se quedó abierta después de arrancar")
	}
	if todas[0].EndReason != model.FinCaidaDelSistema {
		t.Fatalf("se cerró con %q y tenía que ser %q: sin ese motivo el registro miente",
			todas[0].EndReason, model.FinCaidaDelSistema)
	}
}

// F2-28 y F2-62 — con el control tomado, el deck manual retiene el aire: los
// bloques del plan cuya hora pasa durante la retención **no salen**, y al
// soltar quedan marcados `manual_hold`, que es distinto de `aired` y distinto
// de `skipped`. Cuando alguien pregunte por qué no salió un spot pagado, esa
// diferencia es la respuesta.
func TestF2_28y62ConElControlTomadoElPlanNoSale(t *testing.T) {
	ahora := alasDosEnPunto
	a := conReloj(t, ahora)
	ctx := context.Background()
	conCartel(t, a)

	// Un programa de media hora que ya está al aire.
	_, programa := conBloque(t, a, "Kojak", ahora.Add(-5*time.Minute), 30*time.Minute)
	// Y un spot que le toca dentro de dos minutos, durante la retención.
	spot, _ := conBloque(t, a, "spot de la ferretería", ahora.Add(48*time.Hour), 2*time.Minute)
	corte := conBloqueDe(t, a, spot, model.DeckCommercial, ahora.Add(2*time.Minute), 2*time.Minute, nil)

	f := a.nuevaFuenteDelPlan(ctx, engine.CAtv)

	// Sin nadie al mando, el aire es del programa.
	clip, _, err := f.Next(ahora)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	if clip.Ref != programa.ID {
		t.Fatalf("salió el bloque %d y tenía que salir el programa %d", clip.Ref, programa.ID)
	}

	// Alguien toma el control.
	if _, err := a.TomarElControl(ctx, "Rolando"); err != nil {
		t.Fatal(err)
	}

	// Dos minutos después, cuando le tocaba al corte, **no sale el corte**:
	// el aire lo tiene una persona y todavía no ha disparado nada, así que
	// sale el relleno. Nunca negro (F2-09).
	enElCorte := ahora.Add(2 * time.Minute)
	clip, _, err = f.Next(enElCorte)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner en manual: %v", err)
	}
	if clip.Ref == corte.ID {
		t.Fatal("salió el corte comercial mientras una persona tenía el aire")
	}

	// Y lo que dispare esa persona sí sale, porque el deck manual manda.
	// El reloj se mueve al instante del disparo: lo que se pone a mano sale
	// desde ese momento, no desde la hora en que arrancó la prueba.
	a.now = func() time.Time { return enElCorte }
	suyo, err := a.DispararAlAire(ctx, spot.ID)
	if err != nil {
		t.Fatalf("no se pudo disparar a mano: %v", err)
	}
	clip, _, err = f.Next(enElCorte.Add(time.Second))
	if err != nil {
		t.Fatalf("la fuente no supo qué poner después del disparo: %v", err)
	}
	if clip.Ref != suyo.ID {
		t.Fatalf("salió el bloque %d y tenía que salir el que se disparó a mano, %d", clip.Ref, suyo.ID)
	}

	// Se suelta el control al minuto siguiente: el corte que se quedó sin
	// salir queda marcado `manual_hold`.
	a.now = func() time.Time { return enElCorte.Add(5 * time.Minute) }
	if err := a.cerrarLaRetencion(ctx, model.FinSoltado, a.Now()); err != nil {
		t.Fatal(err)
	}
	items, err := a.Store.Plan.ListRange(ctx, a.ChannelID, ahora.Add(-time.Hour), ahora.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	var elCorte *model.PlanItem
	for i := range items {
		if items[i].ID == corte.ID {
			elCorte = &items[i]
		}
	}
	if elCorte == nil {
		t.Fatal("el corte desapareció del plan")
	}
	if elCorte.State != model.EnManual {
		t.Fatalf("el corte quedó en %q y tenía que quedar en %q: no es que se saltara, es que había alguien al mando",
			elCorte.State, model.EnManual)
	}
}

// F2-31 — «volver al automático» **no corta nada por el medio**: espera a que
// termine lo que está sonando, como máximo un minuto.
func TestF2_31SoltarEsperaAQueAcabeLoQueSuena(t *testing.T) {
	ahora := alasDosEnPunto
	a := conReloj(t, ahora)
	ctx := context.Background()
	conCartel(t, a)

	spot, _ := conBloque(t, a, "spot de la ferretería", ahora.Add(48*time.Hour), 20*time.Second)
	if _, err := a.TomarElControl(ctx, "Rolando"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DispararAlAire(ctx, spot.ID); err != nil {
		t.Fatal(err)
	}

	cuando, err := a.SoltarElControl(ctx)
	if err != nil {
		t.Fatalf("no se pudo soltar: %v", err)
	}
	if !a.EnManual() {
		t.Fatal("el aire se entregó de golpe: soltar tiene que esperar a que acabe el clip")
	}
	if esperado := ahora.Add(20 * time.Second); !cuando.Equal(esperado) {
		t.Fatalf("se entrega a las %s y tenía que ser a las %s (el fin del clip)",
			cuando.Format("15:04:05"), esperado.Format("15:04:05"))
	}

	// A mitad del clip sigue siendo suyo.
	if !a.revisarElManual(ctx, ahora.Add(10*time.Second)) {
		t.Fatal("el aire volvió al automático a mitad del clip")
	}
	// Al acabar, vuelve solo.
	if a.revisarElManual(ctx, ahora.Add(20*time.Second)) {
		t.Fatal("el clip acabó y el aire no volvió al automático")
	}
	if a.EnManual() {
		t.Fatal("la retención se quedó abierta")
	}
}

// F2-31, el tope — un clip largo no deja el aire retenido para siempre: se
// espera un minuto como mucho.
func TestF2_31LaEsperaTieneTope(t *testing.T) {
	ahora := alasDosEnPunto
	a := conReloj(t, ahora)
	ctx := context.Background()
	conCartel(t, a)

	largo, _ := conBloque(t, a, "documental larguísimo", ahora.Add(48*time.Hour), 2*time.Hour)
	if _, err := a.TomarElControl(ctx, "Rolando"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DispararAlAire(ctx, largo.ID); err != nil {
		t.Fatal(err)
	}
	cuando, err := a.SoltarElControl(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if esperado := ahora.Add(EsperaAlSoltar); !cuando.Equal(esperado) {
		t.Fatalf("se entrega a las %s: con un clip de dos horas el tope es un minuto (%s)",
			cuando.Format("15:04:05"), esperado.Format("15:04:05"))
	}
}

// F2-78 — «parar todo» corta **en seco**, sin esperar al clip, y lo que se
// quedó a medias queda marcado `parcial`. Es la diferencia con soltar, y
// quien vaya a facturar ese spot necesita saberla.
func TestF2_78PararTodoCortaEnSecoYDejaElBloqueAMedias(t *testing.T) {
	ahora := alasDosEnPunto
	a := conReloj(t, ahora)
	ctx := context.Background()
	conCartel(t, a)

	spot, _ := conBloque(t, a, "spot pagado", ahora.Add(48*time.Hour), 30*time.Second)
	if _, err := a.TomarElControl(ctx, "Rolando"); err != nil {
		t.Fatal(err)
	}
	suyo, err := a.DispararAlAire(ctx, spot.ID)
	if err != nil {
		t.Fatal(err)
	}

	// A los diez segundos, parar todo.
	a.now = func() time.Time { return ahora.Add(10 * time.Second) }
	if err := a.PararTodo(ctx, "Rolando"); err != nil {
		t.Fatalf("no se pudo parar todo: %v", err)
	}
	if a.EnManual() {
		t.Fatal("parar todo no devolvió el aire al automático, y tiene que hacerlo en seco")
	}

	items, err := a.Store.Plan.ListRange(ctx, a.ChannelID, ahora.Add(-time.Hour), ahora.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	var elSpot *model.PlanItem
	for i := range items {
		if items[i].ID == suyo.ID {
			elSpot = &items[i]
		}
	}
	if elSpot == nil {
		t.Fatal("el bloque disparado desapareció del plan")
	}
	if !elSpot.Partial {
		t.Fatal("el spot se cortó a los diez segundos de treinta y no quedó marcado como parcial")
	}

	// Y la retención se cerró con su motivo propio.
	todas, err := a.Store.Manual.Ultimas(ctx, a.ChannelID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if todas[0].EndReason != model.FinPararTodo {
		t.Fatalf("se cerró con %q y tenía que ser %q", todas[0].EndReason, model.FinPararTodo)
	}
}

// F2-32 — el que toma el control durante un bloque y no lo suelta: cuando el
// bloque termina, el aire vuelve solo al automático sin que nadie pulse nada.
func TestF2_32AlAcabarElBloqueElAireVuelveSolo(t *testing.T) {
	ahora := alasDosEnPunto
	a := conReloj(t, ahora)
	ctx := context.Background()
	conCartel(t, a)

	// Un programa que empezó hace cinco minutos y dura media hora: se acaba
	// a las 14:25.
	conBloque(t, a, "Kojak", ahora.Add(-5*time.Minute), 30*time.Minute)
	if _, err := a.TomarElControl(ctx, "Rolando"); err != nil {
		t.Fatal(err)
	}

	if !a.revisarElManual(ctx, ahora.Add(10*time.Minute)) {
		t.Fatal("el aire volvió al automático antes de que acabara el bloque")
	}
	if a.revisarElManual(ctx, ahora.Add(25*time.Minute)) {
		t.Fatal("el bloque se acabó y el aire no volvió solo al automático")
	}
	todas, err := a.Store.Manual.Ultimas(ctx, a.ChannelID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if todas[0].EndReason != model.FinDeBloque {
		t.Fatalf("se cerró con %q y tenía que ser %q", todas[0].EndReason, model.FinDeBloque)
	}
}

// F2-30 — el hueco que T3 dejó preparado por fin tiene quien lo llene: con
// silencio al aire por encima del umbral, la vigilancia devuelve el control y
// la retención se cierra con `timeout`, no con `soltado`.
func TestF2_30ElSilencioDevuelveElControlYSeCierraComoTimeout(t *testing.T) {
	a := conReloj(t, alasDosEnPunto)
	ctx := context.Background()

	if _, err := a.TomarElControl(ctx, "Rolando"); err != nil {
		t.Fatal(err)
	}
	c := controlManual{a}
	if !c.EnManual() {
		t.Fatal("el control instalado no ve la retención que se acaba de abrir")
	}
	if err := c.VolverAlAutomatico(ctx, "silencio al aire más de 15s"); err != nil {
		t.Fatalf("no se pudo devolver el control: %v", err)
	}
	if a.EnManual() {
		t.Fatal("la vigilancia pidió el aire de vuelta y sigue en manual")
	}
	todas, err := a.Store.Manual.Ultimas(ctx, a.ChannelID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if todas[0].EndReason != model.FinPorTimeout {
		t.Fatalf("se cerró con %q y tenía que ser %q", todas[0].EndReason, model.FinPorTimeout)
	}
}

// Disparar sin tener el control no puede funcionar: sería poner algo al aire
// por encima del plan sin que quede dicho quién estaba al mando.
func TestSinElControlNoSeDispara(t *testing.T) {
	a := conReloj(t, alasDosEnPunto)
	ctx := context.Background()
	spot, _ := conBloque(t, a, "spot", alasDosEnPunto.Add(48*time.Hour), 30*time.Second)

	if _, err := a.DispararAlAire(ctx, spot.ID); err == nil {
		t.Fatal("se disparó algo al aire sin tener el control")
	}
}

// Volver a pulsar «tomar el control» siendo la misma persona no puede
// contestar que el aire lo tiene otro —que eres tú—. Lo enseñó abrir el
// producto contra el binario: un doble clic devolvía «Saul tiene el control
// desde las 6:41 PM» a Saul.
func TestTomarloDosVecesElMismoNoEsUnConflicto(t *testing.T) {
	a := conReloj(t, alasDosEnPunto)
	ctx := context.Background()

	uno, err := a.TomarElControl(ctx, "Rolando")
	if err != nil {
		t.Fatal(err)
	}
	dos, err := a.TomarElControl(ctx, "Rolando")
	if err != nil {
		t.Fatalf("Rolando pidió otra vez el aire que ya tenía y le dijeron que no: %v", err)
	}
	if dos.ID != uno.ID {
		t.Fatalf("se abrió una retención nueva (%d) en vez de devolver la que había (%d)", dos.ID, uno.ID)
	}
	todas, err := a.Store.Manual.Ultimas(ctx, a.ChannelID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(todas) != 1 {
		t.Fatalf("quedaron %d retenciones y tenía que quedar una", len(todas))
	}
}

// F2-32, el caso que el binario destapó — tomar el control mientras lo que
// sale es **relleno** no programa una vuelta automática para dentro de dos
// días.
//
// En una instalación recién hecha el plan empieza con un bloque de cartel que
// puede durar mucho. Tratarlo como «el bloque» dejaba `fin_de_bloque` a dos
// días vista, que es lo mismo que no tener vuelta automática y peor, porque
// la pantalla enseñaba una hora.
func TestTomarElControlSobreRellenoNoInventaUnFinDeBloque(t *testing.T) {
	ahora := alasDosEnPunto
	a := conReloj(t, ahora)
	ctx := context.Background()
	conCartel(t, a)

	// Un bloque larguísimo de relleno, como el que siembra una instalación
	// nueva.
	largo, _ := conBloque(t, a, "cartel de la estación", ahora.Add(48*time.Hour), 48*time.Hour)
	conBloqueDe(t, a, largo, model.DeckFiller, ahora.Add(-time.Hour), 48*time.Hour, nil)

	if _, err := a.TomarElControl(ctx, "Rolando"); err != nil {
		t.Fatal(err)
	}
	e := a.ElManual()
	if e.FinDeBloque != nil {
		t.Fatalf("se programó una vuelta automática para %s por un bloque de relleno",
			e.FinDeBloque.Format("2006-01-02 15:04"))
	}
	// Y sigue en manual pasado un buen rato: de un hueco no se sale por fin
	// de bloque, se sale soltándolo.
	if !a.revisarElManual(ctx, ahora.Add(3*time.Hour)) {
		t.Fatal("el aire volvió al automático solo, y lo que salía era relleno")
	}
}
