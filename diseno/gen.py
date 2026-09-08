import io
src=io.open('Main.dc.html',encoding='utf-8').read()
HEAD=src[:src.index('</helmet>')+len('</helmet>')+1]; FOOT='</x-dc>\n</body>\n</html>\n'
Q="'"
ICONS={
 'Al aire':'<path d="M12 20v-7"/><path d="M8.5 13 12 4l3.5 9"/><path d="M4.9 16.5a9 9 0 0 1 0-9"/><path d="M19.1 7.5a9 9 0 0 1 0 9"/>',
 'Parrilla':'<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18M8 3v4M16 3v4"/>',
 'Reglas':'<rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/>',
 'Biblioteca':'<rect x="3" y="3" width="7" height="9" rx="1.5"/><rect x="14" y="3" width="7" height="9" rx="1.5"/><rect x="3" y="15" width="7" height="6" rx="1.5"/><rect x="14" y="15" width="7" height="6" rx="1.5"/>',
 'Anuncios':'<path d="M20.6 13.4 13.4 20.6a2 2 0 0 1-2.8 0l-7.2-7.2A2 2 0 0 1 2.8 12V4.8A2 2 0 0 1 4.8 2.8H12a2 2 0 0 1 1.4.6l7.2 7.2a2 2 0 0 1 0 2.8Z"/><circle cx="7.5" cy="7.5" r="1.2"/>',
 'Ajustes':'<circle cx="12" cy="12" r="3.2"/><path d="M12 2.6v2.6M12 18.8v2.6M21.4 12h-2.6M5.2 12H2.6M18.6 5.4l-1.8 1.8M7.2 16.8l-1.8 1.8M18.6 18.6l-1.8-1.8M7.2 7.2 5.4 5.4"/>'}
FONT = "'IBM Plex Sans'"
def NAV(act, ads=True):
    rows=[]
    for lab,ic in ICONS.items():
        if lab=='Anuncios' and not ads: continue
        on=(lab==act); bg='background:#161B22;' if on else ''
        col='#E6EDF3' if on else '#8B949E'; st='#22D3EE' if on else '#6E7681'
        bar='<div style="position:absolute;left:0;top:9px;bottom:9px;width:3px;border-radius:0 3px 3px 0;background:#22D3EE;"></div>' if on else ''
        wt='600' if on else '400'
        rows.append('<div style="position:relative;display:flex;align-items:center;gap:12px;padding:11px 14px;border-radius:8px;%s">%s<svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="%s" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">%s</svg><span style="font:%s 14.5px %s,system-ui;color:%s;">%s</span></div>'%(bg,bar,st,ic,wt,FONT,col,lab))
    return '''<div style="width:224px;flex-shrink:0;background:#0B0E12;border-right:1px solid #21262D;display:flex;flex-direction:column;padding:22px 0;">
  <div style="padding:0 20px 26px;display:flex;align-items:center;gap:10px;">
    <svg width="25" height="25" viewBox="0 0 24 24" fill="none" stroke="#22D3EE" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">%s</svg>
    <span style="font:600 17px %s,system-ui;color:#E6EDF3;letter-spacing:-.3px;">Antena<span style="color:#22D3EE;">787</span></span></div>
  <div style="display:flex;flex-direction:column;gap:3px;padding:0 12px;flex-grow:1;">%s</div>
  <div style="padding:16px 20px 0;border-top:1px solid #21262D;margin:0 12px;">
    <div style="font:500 12px %s;color:#6E7681;letter-spacing:.4px;">CANAL</div>
    <div style="font:500 14px %s;color:#8B949E;margin-top:5px;">Caribbean Advantage TV</div></div></div>'''%(ICONS['Al aire'],FONT,''.join(rows),FONT,FONT)

def tabs(act):
    out=[]
    for t in ['Semana','Mes','Guía']:
        on=(t==act); sty='background:#161B22;border:1px solid #22D3EE;color:#E6EDF3;' if on else 'border:1px solid #21262D;color:#8B949E;'
        out.append('<span style="padding:8px 18px;border-radius:7px;font:%s 13.5px %s;%s">%s</span>'%('600' if on else '500',FONT,sty,t))
    return '<div style="display:flex;gap:7px;">'+''.join(out)+'</div>'

PAL={'Kojak':('#2b3346','#151a22'),'Comics 9th Art':('#3a2a3f','#171a20'),"Gilligan's Island":('#1f3a37','#151a20'),
 'RadioOnce Live!':('#0f3a44','#131a20'),'Los Simuladores':('#3a2226','#161a20'),"You're Under Arrest":('#2a3348','#151920'),
 'Samurai X':('#3d2420','#171a20'),'Zoids':('#243b30','#151a20'),'Mazinger Z':('#3c2a1c','#171a20'),'Zorro 57':('#3b2f1a','#171b22')}
def art(t,fs=22,r=6):
    a,b=PAL.get(t,('#2a3040','#161a20')); ini=''.join(p[0] for p in t.split()[:2]).upper()
    return '<div style="position:relative;width:100%%;height:100%%;border-radius:%spx;overflow:hidden;background:linear-gradient(145deg,%s,%s);display:flex;align-items:center;justify-content:center;"><span style="font:700 %spx %s;color:rgba(255,255,255,.13);">%s</span></div>'%(r,a,b,fs,FONT,ini)

# ══════════ MES ══════════
# 1 sep 2026 = MARTES. La cuadricula arranca en la columna MAR (indices 0 y 1 = 30 y 31 de agosto).
vence={6:'Familia Robinson',7:'Magic Knight',11:'Los Simuladores',14:'Hack Legend',
       15:'I Dream of Jeannie',16:'Get Smart',22:'Comics 9th Art',23:'Tarzan',30:'Green Hornet'}
empieza={8:'Zoids'}
PREV={0:30,1:31}   # 30 y 31 de agosto, atenuados
RED='repeating-linear-gradient(45deg,rgba(248,81,73,.55),rgba(248,81,73,.55) 3px,rgba(248,81,73,.2) 3px,rgba(248,81,73,.2) 6px)'
def mini(d):
    dow=(d+1)%7
    finde = dow in (0,6)
    segs=[('f',0,60),('v',60,360),('f',360,1440)] if not finde else [('f',0,60),('v',60,420),('f',420,660),('v',660,840),('f',840,900),('v',900,1080),('f',1080,1440)]
    h=''
    for k,a,b in segs:
        w=(b-a)/1440*100
        h+='<div style="width:%.2f%%;background:%s;"></div>'%(w, RED if k=='v' else '#2a3a4a')
    return h, finde
