import io
exec(open('base.py').read())
# heatmap: filas = franjas, columnas = dias
FRANJAS=['6–9 AM','9–12 PM','12–3 PM','3–6 PM','6–9 PM','9–12 AM','12–6 AM']
CARGA=[[3,5,5,5,5,4,2],[7,9,9,8,9,6,3],[6,11,10,11,10,5,4],[8,12,12,12,11,7,5],[9,12,12,12,12,8,6],[5,8,9,8,9,6,4],[0,0,0,0,0,0,0]]
def celda(m):
    if m==0: return '<div style="height:34px;border-radius:5px;background:#12161c;border:1px solid #1c2128;"></div>'
    pct=m/12*100
    col='#F85149' if m>=11 else ('#D29922' if m>=8 else '#3FB950')
    return '<div style="height:34px;border-radius:5px;background:#12161c;overflow:hidden;display:flex;flex-direction:column;justify-content:flex-end;"><div style="height:%.0f%%;background:%s;opacity:.85;"></div></div>'%(pct,col)
filas=''
for i,f in enumerate(FRANJAS):
    filas+='<div style="display:flex;align-items:center;gap:6px;"><span class="mono" style="width:62px;flex-shrink:0;font-size:11px;color:#6E7681;text-align:right;">%s</span>%s</div>'%(f,''.join('<div style="flex:1;">%s</div>'%celda(m) for m in CARGA[i]))
dias=''.join('<div style="flex:1;text-align:center;font:600 11px %s;letter-spacing:.8px;color:#6E7681;">%s</div>'%(F,d) for d in ['D','L','M','M','J','V','S'])

CLI=[('Ferretería del Este','#2b3346',14,20,'#3FB950'),('Pizzería Borinquen','#3a2226',22,24,'#3FB950'),
     ('Óptica Central','#1f3a37',5,12,'#D29922'),('Muebles Aguadilla','#33253a',8,8,'#3FB950')]
cli=''
for n,c,hechos,total,col in CLI:
    ini=''.join(p[0] for p in n.split()[:2]).upper(); pct=hechos/total*100
    barras=''.join('<div style="flex:1;height:7px;border-radius:2px;background:%s;"></div>'%('#22D3EE' if k<hechos else '#21262D') for k in range(total))
    cli+='''<div style="display:flex;align-items:center;gap:14px;background:#161B22;border:1px solid #21262D;border-radius:10px;padding:13px 17px;">
  <div style="width:38px;height:38px;border-radius:8px;background:linear-gradient(145deg,%s,#161a20);display:flex;align-items:center;justify-content:center;flex-shrink:0;"><span style="font:700 13px %s;color:rgba(255,255,255,.42);">%s</span></div>
  <div style="width:170px;flex-shrink:0;"><div style="font:600 14.5px %s;color:#E6EDF3;">%s</div>
  <div style="font:400 12px %s;color:#8B949E;margin-top:2px;">%d de %d emitidos</div></div>
  <div style="flex-grow:1;display:flex;gap:2px;">%s</div>
  <span style="display:inline-block;width:8px;height:8px;border-radius:50%%;background:%s;flex-shrink:0;"></span></div>'''%(c,F,ini,F,n,F,hechos,total,barras,col)

