package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"antena787/internal/model"
	"antena787/internal/resolver"
	"antena787/internal/store"
)

// resolverLoop corre el resolver al arrancar, cada hora en punto, y cada vez
// que alguien pide un recálculo (con dos segundos de espera para juntar
// varios cambios seguidos en una sola corrida).
func (a *App) resolverLoop(ctx context.Context) error {
	if _, err := a.Resolve(ctx); err != nil && ctx.Err() == nil {
		a.Publish("plan", "resolver", "no se pudo armar el plan: "+err.Error())
	}

	debounce := time.NewTimer(time.Hour)
	stop(debounce)
	hourly := time.NewTimer(untilNextHour(a.Now()))
	defer hourly.Stop()
	defer debounce.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-a.recalc:
			stop(debounce)
			debounce.Reset(RecalcDebounce)

		case <-debounce.C:
			if _, err := a.Resolve(ctx); err != nil && ctx.Err() == nil {
				a.Publish("plan", "resolver", "no se pudo armar el plan: "+err.Error())
			}

		case <-hourly.C:
			hourly.Reset(untilNextHour(a.Now()))
			if _, err := a.Resolve(ctx); err != nil && ctx.Err() == nil {
				a.Publish("plan", "resolver", "no se pudo armar el plan: "+err.Error())
			}
		}
	}
}