cells=[]
for i in range(35):
    dd=i-1
    if i in PREV:
        cells.append('<div style="border-radius:8px;background:#0d1117;border:1px solid #14181f;padding:9px 10px;"><span class="mono" style="font-size:14px;color:#3d444d;">%s</span></div>'%PREV[i]); continue
    if i>31:
        cells.append('<div style="border-radius:8px;background:#0d1117;border:1px solid #14181f;"></div>'); continue
    bar,finde=mini(dd); hoy=(dd==4); v=vence.get(dd)
    borde='border:1.5px solid #22D3EE;' if hoy else ('border:1px solid rgba(248,81,73,.35);' if finde else 'border:1px solid #21262D;')
    numcol='#22D3EE' if hoy else '#E6EDF3'; numw='600' if hoy else '400'
    hoytag='<span style="font:600 9px %s;letter-spacing:.6px;color:#22D3EE;">HOY</span>'%FONT if hoy else ''
    marca=''
    if v: marca='<div style="display:flex;align-items:center;gap:5px;margin-top:6px;"><span style="display:inline-block;width:5px;height:5px;border-radius:50%%;background:#D29922;flex-shrink:0;"></span><span style="font:500 10px %s;color:#D29922;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">vence %s</span></div>'%(FONT,v)
    e=empieza.get(dd)
    if e: marca+='<div style="display:flex;align-items:center;gap:5px;margin-top:4px;"><span style="display:inline-block;width:5px;height:5px;border-radius:50%%;background:#22D3EE;flex-shrink:0;"></span><span style="font:500 10px %s;color:#22D3EE;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">empieza %s</span></div>'%(FONT,e)
    hrs='9 h vacías' if finde else '5 h vacías'; hcol='#F85149' if finde else '#6E7681'
    cells.append('''<div style="border-radius:8px;background:#161B22;%spadding:9px 10px;display:flex;flex-direction:column;">
  <div style="display:flex;align-items:baseline;justify-content:space-between;"><span class="mono" style="font-size:14px;color:%s;font-weight:%s;">%s</span>%s</div>
  <div style="display:flex;height:7px;border-radius:3px;overflow:hidden;margin-top:8px;">%s</div>
  <div style="font:400 10px %s;color:%s;margin-top:5px;">%s</div>%s</div>'''%(borde,numcol,numw,dd,hoytag,bar,FONT,hcol,hrs,marca))
dias=''.join('<div style="font:600 11.5px %s;letter-spacing:1px;color:#6E7681;text-align:center;padding-bottom:2px;">%s</div>'%(FONT,d) for d in ['DOM','LUN','MAR','MIÉ','JUE','VIE','SÁB'])
mes=HEAD+'''<div style="display:flex;width:1440px;height:900px;background:#0E1116;">
%s
<div style="flex-grow:1;padding:26px 32px;display:flex;flex-direction:column;gap:16px;overflow:hidden;">
  <div style="display:flex;align-items:flex-end;justify-content:space-between;">
    <div><div style="font:700 24px %s;letter-spacing:-.4px;color:#E6EDF3;">Septiembre 2026</div>
    <div style="font:400 13.5px %s;color:#8B949E;margin-top:3px;">Cada día muestra sus 24 horas. Lo rayado en rojo está vacío.</div></div>%s</div>
  <div style="display:flex;align-items:center;gap:22px;">
    <div style="display:flex;align-items:center;gap:8px;"><div style="width:22px;height:8px;border-radius:3px;background:#2a3a4a;"></div><span style="font:400 12.5px %s;color:#8B949E;">programado</span></div>
    <div style="display:flex;align-items:center;gap:8px;"><div style="width:22px;height:8px;border-radius:3px;background:%s;"></div><span style="font:400 12.5px %s;color:#8B949E;">vacío</span></div>
    <div style="display:flex;align-items:center;gap:8px;"><span style="display:inline-block;width:6px;height:6px;border-radius:50%%;background:#D29922;"></span><span style="font:400 12.5px %s;color:#8B949E;">se vence una regla</span></div>
    <div style="display:flex;align-items:center;gap:8px;"><span style="display:inline-block;width:6px;height:6px;border-radius:50%%;background:#22D3EE;"></span><span style="font:400 12.5px %s;color:#8B949E;">entra al aire</span></div>
    <div style="flex-grow:1;"></div>
    <div style="font:600 13px %s;color:#F85149;">201 horas vacías este mes · 28%% del aire</div></div>
  <div style="display:grid;grid-template-columns:repeat(7, minmax(0, 1fr));gap:8px;">%s</div>
  <div style="display:grid;grid-template-columns:repeat(7, minmax(0, 1fr));grid-auto-rows:100px;gap:8px;flex-grow:1;">%s</div>
  <div style="display:flex;align-items:center;gap:16px;background:#161B22;border:1px solid #21262D;border-left:3px solid #D29922;border-radius:9px;padding:15px 20px;">
    <div style="flex-grow:1;"><div style="font:600 15px %s;color:#E6EDF3;">Los fines de semana tienen casi el doble de aire vacío que los días de semana</div>
    <div style="font:400 13px %s;color:#8B949E;margin-top:3px;">9 horas contra 5. Sábados y domingos de 11:00 AM a 2:00 PM y de 3:00 a 6:00 PM.</div></div>
    <div style="background:#22D3EE;color:#06141a;border-radius:8px;padding:11px 20px;font:600 14px %s;">Llenar los fines de semana</div></div>
</div></div>'''%(NAV('Parrilla'),FONT,FONT,tabs('Mes'),FONT,RED,FONT,FONT,FONT,FONT,dias,''.join(cells),FONT,FONT,FONT)+FOOT
io.open('Mes.dc.html','w',encoding='utf-8').write(mes)
print('Mes ok')

# ══════════ GUÍA ══════════
W=1150.0; MIN0=13*60; MINS=240; PX=W/MINS
GUIA=[('Los Simuladores',60,True),("You're Under Arrest",30,True),('Samurai X',30,True),
      ('Magic Knight Rayearth',30,False),('Mazinger Z',30,True),('Green Hornet',30,True),('Zorro 57',30,True)]
PLAN=[('Los Simuladores',60,True),("You're Under Arrest",30,True),('Samurai X',30,True),
      ('Zoids',30,False),('Mazinger Z',30,True),('Green Hornet',30,True),('Zorro 57',30,True)]
def fila(items, tono):
    out=[]
    for t,dur,ok in items:
        w=round(dur*PX)-4
        if ok:
            bg='#161B22'; bd='1px solid #21262D'; col='#E6EDF3'
        else:
            bg='rgba(248,81,73,.13)'; bd='1.5px solid #F85149'; col='#F85149'
        out.append('<div style="width:%spx;flex-shrink:0;margin-right:4px;height:62px;background:%s;border:%s;border-radius:7px;padding:9px 11px;display:flex;flex-direction:column;justify-content:center;overflow:hidden;"><div style="font:600 13px %s;color:%s;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">%s</div><div class="mono" style="font-size:10.5px;color:#6E7681;margin-top:3px;">%s min</div></div>'%(w,bg,bd,FONT,col,t,dur))
    return ''.join(out)
