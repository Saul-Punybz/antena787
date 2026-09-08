# ATENCION: este script quedo ATRASADO. Manual.dc.html se afino a mano despues
# (fichas del pad con degradado, gap 15px) y correr esto lo revierte a la version plana.
# No lo corras sin comparar antes contra Manual.dc.html.
import io
exec(open('base.py').read())
AMB='#D29922'
# ── cart wall ──
CATS=[('Cortinillas','#2b4a8f'),('Anuncios','#8f5a1f'),('Música','#1f6b4a'),('IDs','#5a3a8f')]
CELLS=[('Entrada RadioOnce','Cortinillas',0,14),('Cortinilla corta','Cortinillas',0,6),('Bumper regreso','Cortinillas',0,8),('Transición','Cortinillas',0,4),
 ('Ferretería del Este','Anuncios',0,30),('Pizzería Borinquen','Anuncios',0,15),('Óptica Central','Anuncios',0,60),('Muebles Aguadilla','Anuncios',0,30),
 ('Cama instrumental','Música',62,180),('Cama suave','Música',0,180),('Cierre musical','Música',0,25),('Fondo hablado','Música',0,120),
 ('ID Caribbean Advantage','IDs',0,10),('ID con hora','IDs',0,8),('Promo parrilla','IDs',0,20),('Aviso legal','IDs',0,12)]
COL={c:v for c,v in CATS}
def cell(nombre,cat,prog,dur):
    c=COL[cat]
    m,s=divmod(dur,60); total='%d:%02d'%(m,s)
    if prog:
        rest=dur-int(dur*prog/100); mr,sr=divmod(rest,60)
        fill='<div style="position:absolute;left:0;top:0;bottom:0;width:%d%%;background:rgba(255,255,255,.16);"></div>'%prog
        borde='border:2px solid #E6EDF3;animation:pulso 1.8s ease-in-out infinite;'
        tiempo='<span class="mono" style="font-size:12px;color:#E6EDF3;">%d:%02d</span>'%(mr,sr)
    else:
        fill=''; borde='border:1px solid rgba(255,255,255,.09);'
        tiempo='<span class="mono" style="font-size:11.5px;color:rgba(230,237,243,.5);">%s</span>'%total
    return '''<div style="position:relative;height:104px;border-radius:9px;overflow:hidden;background:%s;%sdisplay:flex;flex-direction:column;justify-content:space-between;padding:11px 12px;">%s
      <div style="position:relative;font:600 13.5px %s;color:#E6EDF3;line-height:1.25;">%s</div>
      <div style="position:relative;display:flex;justify-content:flex-end;">%s</div></div>'''%(c,borde,fill,F,nombre,tiempo)
tabs=''.join('<span style="padding:7px 15px;border-radius:7px;font:%s 13px %s;%s">%s</span>'%('600' if i==0 else '500',F,'background:rgba(255,255,255,.1);color:#E6EDF3;' if i==0 else 'color:rgba(230,237,243,.55);',c) for i,(c,_) in enumerate(CATS))
grid=''.join(cell(*x) for x in CELLS)