func stop(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

// untilNextHour es lo que falta para la próxima hora en punto.
func untilNextHour(now time.Time) time.Duration {
	next := now.Truncate(time.Hour).Add(time.Hour)
	if d := next.Sub(now); d > 0 {
		return d
	}
	return time.Hour
}

// Resolve arma el plan de la ventana, aplica el diff contra lo guardado,
// manda los avisos de vencimiento y regenera la guía. Es lo que corre cada
// hora y lo que contesta POST /plan/recalcular.
func (a *App) Resolve(ctx context.Context) (resolver.Output, error) {
	// Una sola resolución a la vez: la corrida de cada hora y un
	// POST /plan/recalcular a mano pueden coincidir, y dos diffs entrelazados
	// dejan el plan con solapes que el esquema rechaza.
	a.resolveMu.Lock()
	defer a.resolveMu.Unlock()

	in, err := a.resolverInput(ctx)
	if err != nil {
		return resolver.Output{}, err
	}
	out := resolver.Resolve(in)

	// El diff: se borra lo planeado del futuro (lo cued y lo aired no se
	// toca nunca) y se mete lo nuevo. Correr esto dos veces con la misma
	// entrada da el mismo plan, así que nunca duplica nada.
	//
	// El corte no es exactamente "ahora": un bloque planeado que empezó antes
	// y todavía no ha terminado se borra también. Si se quedara, el primer
	// bloque nuevo caería encima de él y el esquema —con razón— no deja dos
	// cosas al aire a la vez (auditoría B6).
	if _, err := a.Store.Plan.DeleteFuturePlanned(ctx, a.ChannelID, a.deleteFrom(ctx, in.Now)); err != nil {
		return out, err
	}
	items := make([]*model.PlanItem, len(out.Items))
	for i := range out.Items {
		it := out.Items[i]
		items[i] = &it
	}
	if err := a.Store.Plan.InsertBatch(ctx, items); err != nil {
		return out, err
	}

	a.mu.Lock()
	a.warnings = out.Warnings
	a.advance = out.EpisodeAdvance
	a.owner = out.CounterOwner
	a.mu.Unlock()

	a.expiryNotices(ctx, in.Rules, out.Warnings)

	if err := a.rebuildGuide(ctx, in.Channel); err != nil {
		a.Publish("plan", "guia", "no se pudo publicar la guía: "+err.Error())
	}
	a.Publish("plan", "recalculado",
		fmt.Sprintf("el plan de las próximas %s se armó con %d bloques", humanHours(in.Horizon), len(out.Items)))
	return out, nil
}

func humanHours(d time.Duration) string {
	h := int(d.Hours())
	if h == 1 {
		return "hora"
	}
	return fmt.Sprintf("%d horas", h)
}

// deleteFrom devuelve desde qué instante se borra el plan planeado: `now`, o
// el arranque del bloque planeado que está a caballo sobre `now` si lo hay.
// Lo que ya está cued o aired no entra aquí: eso no se toca nunca.
func (a *App) deleteFrom(ctx context.Context, now time.Time) time.Time {
	// Un bloque puede ser largo; medio día hacia atrás cubre cualquier caso
	// real y no cuesta nada.
	previos, err := a.Store.Plan.ListRange(ctx, a.ChannelID, now.Add(-12*time.Hour), now)
	if err != nil {
		return now
	}
	corte := now
	for _, it := range previos {
		if it.State == model.Planned && it.PlannedAt.Before(corte) && it.End().After(now) {
			corte = it.PlannedAt
		}
	}
	return corte
}

// resolverInput junta todo lo que el resolver necesita. Él no busca nada:
// se lo damos hecho.
func (a *App) resolverInput(ctx context.Context) (resolver.Input, error) {
	return a.ResolverInputAt(ctx, a.Now(), a.Horizon())
}

// Horizon es cuánto plan se escribe por delante (48 h por defecto).
func (a *App) Horizon() time.Duration {
	if a.opts.Horizon > 0 {
		return a.opts.Horizon
	}
	return resolver.DefaultHorizon
}

// ResolverInputAt arma la entrada del resolver como si fuera ese instante,
// con ese horizonte. Es lo que usan la semana y el mes para proyectar, con
// el mismo resolver y sin escribir nada, lo que las reglas pondrán en los
// días que todavía no están en el plan.
func (a *App) ResolverInputAt(ctx context.Context, now time.Time, horizon time.Duration) (resolver.Input, error) {
	var in resolver.Input

	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return in, err
	}
	rules, err := a.Store.Rule.ListAll(ctx, a.ChannelID)
	if err != nil {
		return in, err
	}
	titles, err := a.Store.Title.List(ctx)
	if err != nil {
		return in, err
	}
	assets, err := a.Store.Media.ListReady(ctx)
	if err != nil {
		return in, err
	}
	fillers, err := a.Store.Filler.List(ctx, a.ChannelID)
	if err != nil {
		return in, err
	}

	if horizon <= 0 {
		horizon = resolver.DefaultHorizon
	}
	existing, err := a.Store.Plan.ListRange(ctx, a.ChannelID, now, now.Add(horizon))
	if err != nil {
		return in, err
	}

	decks := map[model.DeckKind]int64{}
	for _, k := range []model.DeckKind{model.DeckManual, model.DeckCommercial, model.DeckProgram, model.DeckFiller} {
		d, err := a.Store.Deck.ByKind(ctx, a.ChannelID, k)
		if err != nil {
			return in, err
		}
		decks[k] = d.ID
	}

	byTitle := map[int64]model.Title{}
	episodes := map[int64][]model.Episode{}
	for _, t := range titles {
		byTitle[t.ID] = t
		eps, err := a.Store.Episode.ListByTitle(ctx, t.ID)
		if err != nil {
			return in, err
		}
		if len(eps) > 0 {
			episodes[t.ID] = eps
		}
	}
	byAsset := map[int64]model.MediaAsset{}
	for _, m := range assets {
		byAsset[m.ID] = m
	}

	return resolver.Input{
		Channel:  ch,
		Rules:    rules,
		Titles:   byTitle,
		Episodes: episodes,
		Assets:   byAsset,
		Fillers:  fillers,
		Existing: existing,
		Decks:    decks,
		Now:      now,
		Horizon:  horizon,
	}, nil
}