horas=''.join('<div style="position:absolute;left:%spx;top:0;"><div style="width:1px;height:8px;background:#21262D;"></div><span class="mono" style="font-size:11.5px;color:#6E7681;position:absolute;top:11px;left:-2px;white-space:nowrap;">%s:00 PM</span></div>'%(round((h*60-MIN0)*PX), h-12) for h in range(13,18))
def check(ok,txt):
    c='#3FB950' if ok else '#F85149'
    ico='<path d="m5 12 5 5L19 7"/>' if ok else '<path d="M18 6 6 18M6 6l12 12"/>'
    return '<div style="display:flex;align-items:center;gap:9px;"><svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="%s" stroke-width="2.6" stroke-linecap="round"><%s</svg><span style="font:500 13.5px %s;color:#E6EDF3;">%s</span></div>'%(c,ico[1:],FONT,txt)

guia=HEAD+'''<div style="display:flex;width:1440px;height:900px;background:#0E1116;">
%s
<div style="flex-grow:1;padding:26px 32px;display:flex;flex-direction:column;gap:20px;overflow:hidden;">
  <div style="display:flex;align-items:flex-end;justify-content:space-between;">
    <div><div style="font:700 24px %s;letter-spacing:-.4px;color:#E6EDF3;">Guía electrónica</div>
    <div style="font:400 13.5px %s;color:#8B949E;margin-top:3px;">Lo que ve el televidente en su televisor, comparado con lo que de verdad va a salir.</div></div>%s</div>

  <div style="display:flex;gap:14px;">
    <div style="flex-grow:1;background:#161B22;border:1px solid #21262D;border-left:3px solid #F85149;border-radius:10px;padding:17px 20px;display:flex;align-items:center;gap:26px;">
      <div><div style="font:700 17px %s;color:#F85149;">1 programa no coincide</div>
      <div style="font:400 13px %s;color:#8B949E;margin-top:3px;">La guía anuncia algo distinto de lo que va a salir al aire</div></div>
      <div style="flex-grow:1;"></div>
      <div style="display:flex;flex-direction:column;gap:9px;">%s%s%s</div>
      <div style="background:#22D3EE;color:#06141a;border-radius:8px;padding:11px 20px;font:600 14px %s;">Regenerar la guía</div>
    </div>
  </div>

  <div style="position:relative;height:26px;">%s</div>

  <div style="display:flex;flex-direction:column;gap:12px;">
    <div style="display:flex;align-items:center;gap:12px;">
      <span style="width:150px;flex-shrink:0;font:600 11.5px %s;letter-spacing:1.1px;color:#6E7681;">LA GUÍA DICE</span>
      <div style="height:1px;flex-grow:1;background:#21262D;"></div></div>
    <div style="display:flex;">%s</div>
    <div style="display:flex;align-items:center;gap:12px;margin-top:6px;">
      <span style="width:150px;flex-shrink:0;font:600 11.5px %s;letter-spacing:1.1px;color:#22D3EE;">LO QUE VA A SALIR</span>
      <div style="height:1px;flex-grow:1;background:#21262D;"></div></div>
    <div style="display:flex;">%s</div>
  </div>

  <div style="background:#161B22;border:1px solid #21262D;border-radius:10px;padding:20px 24px;margin-top:8px;">
    <div style="font:600 11.5px %s;letter-spacing:1.2px;color:#6E7681;margin-bottom:14px;">POR QUÉ NO COINCIDEN</div>
    <div style="display:flex;align-items:center;gap:18px;">
      <div style="width:52px;height:52px;border-radius:9px;background:rgba(248,81,73,.14);display:flex;align-items:center;justify-content:center;flex-shrink:0;">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#F85149" stroke-width="2" stroke-linecap="round"><path d="M12 8v5M12 17h.01"/><circle cx="12" cy="12" r="9"/></svg></div>
      <div style="flex-grow:1;">
        <div style="font:500 15px %s;color:#E6EDF3;line-height:1.5;">La regla de <strong style="color:#F85149;">Magic Knight Rayearth</strong> venció el 7 de septiembre y desde el 8 <strong style="color:#22D3EE;">Zoids</strong> ocupa las 3:00 PM. La guía se publicó antes del relevo y quedó anunciando el programa viejo.</div>
        <div style="font:400 13px %s;color:#8B949E;margin-top:7px;">Regenerar la guía lo arregla. Antena787 la revalida sola cada vez que cambia el plan.</div></div></div></div>

  <div style="display:flex;align-items:center;gap:11px;margin-top:2px;">
    <span style="display:inline-block;width:7px;height:7px;border-radius:50%%;background:#3FB950;"></span>
    <span style="font:400 13px %s;color:#8B949E;">Publicada en <span class="mono" style="color:#22D3EE;">/xmltv.xml</span> · identificador de canal <span class="mono" style="color:#E6EDF3;">catv.pr</span> · revalidada hace 6 minutos</span></div>
</div></div>'''%(NAV('Parrilla'),FONT,FONT,tabs('Guía'),FONT,FONT,
   check(True,'Identificador de canal correcto'),check(True,'168 programas con hora y duración'),check(False,'1 programa no coincide con el plan'),
   FONT,horas,FONT,fila(GUIA,'g'),FONT,fila(PLAN,'p'),FONT,FONT,FONT,FONT)+FOOT
io.open('Guia.dc.html','w',encoding='utf-8').write(guia)
print('Guia ok')

# ══════════ ANUNCIANTES ══════════
carga=[0,0,0,0,0,0,3,5,7,9,12,12,11,10,8,9,11,12,12,10,7,4,2,0]  # min vendidos de 12
horas_html=''
for h,m in enumerate(carga):
    pct=m/12*100
    if m==0: col='#21262D'
    elif m>=11: col='#F85149'
    elif m>=8: col='#D29922'
    else: col='#3FB950'
    lbl=('%d'%(h if h<=12 else h-12)) if h%3==0 else ''
    horas_html+='<div style="flex-grow:1;display:flex;flex-direction:column;align-items:center;gap:5px;"><div style="width:100%%;height:96px;background:#12161c;border-radius:4px;display:flex;flex-direction:column;justify-content:flex-end;overflow:hidden;"><div style="height:%.1f%%;background:%s;border-radius:3px;"></div></div><span class="mono" style="font-size:9.5px;color:#6E7681;height:12px;">%s</span></div>'%(pct,col,lbl)