out=HEAD+'''<div style="display:flex;width:1440px;height:900px;background:#1a1508;position:relative;">
  <div style="position:absolute;inset:0;background:linear-gradient(180deg,rgba(210,153,34,.16),rgba(210,153,34,.05));pointer-events:none;"></div>
%s
<div style="position:relative;flex-grow:1;padding:22px 30px;display:flex;flex-direction:column;gap:16px;overflow:hidden;">

  <div style="display:flex;align-items:center;gap:20px;background:rgba(210,153,34,.16);border:1.5px solid %s;border-radius:11px;padding:15px 22px;">
    <div style="display:flex;align-items:center;gap:11px;">
      <span style="display:inline-block;width:12px;height:12px;border-radius:50%%;background:%s;animation:pulso 1.4s ease-in-out infinite;"></span>
      <span style="font:700 21px %s;letter-spacing:1.6px;color:#F5D890;">MODO MANUAL</span></div>
    <div style="width:1px;height:26px;background:rgba(210,153,34,.35);"></div>
    <span style="font:500 14px %s;color:#E6EDF3;">Tú controlas el aire ahora mismo</span>
    <div style="flex-grow:1;"></div>
    <div style="text-align:right;">
      <div style="font:600 10.5px %s;letter-spacing:1.2px;color:rgba(245,216,144,.75);">VUELVE SOLO AL AUTOMÁTICO EN</div>
      <div class="mono" style="font-size:27px;color:#F5D890;font-weight:600;margin-top:2px;">4:12</div></div>
    <div style="display:flex;align-items:center;gap:9px;background:#E6EDF3;color:#0E1116;border-radius:24px;padding:13px 26px;font:600 15px %s;">
      <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#0E1116" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 1 0 3-6.7"/><path d="M3 4v5h5"/></svg>
      Volver al automático</div></div>

  <div style="display:flex;gap:18px;flex-grow:1;">
    <div style="flex-grow:1;display:flex;flex-direction:column;gap:12px;">
      <div style="display:flex;align-items:center;gap:8px;">%s
        <div style="flex-grow:1;"></div>
        <span style="font:400 12px %s;color:rgba(230,237,243,.45);">se dispara al soltar, no al tocar</span></div>
      <div style="display:grid;grid-template-columns:repeat(4, minmax(0, 1fr));gap:11px;">%s</div>
    </div>

    <div style="width:330px;flex-shrink:0;display:flex;flex-direction:column;gap:12px;">
      <div style="background:rgba(14,17,22,.62);border:1px solid rgba(255,255,255,.09);border-radius:11px;padding:17px 19px;">
        <div style="font:600 10.5px %s;letter-spacing:1.2px;color:#6E7681;">LO QUE EL AUTOMÁTICO PONDRÍA</div>
        <div style="display:flex;flex-direction:column;gap:11px;margin-top:14px;">
          <div style="display:flex;align-items:center;gap:11px;"><span class="mono" style="font-size:12px;color:#22D3EE;width:52px;">ahora</span><span style="font:500 14px %s;color:#E6EDF3;flex-grow:1;">Los Simuladores</span></div>
          <div style="display:flex;align-items:center;gap:11px;"><span class="mono" style="font-size:12px;color:#6E7681;width:52px;">2:00 PM</span><span style="font:400 14px %s;color:#8B949E;flex-grow:1;">You're Under Arrest</span></div>
          <div style="display:flex;align-items:center;gap:11px;"><span class="mono" style="font-size:12px;color:#6E7681;width:52px;">2:30 PM</span><span style="font:400 14px %s;color:#8B949E;flex-grow:1;">Samurai X</span></div>
        </div>
        <div style="font:400 11.5px %s;color:#6E7681;margin-top:14px;line-height:1.5;padding-top:13px;border-top:1px solid rgba(255,255,255,.07);">Al soltar el control, el sistema entra por el minuto que le toca — no reinicia el bloque.</div></div>

      <div style="background:rgba(14,17,22,.62);border:1px solid rgba(255,255,255,.09);border-radius:11px;padding:17px 19px;">
        <div style="font:600 10.5px %s;letter-spacing:1.2px;color:#6E7681;">SALIENDO AHORA</div>
        <div style="display:flex;align-items:center;gap:12px;margin-top:13px;">
          <span style="display:inline-block;width:9px;height:9px;border-radius:50%%;background:#FF3B30;animation:tally 1.6s ease-in-out infinite;flex-shrink:0;"></span>
          <div style="flex-grow:1;"><div style="font:600 14.5px %s;color:#E6EDF3;">Cama instrumental</div>
          <div style="font:400 12px %s;color:#8B949E;margin-top:2px;">disparada por ti hace 1:58</div></div></div></div>

      <div style="flex-grow:1;"></div>
      <div style="display:flex;align-items:center;justify-content:center;gap:11px;height:74px;border-radius:37px;background:rgba(248,81,73,.14);border:1.5px solid #F85149;">
        <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="#F85149" stroke-width="2.4"><rect x="6" y="6" width="12" height="12" rx="1.5"/></svg>
        <span style="font:700 16px %s;letter-spacing:.6px;color:#F85149;">PARAR TODO</span></div>
    </div>
  </div>
</div></div>
'''%(NAV('Al aire',tint='#141005'),AMB,AMB,F,F,F,F,tabs,F,grid,F,F,F,F,F,F,F,F,F)+FOOT
io.open('Manual.dc.html','w',encoding='utf-8').write(out); plain('Manual',out)
print('Manual ok')