// expiryNotices guarda el aviso de vencimiento que toca en la regla, lo deja
// como alarma viva —para que Al aire lo enseñe, que es la tercera pantalla
// que pide F1-46— y, en el umbral de los siete días, lo saca por el canal de
// avisos configurado. Se manda una sola vez por umbral y por regla: el
// resolver ya calla los de las reglas que tienen quien las releve
// (auditoría B10).
func (a *App) expiryNotices(ctx context.Context, rules []model.ScheduleRule, warns []resolver.Warning) {
	byID := map[int64]model.ScheduleRule{}
	for _, r := range rules {
		byID[r.ID] = r
	}
	alarmas := []Alarma{}
	for _, w := range warns {
		if w.Kind != resolver.WarnExpiry || w.Notice == "" || w.RuleID == nil {
			continue
		}
		rule, ok := byID[*w.RuleID]
		if !ok {
			continue
		}
		// La alarma se enseña siempre que el aviso siga vigente, aunque ya se
		// hubiera anotado: es un estado, no un suceso.
		alarmas = append(alarmas, Alarma{
			Tipo:    "vencimiento",
			Nivel:   NivelAviso,
			Texto:   w.Text,
			Detalle: fmt.Sprintf("la regla llega hasta el %s", rule.To),
			Accion:  &AccionAlarma{Texto: "ver", Ruta: "/reglas?filtro=vencen"},
		})
		if rule.LastNoticeSent == w.Notice {
			continue
		}
		if err := a.Store.Rule.SetLastNotice(ctx, rule.ID, w.Notice); err != nil {
			continue
		}
		a.Incident("vencimiento", w.Text)
		if w.Notice == UmbralDeAviso {
			a.Notificar(ctx, "Antena787 · una regla se acaba", w.Text)
		}
	}
	a.setAlarms("vencimiento", alarmas)
}

// rebuildGuide regenera el XMLTV con el plan que hay ahora mismo, lo deja en
// memoria para /guia.xml y lo escribe en la ruta configurada si la hay
// (auditoría B8: la guía nunca va más de un minuto atrasada del plan).
func (a *App) rebuildGuide(ctx context.Context, ch model.Channel) error {
	now := a.Now()
	horizon := a.opts.Horizon
	if horizon <= 0 {
		horizon = resolver.DefaultHorizon
	}
	// La guía mira un poco hacia atrás: un receptor quiere saber qué está
	// puesto ahora, no solo lo que viene.
	items, err := a.Store.Plan.ListRange(ctx, a.ChannelID, now.Add(-6*time.Hour), now.Add(horizon))
	if err != nil {
		return err
	}
	titles, episodes, err := a.guideCatalog(ctx)
	if err != nil {
		return err
	}
	data, err := resolver.XMLTV(ch, items, titles, episodes)
	if err != nil {
		return err
	}

	// La puerta de F1-28: el validador propio corre **antes** de publicar. Si
	// la guía está mal armada, la que ya estaba sigue puesta —vale más una
	// guía vieja que una guía mal— y queda constancia de por qué.
	graves, flojos := resolver.ValidateXMLTVPorGravedad(data)
	if len(graves) > 0 {
		detalle := strings.Join(graves, "; ")
		a.Incident("guia_rechazada", "la guía no se publicó porque "+detalle)
		a.setAlarms("guia", []Alarma{{
			Tipo:    "guia",
			Nivel:   NivelProblema,
			Texto:   "la guía no se pudo publicar: sigue puesta la anterior",
			Detalle: detalle,
			Accion:  &AccionAlarma{Texto: "ver la parrilla", Ruta: "/parrilla"},
		}})
		return errors.New(detalle)
	}
	if len(flojos) > 0 {
		a.setAlarms("guia", []Alarma{{
			Tipo:    "hueco",
			Nivel:   NivelAviso,
			Texto:   "la guía se publicó con tramos sin describir",
			Detalle: strings.Join(flojos, "; "),
			Accion:  &AccionAlarma{Texto: "llenar", Ruta: "/parrilla"},
		}})
	} else {
		a.setAlarms("guia", nil)
	}

	a.mu.Lock()
	a.guide = data
	a.guideItems = items
	a.guideAt = now
	a.mu.Unlock()

	if err := a.writeGuide(ctx, data); err != nil {
		return err
	}
	a.pushGuide(ctx, data)
	return nil
}

// writeGuide deja la guía en la ruta configurada, si la hay, con un cambio de
// nombre atómico: quien la esté leyendo nunca ve media guía.
func (a *App) writeGuide(ctx context.Context, data []byte) error {
	path, err := a.Store.Settings.Get(ctx, KeyGuidePath)
	if err != nil || path == "" {
		return nil //nolint:nilerr // que no haya ruta configurada no es un fallo
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("no se pudo crear la carpeta de la guía %q: %w", dir, err)
		}
	}
	tmp := path + ".nuevo"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("no se pudo escribir la guía en %q: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("no se pudo dejar la guía en %q: %w", path, err)
	}
	return nil
}