CLI=[('Ferretería del Este','#2b3346',18,'30 seg',540,'#3FB950'),
     ('Pizzería Borinquen','#3a2226',24,'15 seg',360,'#3FB950'),
     ('Óptica Central','#1f3a37',12,'60 seg',720,'#D29922'),
     ('Muebles Aguadilla','#33253a',8,'30 seg',240,'#3FB950')]
cli_html=''
for n,c,cant,dur,seg,est in CLI:
    ini=''.join(p[0] for p in n.split()[:2]).upper()
    cli_html+='''<div style="display:flex;align-items:center;gap:15px;background:#161B22;border:1px solid #21262D;border-radius:10px;padding:14px 18px;">
  <div style="width:42px;height:42px;border-radius:9px;background:linear-gradient(145deg,%s,#161a20);display:flex;align-items:center;justify-content:center;flex-shrink:0;"><span style="font:700 15px %s;color:rgba(255,255,255,.42);">%s</span></div>
  <div style="flex-grow:1;"><div style="font:600 15px %s;color:#E6EDF3;">%s</div>
  <div style="font:400 12.5px %s;color:#8B949E;margin-top:2px;">%s spots de %s · %s seg al mes</div></div>
  <div style="text-align:right;"><div style="display:flex;align-items:center;gap:7px;justify-content:flex-end;"><span style="display:inline-block;width:7px;height:7px;border-radius:50%%;background:%s;"></span><span style="font:500 12.5px %s;color:%s;">%s de %s salieron</span></div>
  <div style="font:400 11.5px %s;color:#6E7681;margin-top:3px;">reporte enviado el 1 de sep</div></div></div>'''%(c,FONT,ini,FONT,n,FONT,cant,dur,seg,est,FONT,est,cant,cant,FONT)

anun=HEAD+'''<div style="display:flex;width:1440px;height:900px;background:#0E1116;">
%s
<div style="flex-grow:1;padding:26px 32px;display:flex;flex-direction:column;gap:20px;overflow:hidden;">
  <div style="display:flex;align-items:flex-end;justify-content:space-between;">
    <div><div style="font:700 24px %s;letter-spacing:-.4px;color:#E6EDF3;">Anuncios</div>
    <div style="font:400 13.5px %s;color:#8B949E;margin-top:3px;">Septiembre 2026 · 12 minutos por hora es tu tope</div></div>
    <div style="display:flex;align-items:center;gap:9px;background:#22D3EE;color:#06141a;border-radius:8px;padding:11px 20px;font:600 14.5px %s;">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#06141a" stroke-width="2.4" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>Nuevo cliente</div></div>

  <div style="display:flex;gap:16px;">
    <div style="flex-grow:1;background:#161B22;border:1px solid #21262D;border-radius:10px;padding:20px 24px;">
      <div style="display:flex;align-items:baseline;justify-content:space-between;">
        <span style="font:600 11.5px %s;letter-spacing:1.2px;color:#6E7681;">INVENTARIO DEL MES</span>
        <span style="font:400 12.5px %s;color:#8B949E;"><span class="mono" style="color:#E6EDF3;font-size:14px;">1,860</span> de <span class="mono">8,640</span> minutos vendidos</span></div>
      <div style="height:10px;border-radius:5px;background:#12161c;margin-top:14px;overflow:hidden;"><div style="width:21.5%%;height:100%%;background:linear-gradient(90deg,#22D3EE,#3FB950);border-radius:5px;"></div></div>
      <div style="display:flex;justify-content:space-between;margin-top:9px;">
        <span style="font:600 13px %s;color:#22D3EE;">21.5%% vendido</span>
        <span style="font:400 12.5px %s;color:#8B949E;">te quedan <span class="mono" style="color:#E6EDF3;">6,780</span> minutos por vender</span></div></div>
    <div style="width:250px;flex-shrink:0;background:#161B22;border:1px solid #21262D;border-radius:10px;padding:20px 22px;">
      <div style="font:600 11.5px %s;letter-spacing:1.2px;color:#6E7681;">EVIDENCIA DE EMISIÓN</div>
      <div style="font:700 30px %s;color:#3FB950;margin-top:11px;letter-spacing:-.5px;">100%%</div>
      <div style="font:400 12.5px %s;color:#8B949E;margin-top:4px;line-height:1.45;">62 de 62 spots salieron.<br>Ninguno tapado por alerta.</div></div></div>

  <div style="background:#161B22;border:1px solid #21262D;border-radius:10px;padding:20px 24px;">
    <div style="display:flex;align-items:baseline;justify-content:space-between;margin-bottom:15px;">
      <span style="font:600 11.5px %s;letter-spacing:1.2px;color:#6E7681;">CUÁNTO LE CABE A CADA HORA</span>
      <div style="display:flex;gap:18px;">
        <span style="display:flex;align-items:center;gap:6px;"><span style="width:9px;height:9px;border-radius:2px;background:#3FB950;display:inline-block;"></span><span style="font:400 12px %s;color:#8B949E;">hay espacio</span></span>
        <span style="display:flex;align-items:center;gap:6px;"><span style="width:9px;height:9px;border-radius:2px;background:#D29922;display:inline-block;"></span><span style="font:400 12px %s;color:#8B949E;">casi llena</span></span>
        <span style="display:flex;align-items:center;gap:6px;"><span style="width:9px;height:9px;border-radius:2px;background:#F85149;display:inline-block;"></span><span style="font:400 12px %s;color:#8B949E;">llena</span></span></div></div>
    <div style="display:flex;gap:5px;align-items:flex-end;">%s</div>
    <div style="font:400 12.5px %s;color:#8B949E;margin-top:12px;">De medianoche a 6:00 AM no hay nada vendido — son las horas que hoy salen en negro.</div></div>

  <div style="flex-grow:1;display:flex;flex-direction:column;gap:11px;">
    <div style="display:flex;align-items:baseline;justify-content:space-between;">
      <span style="font:600 15.5px %s;color:#E6EDF3;">Clientes</span>
      <span style="font:500 12.5px %s;color:#22D3EE;">enviar reportes de septiembre</span></div>
    %s</div>
</div></div>'''%(NAV('Anuncios',ads=True),FONT,FONT,FONT,FONT,FONT,FONT,FONT,FONT,FONT,FONT,FONT,FONT,FONT,FONT,horas_html,FONT,FONT,FONT,cli_html)
# OBSOLETO: Anunciantes.dc.html lo genera mk_anuncios.py. Se deja el bloque como referencia
# pero no se escribe, para no pisar la version vigente al correr gen.py.
# io.open('Anunciantes.dc.html','w',encoding='utf-8').write(anun)
print('Anunciantes ok')

