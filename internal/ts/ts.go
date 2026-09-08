// Package ts lee transport streams MPEG-2 (188 bytes por paquete) lo justo
// para saber si un multiplexor los aceptaría: continuidad, PCR, tasa, y
// marcas de tiempo. No hay TSDuck (ADR 0003); esto es lo nuestro.
package ts

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"sort"
)

const PacketSize = 188

// Report es lo que se mide de un TS completo.
type Report struct {
	Packets       int64
	NullPackets   int64
	CCErrors      int64
	PIDs          map[uint16]int64
	PCRCount      int64
	PCRMaxGapMs   float64
	PCRMeanGapMs  float64
	BitrateMean   float64 // bits/s medidos entre PCRs
	BitrateMin    float64
	BitrateMax    float64
	BitrateDevPct float64 // desviación máxima respecto a la media, en %
	TSNonMonotone int64   // saltos hacia atrás de DTS/PTS por PID
	TSGapsOver1s  int64   // saltos hacia adelante de más de 1 s
	DurationSec   float64
}

func (r Report) String() string {
	return fmt.Sprintf("paquetes %d (nulos %.1f%%) · PIDs %d · errores CC %d · PCR %d, brecha max %.1f ms, media %.1f ms · tasa media %.3f Mb/s (min %.3f, max %.3f, desvío %.2f%%) · marcas no monotónicas %d · saltos >1s %d · %.1f s",
		r.Packets, 100*float64(r.NullPackets)/math.Max(1, float64(r.Packets)), len(r.PIDs), r.CCErrors,
		r.PCRCount, r.PCRMaxGapMs, r.PCRMeanGapMs, r.BitrateMean/1e6, r.BitrateMin/1e6, r.BitrateMax/1e6, r.BitrateDevPct,
		r.TSNonMonotone, r.TSGapsOver1s, r.DurationSec)
}

// Analyze recorre el TS entero.
func Analyze(rd io.Reader) (Report, error) {
	r := Report{PIDs: map[uint16]int64{}}
	br := bufio.NewReaderSize(rd, 1<<20)
	cc := map[uint16]int{}
	lastTS := map[uint16]int64{}
	var pkt [PacketSize]byte
	var lastPCR float64 = -1
	var lastPCRPkt int64
	_ = lastPCRPkt
	var firstPCR, curPCR float64 = -1, -1
	var winPCR float64 = -1
	var winPkt int64
	var rates []float64
	var gaps []float64

	for {
		if _, err := io.ReadFull(br, pkt[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return r, err
		}
		r.Packets++
		if pkt[0] != 0x47 {
			return r, fmt.Errorf("paquete %d sin byte de sincronía", r.Packets)
		}
		pid := uint16(pkt[1]&0x1f)<<8 | uint16(pkt[2])
		r.PIDs[pid]++
		if pid == 0x1fff {
			r.NullPackets++
			continue
		}
		afc := (pkt[3] >> 4) & 3
		c := int(pkt[3] & 0x0f)
		hasPayload := afc&1 == 1
		if prev, ok := cc[pid]; ok && hasPayload {
			if (prev+1)&0x0f != c {
				r.CCErrors++
			}
		}
		if hasPayload {
			cc[pid] = c
		}
		payload := 4
		if afc&2 == 2 {
			al := int(pkt[4])
			if al > 0 && pkt[5]&0x10 != 0 && al >= 7 {
				base := uint64(pkt[6])<<25 | uint64(pkt[7])<<17 | uint64(pkt[8])<<9 | uint64(pkt[9])<<1 | uint64(pkt[10]>>7)
				ext := uint64(pkt[10]&1)<<8 | uint64(pkt[11])
				pcr := float64(base*300+ext) / 27e6
				curPCR = pcr
				if firstPCR < 0 {
					firstPCR = pcr
				}
				if lastPCR >= 0 && pcr > lastPCR {
					gaps = append(gaps, (pcr-lastPCR)*1000)
				}
				// La tasa se mide en ventanas de un segundo: entre dos PCR
				// la granularidad de paquete la haría mentir.
				if winPCR < 0 {
					winPCR, winPkt = pcr, r.Packets
				} else if pcr-winPCR >= 1.0 {
					bits := float64(r.Packets-winPkt) * PacketSize * 8
					rates = append(rates, bits/(pcr-winPCR))
					winPCR, winPkt = pcr, r.Packets
				}
				lastPCR, lastPCRPkt = pcr, r.Packets
				r.PCRCount++
			}
			payload += 1 + al
		}
		// PES: solo en el arranque de unidad, buscamos DTS (o PTS).
		pusi := pkt[1]&0x40 != 0
		if pusi && hasPayload && payload+13 < PacketSize && pkt[payload] == 0 && pkt[payload+1] == 0 && pkt[payload+2] == 1 {
			flags := pkt[payload+7]
			var ts int64 = -1
			if flags&0xc0 == 0xc0 { // PTS y DTS: usamos DTS
				ts = readTS(pkt[payload+14:])
			} else if flags&0x80 != 0 {
				ts = readTS(pkt[payload+9:])
			}
			if ts >= 0 {
				if prev, ok := lastTS[pid]; ok {
					d := ts - prev
					if d < 0 {
						r.TSNonMonotone++
					} else if d > 90000 {
						r.TSGapsOver1s++
					}
				}
				lastTS[pid] = ts
			}
		}
	}
	if len(gaps) > 0 {
		sort.Float64s(gaps)
		r.PCRMaxGapMs = gaps[len(gaps)-1]
		var sum float64
		for _, g := range gaps {
			sum += g
		}
		r.PCRMeanGapMs = sum / float64(len(gaps))
	}
	if len(rates) > 0 {
		var sum float64
		r.BitrateMin, r.BitrateMax = math.MaxFloat64, 0
		for _, x := range rates {
			sum += x
			r.BitrateMin = math.Min(r.BitrateMin, x)
			r.BitrateMax = math.Max(r.BitrateMax, x)
		}
		r.BitrateMean = sum / float64(len(rates))
		dev := math.Max(r.BitrateMax-r.BitrateMean, r.BitrateMean-r.BitrateMin)
		r.BitrateDevPct = 100 * dev / r.BitrateMean
	}
	if firstPCR >= 0 && curPCR > firstPCR {
		r.DurationSec = curPCR - firstPCR
	}
	return r, nil
}

func readTS(b []byte) int64 {
	if len(b) < 5 {
		return -1
	}
	return int64(b[0]>>1&7)<<30 | int64(b[1])<<22 | int64(b[2]>>1&0x7f)<<15 | int64(b[3])<<7 | int64(b[4]>>1)
}