// guideCatalog devuelve los títulos por id y los episodios por id, que es
// como los quiere resolver.XMLTV.
func (a *App) guideCatalog(ctx context.Context) (map[int64]model.Title, map[int64]model.Episode, error) {
	titles, err := a.Store.Title.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	byTitle := map[int64]model.Title{}
	byEpisode := map[int64]model.Episode{}
	for _, t := range titles {
		byTitle[t.ID] = t
		eps, err := a.Store.Episode.ListByTitle(ctx, t.ID)
		if err != nil {
			return nil, nil, err
		}
		for _, e := range eps {
			byEpisode[e.ID] = e
		}
	}
	return byTitle, byEpisode, nil
}

// RefreshGuide vuelve a escribir la guía con el plan que hay guardado ahora
// mismo, sin correr el resolver. Es lo que usa /guia.xml cuando todavía no
// ha corrido ninguna resolución en este arranque.
func (a *App) RefreshGuide(ctx context.Context) error {
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return err
	}
	return a.rebuildGuide(ctx, ch)
}

// Guide es la guía vigente, tal como se sirve en /guia.xml, y el instante en
// que se generó.
func (a *App) Guide() ([]byte, time.Time) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.guide, a.guideAt
}

// GuideItems son los plan_item con los que se armó la guía vigente: es lo que
// compara GET /guia contra el plan real.
func (a *App) GuideItems() []model.PlanItem {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]model.PlanItem(nil), a.guideItems...)
}

// Warnings son los avisos de la última corrida del resolver.
func (a *App) Warnings() []resolver.Warning {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]resolver.Warning(nil), a.warnings...)
}

// MarkAired es lo que llamará el motor cuando un bloque termine de salir al
// aire: cierra el plan_item y **solo entonces** avanza el contador de
// episodios de la regla que lo puso (resolver.Output.EpisodeAdvance).
//
// En F1 el canal está en modo sombra y no hay motor: nadie llama a esta
// función. Está escrita y probada para que F2 la enchufe sin inventar la
// semántica del contador a última hora.
func (a *App) MarkAired(ctx context.Context, itemID int64, at time.Time, actualMs int64, partial bool) error {
	if err := a.Store.Plan.MarkAired(ctx, itemID, at, actualMs, partial); err != nil {
		return err
	}
	item, err := a.PlanItem(ctx, itemID)
	if err != nil || item.RuleID == nil {
		return nil //nolint:nilerr // sin regla no hay contador que avanzar
	}
	// El contador no es siempre de la regla que puso el bloque: una regla de
	// repetición no tiene contador propio y escribe en el de su primaria
	// (auditoría B5). Quién es la dueña lo dijo el resolver en CounterOwner,
	// y hay que resolverlo **antes** de buscar el avance, porque el avance
	// también está indexado por la regla dueña.
	dueña := *item.RuleID
	a.mu.RLock()
	if o, hay := a.owner[dueña]; hay && o != 0 {
		dueña = o
	}
	ep, ok := a.advance[dueña]
	a.mu.RUnlock()
	if !ok && item.EpisodeID != nil {
		ep, ok = *item.EpisodeID, true
	}
	if !ok {
		return nil
	}
	return a.Store.Rule.SetLastEpisode(ctx, dueña, ep)
}

// PlanItem busca un plan_item por id. El store no expone un Get, y esto es
// lo único que lo necesita.
func (a *App) PlanItem(ctx context.Context, id int64) (model.PlanItem, error) {
	items, err := a.Store.Plan.ListRange(ctx, a.ChannelID,
		time.Unix(0, 0).UTC(), a.Now().Add(365*24*time.Hour))
	if err != nil {
		return model.PlanItem{}, err
	}
	for _, it := range items {
		if it.ID == id {
			return it, nil
		}
	}
	return model.PlanItem{}, store.ErrNotFound
}