# ══════════ EN VIVO (lección de OBS) ══════════
def meter(pct, peak, lab):
    segs=''
    for i in range(28):
        y=(27-i)/27*100
        on = y <= pct
        if y>92: c='#F85149'
        elif y>78: c='#D29922'
        else: c='#3FB950'
        segs+='<div style="height:6px;border-radius:1px;background:%s;"></div>'%(c if on else '#1a1f27')
    return '''<div style="display:flex;flex-direction:column;align-items:center;gap:7px;">
  <div style="position:relative;width:26px;display:flex;flex-direction:column;gap:2px;padding:4px;background:#0d1117;border:1px solid #21262D;border-radius:4px;">%s
  <div style="position:absolute;left:0;right:0;top:%.1f%%;height:2px;background:#E6EDF3;"></div></div>
  <span class="mono" style="font-size:11px;color:#8B949E;">%s</span></div>'''%(segs,(100-peak)*0.92+2,lab)
escala=''.join('<div style="font-family:ui-monospace,monospace;font-size:9.5px;color:#4d545c;height:%spx;">%s</div>'%(28 if v!=-60 else 0,v) for v in [0,-6,-12,-20,-30,-45,-60])
def stat(lab,val,col='#E6EDF3',sub=''):
    s='<div style="font:400 11px %s;color:#6E7681;margin-top:2px;">%s</div>'%(FONT,sub) if sub else ''
    return '<div style="flex-grow:1;"><div style="font:600 10.5px %s;letter-spacing:1px;color:#6E7681;">%s</div><div class="mono" style="font-size:21px;color:%s;margin-top:6px;font-weight:500;">%s</div>%s</div>'%(FONT,lab,col,val,s)
def dest(n,proto,ok,extra):
    c='#3FB950' if ok else '#6E7681'
    return '''<div style="flex-grow:1;background:#12161c;border:1px solid #21262D;border-radius:8px;padding:14px 16px;display:flex;align-items:center;gap:12px;">
  <span style="display:inline-block;width:9px;height:9px;border-radius:50%%;background:%s;flex-shrink:0;"></span>
  <div style="flex-grow:1;"><div style="font:600 14px %s;color:#E6EDF3;">%s</div><div class="mono" style="font-size:11px;color:#6E7681;margin-top:2px;">%s</div></div>
  <span class="mono" style="font-size:12px;color:%s;">%s</span></div>'''%(c,FONT,n,proto,c,extra)

vivo=HEAD+'''<div style="display:flex;width:1440px;height:900px;background:#0E1116;">
%s
<div style="flex-grow:1;padding:26px 32px;display:flex;flex-direction:column;gap:18px;overflow:hidden;">

  <div style="display:flex;align-items:center;justify-content:space-between;">
    <div style="display:flex;align-items:center;gap:11px;">
      <span style="display:inline-block;width:11px;height:11px;border-radius:50%%;background:#FF3B30;animation:tally 1.6s ease-in-out infinite;"></span>
      <span style="font:700 20px %s;letter-spacing:1.6px;color:#E6EDF3;">AL AIRE · FUENTE EN VIVO</span></div>
    <div style="display:flex;align-items:center;gap:9px;background:#161B22;border:1px solid #21262D;border-radius:20px;padding:8px 16px;">
      <span style="display:inline-block;width:8px;height:8px;border-radius:50%%;background:#3FB950;"></span>
      <span style="font:500 13.5px %s;color:#8B949E;">OBS conectado · empujando a Antena787</span></div></div>

  <div style="display:flex;gap:20px;">
    <div style="flex-grow:1;">
      <div style="position:relative;height:404px;border-radius:12px;overflow:hidden;border:2px solid #22D3EE;background:linear-gradient(155deg,#0f3a44,#16202b 58%%,#10141a);">
        <div style="position:absolute;inset:0;background:radial-gradient(ellipse at 42%% 46%%,rgba(34,211,238,.13),transparent 60%%);"></div>
        <div style="position:absolute;left:0;right:0;bottom:0;height:160px;background:linear-gradient(transparent,rgba(6,9,13,.93));"></div>
        <div style="position:absolute;top:14px;left:16px;display:flex;align-items:center;gap:7px;background:rgba(6,9,13,.7);border-radius:6px;padding:6px 11px;">
          <span style="display:inline-block;width:7px;height:7px;border-radius:50%%;background:#FF3B30;animation:tally 1.6s ease-in-out infinite;"></span>
          <span class="mono" style="font-size:11px;letter-spacing:1.1px;color:#E6EDF3;">RECIBIENDO</span></div>
        <div style="position:absolute;top:14px;right:16px;background:rgba(6,9,13,.7);border-radius:6px;padding:6px 11px;">
          <span class="mono" style="font-size:11px;color:#8B949E;">1920×1080 · 29.97p</span></div>
        <div style="position:absolute;left:22px;right:22px;bottom:20px;">
          <div style="font:700 30px %s;letter-spacing:-.5px;color:#E6EDF3;">RadioOnce Live!</div>
          <div style="font:400 14px %s;color:#8B949E;margin-top:5px;">10:00 AM – 1:00 PM &nbsp;·&nbsp; lunes a viernes</div>
          <div style="display:flex;align-items:center;gap:14px;margin-top:14px;">
            <div style="flex-grow:1;height:5px;border-radius:3px;background:rgba(230,237,243,.14);overflow:hidden;"><div style="width:58%%;height:100%%;background:#22D3EE;border-radius:3px;"></div></div>
            <span class="mono" style="font-size:14px;color:#E6EDF3;">faltan 1:15:40</span></div></div></div>
    </div>

    <div style="width:300px;flex-shrink:0;background:#161B22;border:1px solid #21262D;border-radius:11px;padding:18px 20px;display:flex;flex-direction:column;">
      <div style="font:600 11.5px %s;letter-spacing:1.2px;color:#6E7681;">AUDIO QUE ESTÁ LLEGANDO</div>
      <div style="display:flex;gap:16px;margin-top:16px;justify-content:center;align-items:flex-start;">
        <div style="display:flex;flex-direction:column;align-items:flex-end;padding-top:3px;">%s</div>
        %s%s</div>
      <div style="margin-top:16px;padding-top:15px;border-top:1px solid #21262D;">
        <div style="display:flex;align-items:center;gap:9px;"><span style="display:inline-block;width:8px;height:8px;border-radius:50%%;background:#3FB950;"></span>
        <span style="font:500 13.5px %s;color:#E6EDF3;">Audio normal</span></div>
        <div style="font:400 12px %s;color:#8B949E;margin-top:6px;line-height:1.45;">Si esto se queda en silencio más de 15 segundos, entra el relleno y te llega una alerta.</div></div></div>
  </div>

  <div style="display:flex;gap:14px;background:#161B22;border:1px solid #21262D;border-radius:11px;padding:18px 24px;">
    %s%s%s%s</div>

  <div>
    <div style="font:600 11.5px %s;letter-spacing:1.2px;color:#6E7681;margin-bottom:10px;">ESTA SEÑAL SALE A</div>
    <div style="display:flex;gap:12px;">%s%s%s</div>
    <div style="font:400 12.5px %s;color:#8B949E;margin-top:11px;">OBS empuja una sola vez, a Antena787. Si YouTube se cae, el transmisor ni se entera.</div></div>
</div></div>'''%(NAV('Al aire'),FONT,FONT,FONT,FONT,FONT,escala,meter(72,84,'IZQ'),meter(66,79,'DER'),FONT,FONT,
  stat('BITRATE','6.2 Mbps','#3FB950','estable hace 1h 44m'),
  stat('CUADROS PERDIDOS','0','#3FB950','de 189,420 enviados'),
  stat('TIEMPO AL AIRE','1:44:20','#E6EDF3','empezó a las 10:00 AM'),
  stat('RETRASO','1.2 s','#3FB950','OBS → transmisor'),
  FONT, dest('Transmisor','UDP-TS · 239.1.1.10:5000',True,'al aire'),
  dest('YouTube','RTMP · RadioOnce PR',True,'1,204 viendo'),
  dest('Facebook','RTMP · apagada',False,'—'), FONT)+FOOT