out=HEAD+'''<div style="display:flex;width:1440px;height:900px;background:#0E1116;">
%s
<div style="flex-grow:1;padding:24px 30px;display:flex;flex-direction:column;gap:16px;overflow:hidden;">
  <div style="display:flex;align-items:flex-end;justify-content:space-between;">
    <div><div style="font:700 24px %s;letter-spacing:-.4px;color:#E6EDF3;">Anuncios</div>
    <div style="font:400 13.5px %s;color:#8B949E;margin-top:3px;">Septiembre 2026 · el tope es 12 minutos por hora</div></div>
    <div style="display:flex;align-items:center;gap:9px;background:#22D3EE;color:#06141a;border-radius:8px;padding:11px 20px;font:600 14.5px %s;">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#06141a" stroke-width="2.4" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>Nueva campaña</div></div>

  <div style="display:flex;gap:16px;">
    <div style="flex:1;background:#161B22;border:1px solid #21262D;border-radius:11px;padding:18px 20px;">
      <div style="display:flex;align-items:baseline;justify-content:space-between;margin-bottom:13px;">
        <span style="font:600 11px %s;letter-spacing:1.2px;color:#6E7681;">DÓNDE TE QUEDA ESPACIO</span>
        <div style="display:flex;gap:14px;">
          <span style="display:flex;align-items:center;gap:5px;"><span style="width:9px;height:9px;border-radius:2px;background:#3FB950;display:inline-block;"></span><span style="font:400 11.5px %s;color:#8B949E;">hay sitio</span></span>
          <span style="display:flex;align-items:center;gap:5px;"><span style="width:9px;height:9px;border-radius:2px;background:#D29922;display:inline-block;"></span><span style="font:400 11.5px %s;color:#8B949E;">casi llena</span></span>
          <span style="display:flex;align-items:center;gap:5px;"><span style="width:9px;height:9px;border-radius:2px;background:#F85149;display:inline-block;"></span><span style="font:400 11.5px %s;color:#8B949E;">llena</span></span></div></div>
      <div style="display:flex;gap:6px;margin-bottom:7px;"><span style="width:62px;flex-shrink:0;"></span>%s</div>
      <div style="display:flex;flex-direction:column;gap:5px;">%s</div>
      <div style="font:400 12px %s;color:#8B949E;margin-top:12px;">De medianoche a 6 AM no hay nada vendido — son las horas que hoy salen en negro.</div>
    </div>

    <div style="width:380px;flex-shrink:0;background:#161B22;border:1px solid #21262D;border-left:3px solid #D29922;border-radius:11px;padding:18px 20px;">
      <div style="font:600 11px %s;letter-spacing:1.2px;color:#6E7681;">ANTES DE GUARDAR</div>
      <div style="font:700 17px %s;color:#E6EDF3;margin-top:12px;line-height:1.3;">18 de los 20 anuncios caben</div>
      <div style="font:400 13px %s;color:#8B949E;margin-top:6px;">Panadería La Espiga · 30 seg · septiembre</div>
      <div style="display:flex;flex-direction:column;gap:11px;margin-top:16px;padding-top:15px;border-top:1px solid #21262D;">
        <div style="display:flex;align-items:flex-start;gap:10px;">
          <span style="display:inline-block;width:7px;height:7px;border-radius:50%%;background:#F85149;flex-shrink:0;margin-top:5px;"></span>
          <div><div style="font:500 13px %s;color:#E6EDF3;">Martes 8, corte de 8:00 PM</div>
          <div style="font:400 12px %s;color:#8B949E;margin-top:2px;">ya está en el tope de 12 minutos</div></div></div>
        <div style="display:flex;align-items:flex-start;gap:10px;">
          <span style="display:inline-block;width:7px;height:7px;border-radius:50%%;background:#D29922;flex-shrink:0;margin-top:5px;"></span>
          <div><div style="font:500 13px %s;color:#E6EDF3;">Jueves 10, corte de 9:00 PM</div>
          <div style="font:400 12px %s;color:#8B949E;margin-top:2px;">choca con otra panadería</div></div></div></div>
      <div style="display:flex;gap:9px;margin-top:18px;">
        <span style="flex:1;text-align:center;background:#22D3EE;color:#06141a;border-radius:8px;padding:11px;font:600 13.5px %s;">Aceptar 18</span>
        <span style="flex:1;text-align:center;border:1px solid #21262D;color:#E6EDF3;border-radius:8px;padding:11px;font:500 13.5px %s;">Mover fechas</span></div>
    </div>
  </div>

  <div style="display:flex;gap:16px;">
    <div style="flex:1;background:#161B22;border:1px solid #21262D;border-radius:11px;padding:17px 20px;">
      <div style="display:flex;align-items:baseline;justify-content:space-between;">
        <span style="font:600 11px %s;letter-spacing:1.2px;color:#6E7681;">INVENTARIO DEL MES</span>
        <span style="font:400 12.5px %s;color:#8B949E;"><span class="mono" style="color:#E6EDF3;font-size:14px;">1,860</span> de <span class="mono">8,640</span> min</span></div>
      <div style="height:9px;border-radius:5px;background:#12161c;margin-top:12px;overflow:hidden;"><div style="width:21.5%%;height:100%%;background:linear-gradient(90deg,#22D3EE,#3FB950);"></div></div>
      <div style="font:600 12.5px %s;color:#22D3EE;margin-top:8px;">21.5%% vendido</div></div>
    <div style="width:380px;flex-shrink:0;background:#161B22;border:1px solid #21262D;border-radius:11px;padding:17px 20px;">
      <div style="font:600 11px %s;letter-spacing:1.2px;color:#6E7681;">EVIDENCIA DE EMISIÓN</div>
      <div style="display:flex;align-items:baseline;gap:11px;margin-top:10px;">
        <span style="font:700 26px %s;color:#3FB950;">100%%</span>
        <span style="font:400 12.5px %s;color:#8B949E;">49 de 49 salieron · ninguno tapado</span></div></div>
  </div>

  <div style="flex-grow:1;display:flex;flex-direction:column;gap:9px;">
    <div style="display:flex;align-items:baseline;justify-content:space-between;">
      <span style="font:600 15px %s;color:#E6EDF3;">Campañas activas</span>
      <span style="font:500 12.5px %s;color:#22D3EE;">enviar reportes de septiembre</span></div>
    %s</div>
</div></div>
'''%(NAV('Anuncios'),F,F,F,F,F,F,F,dias,filas,F,F,F,F,F,F,F,F,F,F,F,F,F,F,F,F,F,F,cli)+FOOT
io.open('Anunciantes.dc.html','w',encoding='utf-8').write(out); plain('Anunciantes',out)
print('Anuncios ok')
