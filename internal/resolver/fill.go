package resolver

import (
	"sort"

	"antena787/internal/model"
)

// fillPlan es cómo se cubre un hueco: los clips en el orden en que salen, lo
// que hay que recortarle al último y lo que no se pudo cubrir con nada.
type fillPlan struct {
	clips []model.FillerAsset
	trim  int64 // ms de más que se le quitan al último clip (fundido de 1 s)
	short int64 // ms que quedan sin relleno y van al cartel
}

// packFillers arma la combinación de relleno para un hueco (PRD §14.1). Busca
// suma exacta; si no la hay, se permite exceder hasta 5 s y se recorta el
// último clip. Lo que ni así se cubre se devuelve en short para que salga el
// cartel. Es determinista: la misma biblioteca y el mismo hueco dan siempre
// la misma combinación.
func packFillers(gapMs int64, lib []model.FillerAsset) fillPlan {
	if gapMs <= 0 {
		return fillPlan{}
	}
	if len(lib) == 0 {
		return fillPlan{short: gapMs}
	}
	// lib llega ordenada por duración descendente; el mayor manda el prefijo.
	byDur := map[int64]model.FillerAsset{}
	var durs []int64
	for _, f := range lib {
		if _, ok := byDur[f.DurationMs]; !ok {
			byDur[f.DurationMs] = f
			durs = append(durs, f.DurationMs)
		}
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] > durs[j] })
	longest := durs[0]

	var chosen []int64
	rem := gapMs
	// Para huecos largos se pone el clip más largo hasta dejar una cola que
	// se pueda resolver a mano; así el buscador nunca ve un espacio enorme.
	for rem > 4*longest {
		chosen = append(chosen, longest)
		rem -= longest
	}
	tail, ok := solveTail(rem, durs, 0)
	trim := int64(0)
	if !ok {
		tail, ok = solveTail(rem, durs, MaxFillerExcess.Milliseconds())
		if ok {
			var sum int64
			for _, d := range tail {
				sum += d
			}
			trim = sum - rem
		}
	}
	short := int64(0)
	if !ok {
		// Ni exacto ni dentro del margen: se pone lo que quepa y el resto es
		// cartel. Nunca queda un hueco residual sin cubrir.
		tail = nil
		for {
			placed := false
			for _, d := range durs {
				if d <= rem {
					tail = append(tail, d)
					rem -= d
					placed = true
					break
				}
			}
			if !placed {
				break
			}
		}
		short = rem
	}
	chosen = append(chosen, tail...)

	// Salen de menor a mayor duración: el criterio F1-21 pide 3:00 + 4:00 en
	// ese orden, y así el clip que se recorta es siempre el mismo.
	sort.SliceStable(chosen, func(i, j int) bool { return chosen[i] < chosen[j] })
	plan := fillPlan{trim: trim, short: short}
	for _, d := range chosen {
		plan.clips = append(plan.clips, byDur[d])
	}
	if len(plan.clips) == 0 {
		plan.trim = 0
	}
	return plan
}

// solveTail busca una combinación de duraciones que sume el objetivo, o que
// lo exceda como mucho en tol milisegundos. Prueba primero las duraciones
// largas, así que tiende a usar pocos clips, y lleva presupuesto para no
// quedarse pensando en una biblioteca rara.
func solveTail(target int64, durs []int64, tol int64) ([]int64, bool) {
	if target == 0 {
		return nil, true
	}
	if target < 0 {
		return nil, false
	}
	dead := map[int64]bool{}
	budget := 200000
	var dfs func(rem int64) ([]int64, bool)
	dfs = func(rem int64) ([]int64, bool) {
		if rem == 0 {
			return nil, true
		}
		if rem < 0 {
			if -rem <= tol {
				return nil, true
			}
			return nil, false
		}
		if budget <= 0 || dead[rem] {
			return nil, false
		}
		budget--
		for _, d := range durs {
			if d-rem > tol {
				continue
			}
			if sub, ok := dfs(rem - d); ok {
				return append([]int64{d}, sub...), true
			}
		}
		dead[rem] = true
		return nil, false
	}
	return dfs(target)
}