io.open('EnVivo.dc.html','w',encoding='utf-8').write(vivo)
print('EnVivo ok')

# ══════════ REGLAS (regenerada con NAV correcto) ══════════
def pills(pat):
    o=[]
    for i,d in enumerate('LMMJVSD'):
        on=pat[i]!='_'
        sty='background:rgba(34,211,238,.16);color:#22D3EE;' if on else 'background:#1a1f27;color:#3d444d;'
        o.append('<span style="display:inline-flex;align-items:center;justify-content:center;width:21px;height:21px;border-radius:5px;font:600 11px %s;%s">%s</span>'%(FONT,sty,d))
    return '<div style="display:flex;gap:3px;">'+''.join(o)+'</div>'
CARDS=[('Kojak','LMMJV__','8:00 AM','27 jul – 4 ene 2027','122 días','#3FB950',None),
 ('Familia Robinson','_____SD','9:00 AM','21 mar – 6 sep','2 días','#F85149','vence'),
 ('Los Simuladores','LMMJV__','1:00 PM','11 ago – 11 sep','7 días','#D29922',None),
 ('RadioOnce Live!','LMMJV__','10:00 AM','3 sep – 31 dic','118 días','#3FB950','vivo'),
 ('Gaming Longplays','LMMJV__','6:00 PM','31 ago – 28 sep','24 días','#3FB950','eps'),
 ('Get Smart','LMMJV__','6:00 AM','7 mar – 16 sep','12 días','#D29922',None),
 ('Zoids','LMMJV__','3:00 PM','8 sep – 9 dic','96 días','#3FB950','releva'),
 ("Gilligan's Island",'LMMJV__','9:30 AM','4 sep – 19 ene 2027','137 días','#3FB950',None),
 ('Comics 9th Art','LMMJV__','9:00 AM','4 sep – 22 sep','18 días','#D29922',None),
 ("You're Under Arrest",'LMMJV__','11:00 PM','7 jul – 16 sep','12 días','#D29922','repe'),
 ('Mazinger Z','LMMJV__','3:30 PM','10 jun – 15 oct','41 días','#3FB950',None),
 ('Zorro 57','LMMJV__','4:30 PM','2 jul – 21 oct','47 días','#3FB950',None)]
BADGE={'vivo':('EN VIVO','#22D3EE','rgba(34,211,238,.14)'),'eps':('×10 episodios','#8B949E','#1a1f27'),
 'releva':('releva a Magic Knight','#8B949E','#1a1f27'),'repe':('2.º pase del día','#8B949E','#1a1f27'),
 'vence':('RENOVAR YA','#F85149','rgba(248,81,73,.15)')}
cards=[]
for t,pat,hora,rango,quedan,color,bd in CARDS:
    badge=''
    if bd:
        lab,c,bg=BADGE[bd]
        badge='<div style="margin-top:11px;"><span style="display:inline-block;background:%s;color:%s;border-radius:4px;padding:3px 8px;font:600 10.5px %s;letter-spacing:.3px;">%s</span></div>'%(bg,c,FONT,lab)
    borde='border:1px solid rgba(248,81,73,.55);' if bd=='vence' else 'border:1px solid #21262D;'
    cards.append('''<div style="background:#161B22;%sborder-radius:10px;overflow:hidden;">
  <div style="height:86px;">%s</div>
  <div style="padding:13px 15px 15px;">
    <div style="display:flex;align-items:baseline;justify-content:space-between;gap:8px;">
      <span style="font:600 15px %s;color:#E6EDF3;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">%s</span>
      <span class="mono" style="font-size:13px;color:#22D3EE;flex-shrink:0;">%s</span></div>
    <div style="margin-top:11px;">%s</div>
    <div style="display:flex;align-items:center;gap:8px;margin-top:12px;">
      <span style="display:inline-block;width:8px;height:8px;border-radius:50%%;background:%s;flex-shrink:0;"></span>
      <span style="font:500 12.5px %s;color:%s;">quedan %s</span></div>
    <div class="mono" style="font-size:11px;color:#6E7681;margin-top:5px;">%s</div>%s</div></div>'''%(
    borde,art(t,fs=24,r=0),FONT,t,hora,pills(pat),color,FONT,color,quedan,rango,badge))
reg=HEAD+'''<div style="display:flex;width:1440px;height:900px;background:#0E1116;">
%s
<div style="flex-grow:1;padding:26px 32px;display:flex;flex-direction:column;gap:18px;overflow:hidden;">
  <div style="display:flex;align-items:flex-end;justify-content:space-between;">
    <div><div style="font:700 24px %s;letter-spacing:-.4px;color:#E6EDF3;">Reglas de programación</div>
    <div style="font:400 13.5px %s;color:#8B949E;margin-top:3px;">35 reglas arman la semana completa. La parrilla sale de aquí.</div></div>
    <div style="display:flex;align-items:center;gap:9px;background:#22D3EE;color:#06141a;border-radius:8px;padding:11px 20px;font:600 14.5px %s;">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#06141a" stroke-width="2.4" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>Nueva regla</div></div>
  <div style="display:flex;gap:8px;">
    <span style="background:#161B22;border:1px solid #22D3EE;color:#E6EDF3;border-radius:7px;padding:7px 15px;font:500 13px %s;">Todas · 35</span>
    <span style="background:#161B22;border:1px solid rgba(248,81,73,.45);color:#F85149;border-radius:7px;padding:7px 15px;font:500 13px %s;">Se vencen pronto · 18</span>
    <span style="background:#161B22;border:1px solid #21262D;color:#8B949E;border-radius:7px;padding:7px 15px;font:500 13px %s;">En vivo · 1</span></div>
  <div style="display:grid;grid-template-columns:repeat(4, minmax(0, 1fr));gap:16px;">%s</div>
</div></div>'''%(NAV('Reglas'),FONT,FONT,FONT,FONT,FONT,FONT,''.join(cards))+FOOT
io.open('Reglas.dc.html','w',encoding='utf-8').write(reg)

# ══════════ BIBLIOTECA (regenerada) ══════════
def poster(t,hora=None):
    tag='<div style="position:absolute;top:7px;right:7px;background:rgba(34,211,238,.92);color:#06141a;border-radius:4px;padding:2px 7px;font:600 10px \'IBM Plex Mono\';">%s</div>'%hora if hora else ''
    return '<div style="width:133px;flex-shrink:0;"><div style="position:relative;height:200px;">%s%s</div><div style="font:500 12.5px %s;color:#E6EDF3;margin-top:8px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">%s</div></div>'%(art(t,fs=26,r=7),tag,FONT,t)
f1=''.join(poster(t,h) for t,h in [('Kojak','8:00'),('Comics 9th Art','9:00'),("Gilligan's Island",'9:30'),('RadioOnce Live!','10:00'),('Los Simuladores','1:00'),("You're Under Arrest",'2:00'),('Zorro 57','4:30')])
f2=''.join(poster(t) for t in ['Starsky & Hutch','The Time Tunnel','Land of the Giants','Serial Experiments Lain','Cybersix','The Munsters','Space Cobra'])
bib=HEAD+'''<div style="display:flex;width:1440px;height:900px;background:#0E1116;">
%s
<div style="flex-grow:1;padding:24px 30px;display:flex;flex-direction:column;gap:20px;overflow:hidden;">
  <div style="display:flex;align-items:center;gap:16px;">
    <div style="flex-grow:1;display:flex;align-items:center;gap:11px;background:#161B22;border:1px solid #21262D;border-radius:9px;padding:11px 16px;">
      <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#6E7681" stroke-width="2" stroke-linecap="round"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
      <span style="font:400 14px %s;color:#6E7681;">Buscar en 118 títulos…</span></div>
    <div style="display:flex;align-items:center;gap:9px;background:#161B22;border:1px solid #21262D;border-radius:9px;padding:11px 17px;">
      <span style="display:inline-block;width:8px;height:8px;border-radius:50%%;background:#3FB950;"></span>
      <span style="font:500 13.5px %s;color:#8B949E;">118 títulos · 1.2 TB · importados de la carpeta una vez</span></div></div>
  <div style="position:relative;height:232px;border-radius:12px;overflow:hidden;border:1px solid #21262D;background:linear-gradient(115deg,#2b3346 0%%,#1a1f2b 48%%,#0f1218 100%%);flex-shrink:0;">
    <div style="position:absolute;inset:0;background:radial-gradient(ellipse at 78%% 30%%,rgba(34,211,238,.13),transparent 58%%);"></div>
    <div style="position:absolute;left:0;top:0;bottom:0;width:62%%;background:linear-gradient(90deg,rgba(11,14,18,.95) 30%%,transparent);"></div>
    <div style="position:absolute;left:30px;top:0;bottom:0;display:flex;flex-direction:column;justify-content:center;max-width:520px;">
      <span style="font:600 10.5px %s;letter-spacing:1.4px;color:#22D3EE;">AHORA EN LA PARRILLA</span>
      <div style="font:700 36px %s;letter-spacing:-.7px;color:#E6EDF3;margin-top:9px;">Kojak</div>
      <div style="font:400 13.5px %s;color:#8B949E;margin-top:9px;line-height:1.55;">El teniente Theo Kojak es el protagonista de este popular drama policial. Kojak es un policía duro, pero su marca personal es su afición por los chupetes.</div>
      <div style="display:flex;align-items:center;gap:14px;margin-top:14px;">
        <span class="mono" style="font-size:12.5px;color:#6E7681;">1973</span>
        <span style="border:1px solid #21262D;border-radius:4px;padding:2px 7px;font:500 11px %s;color:#8B949E;">TV-PG</span>
        <span class="mono" style="font-size:12.5px;color:#6E7681;">118 episodios · 1 hr</span>
        <span style="display:flex;align-items:center;gap:5px;"><span style="display:inline-block;width:7px;height:7px;border-radius:50%%;background:#3FB950;"></span><span style="font:500 12.5px %s;color:#3FB950;">regla hasta ene 2027</span></span></div></div></div>
  <div>
    <div style="display:flex;align-items:baseline;justify-content:space-between;margin-bottom:11px;">
      <span style="font:600 15.5px %s;color:#E6EDF3;">En la parrilla esta semana</span>
      <span style="font:500 12.5px %s;color:#6E7681;">35 títulos</span></div>
    <div style="display:flex;gap:16px;">%s</div></div>
  <div>
    <div style="display:flex;align-items:baseline;justify-content:space-between;margin-bottom:11px;">
      <span style="font:600 15.5px %s;color:#E6EDF3;">Sin programar</span>
      <span style="font:500 12.5px %s;color:#D29922;">83 títulos que no estás usando</span></div>
    <div style="display:flex;gap:16px;">%s</div></div>
</div></div>'''%(NAV('Biblioteca'),FONT,FONT,FONT,FONT,FONT,FONT,FONT,FONT,FONT,f1,FONT,FONT,f2)+FOOT
io.open('Biblioteca.dc.html','w',encoding='utf-8').write(bib)
print('Reglas + Biblioteca regeneradas')

# ══════════ AJUSTES ══════════
def chk(ok,t,sub=''):
    c='#3FB950' if ok else '#F85149'
    d='<path d="m5 12 5 5L19 7"/>' if ok else '<path d="M18 6 6 18M6 6l12 12"/>'
    s='<div style="font:400 11.5px %s;color:#6E7681;margin-top:2px;">%s</div>'%(FONT,sub) if sub else ''
    return '<div style="display:flex;align-items:flex-start;gap:10px;"><svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="%s" stroke-width="2.6" stroke-linecap="round" style="flex-shrink:0;margin-top:2px;">%s</svg><div><div style="font:500 13.5px %s;color:#E6EDF3;">%s</div>%s</div></div>'%(c,d,FONT,t,s)
def card(titulo,cuerpo,accion=None,borde='#21262D'):
    ac='<div style="margin-top:15px;"><span style="display:inline-block;border:1px solid #21262D;color:#E6EDF3;border-radius:7px;padding:9px 17px;font:500 13.5px %s;">%s</span></div>'%(FONT,accion) if accion else ''
    return '<div style="background:#161B22;border:1px solid %s;border-radius:11px;padding:20px 22px;"><div style="font:600 11.5px %s;letter-spacing:1.2px;color:#6E7681;margin-bottom:15px;">%s</div>%s%s</div>'%(borde,FONT,titulo,cuerpo,ac)
def kv(k,v,c='#E6EDF3'):
    return '<div style="display:flex;align-items:baseline;justify-content:space-between;padding:7px 0;"><span style="font:400 13.5px %s;color:#8B949E;">%s</span><span style="font:500 13.5px %s;color:%s;">%s</span></div>'%(FONT,k,FONT,c,v)
def toggle(on):
    return '<div style="width:38px;height:22px;border-radius:11px;background:%s;position:relative;flex-shrink:0;"><div style="position:absolute;top:3px;%s:3px;width:16px;height:16px;border-radius:50%%;background:%s;"></div></div>'%('#22D3EE' if on else '#21262D','right' if on else 'left','#06141a' if on else '#6E7681')
def kv_edit(k,v,u):
    return '<div style="display:flex;align-items:center;justify-content:space-between;padding:9px 0 2px;"><span style="font:400 13.5px %s;color:#8B949E;">%s</span><span style="display:inline-flex;align-items:baseline;gap:4px;border:1px solid #21262D;background:#0E1116;border-radius:6px;padding:5px 11px;"><span class="mono" style="font-size:13.5px;color:#E6EDF3;">%s</span><span style="font:400 11.5px %s;color:#6E7681;">%s</span></span></div>'%(FONT,k,v,FONT,u)
def row_tog(t,sub,on):
    return '<div style="display:flex;align-items:center;gap:14px;padding:11px 0;border-bottom:1px solid #1a1f27;"><div style="flex-grow:1;"><div style="font:500 14px %s;color:#E6EDF3;">%s</div><div style="font:400 12px %s;color:#6E7681;margin-top:2px;">%s</div></div>%s</div>'%(FONT,t,FONT,sub,toggle(on))

salud=''.join([chk(True,'Exclusiones de antivirus','Carpetas de video y la base de datos, fuera del escaneo'),
 chk(True,'Plan de energía','Sin suspensión, sin apagado de disco'),
 chk(True,'Arranque tras corte de luz','Habilitado en el BIOS'),
 chk(True,'Rutas largas','Nombres de más de 260 caracteres permitidos'),
 chk(False,'Actualizaciones de Windows','Está en automático — puede reiniciar la máquina al aire')])
salud='<div style="display:grid;grid-template-columns:1fr 1fr;gap:13px 22px;">'+salud+'</div>'

ajustes=HEAD+'''<div style="display:flex;width:1440px;height:900px;background:#0E1116;">
%s
<div style="flex-grow:1;padding:26px 32px;display:flex;flex-direction:column;gap:18px;overflow:hidden;">
  <div style="display:flex;align-items:flex-end;justify-content:space-between;">
    <div><div style="font:700 24px %s;letter-spacing:-.4px;color:#E6EDF3;">Ajustes</div>
    <div style="font:400 13.5px %s;color:#8B949E;margin-top:3px;">Antena787 v1.0.0 · Windows 10 Pro · al aire hace 34 días sin interrupciones</div></div>
    <div style="display:flex;align-items:center;gap:9px;background:#161B22;border:1px solid rgba(248,81,73,.45);border-radius:20px;padding:8px 16px;">
      <span style="display:inline-block;width:8px;height:8px;border-radius:50%%;background:#F85149;"></span>
      <span style="font:500 13.5px %s;color:#E6EDF3;">1 cosa que arreglar</span></div></div>

  <div style="display:grid;grid-template-columns:repeat(4, minmax(0, 1fr));gap:14px;flex-grow:1;align-content:start;">
    <div style="grid-column:span 2;">%s</div>
    %s
    %s
    <div style="grid-column:span 2;">%s</div>
    %s
    %s
    %s
    %s
    <div style="grid-column:span 2;">%s</div>
  </div>
</div></div>'''%(NAV('Ajustes'),FONT,FONT,FONT,
 card('SALUD DE LA MÁQUINA',salud,'Arreglar las actualizaciones de Windows','rgba(248,81,73,.45)'),
 card('RESPALDO', kv('Último','hace 41 minutos','#3FB950')+kv('Cada','1 hora')+kv('Copias guardadas','168 · 7 días')+kv('Tamaño','12.4 MB'), 'Bajar una copia'),
 card('ACTUALIZACIONES', kv('Versión','1.0.0')+kv('Hay disponible','1.0.1','#D29922')+kv('Instalar sola','nunca','#3FB950')+'<div style="font:400 11.5px %s;color:#6E7681;margin-top:9px;line-height:1.45;">Un canal al aire no se actualiza solo. Tú escoges cuándo, con la caja de respaldo lista.</div>'%FONT, 'Ver qué cambia'),
 card('CUMPLIMIENTO', kv('País','Estados Unidos')+kv('Perfil','FCC · ATSC')+kv('Volumen','volumen de televisión de EE. UU.','#3FB950')+kv('Subtítulos','se conservan','#3FB950')+kv('Equipo de alertas','Sage ENDEC · por red','#3FB950')),
 card('ACCESO REMOTO', kv('Tailscale','conectado','#3FB950')+kv('Dirección','antena787-catv')+kv('Puertos abiertos','ninguno','#3FB950')+'<div style="font:400 11.5px %s;color:#6E7681;margin-top:9px;line-height:1.45;">Rolando entra desde el celular sin abrir nada al internet.</div>'%FONT),
 card('HORA', kv('Sincronizada con','time.nist.gov','#3FB950')+kv('Desvío','0.08 s','#3FB950')+'<div style="font:400 11.5px %s;color:#6E7681;margin-top:9px;line-height:1.45;">Avisa si pasa de 1 s.</div>'%FONT),
 card('ACELERACIÓN DE VIDEO', kv('Tarjeta','NVIDIA')+kv('Probada','al arrancar','#3FB950')+kv('Resultado','OK','#3FB950')+'<div style="font:400 11.5px %s;color:#6E7681;margin-top:9px;line-height:1.45;">Se prueba sola cada vez que arranca.</div>'%FONT),
 card('DETECTOR DE SILENCIO', row_tog('Avisa y devuelve el control','Si no sale señal más de 15 s',True)+kv_edit('Umbral','15','s')),
 card('ASISTENTE DE IA', row_tog('Conectar un asistente','Apagado. Antena787 funciona completa sin esto.',False)+'<div style="font:400 11.5px %s;color:#6E7681;margin-top:11px;line-height:1.5;">Si lo enciendes, el asistente puede leer y proponer cambios — nunca poner algo al aire ni facturar.</div>'%FONT))+FOOT
io.open('Ajustes.dc.html','w',encoding='utf-8').write(ajustes)
print('Ajustes ok')
