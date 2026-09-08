import io
F="'IBM Plex Sans'"
ICONS={'Al aire':'<path d="M12 20v-7"/><path d="M8.5 13 12 4l3.5 9"/><path d="M4.9 16.5a9 9 0 0 1 0-9"/><path d="M19.1 7.5a9 9 0 0 1 0 9"/>',
 'Parrilla':'<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18M8 3v4M16 3v4"/>',
 'Reglas':'<rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/>',
 'Biblioteca':'<rect x="3" y="3" width="7" height="9" rx="1.5"/><rect x="14" y="3" width="7" height="9" rx="1.5"/><rect x="3" y="15" width="7" height="6" rx="1.5"/><rect x="14" y="15" width="7" height="6" rx="1.5"/>',
 'Anuncios':'<path d="M20.6 13.4 13.4 20.6a2 2 0 0 1-2.8 0l-7.2-7.2A2 2 0 0 1 2.8 12V4.8A2 2 0 0 1 4.8 2.8H12a2 2 0 0 1 1.4.6l7.2 7.2a2 2 0 0 1 0 2.8Z"/><circle cx="7.5" cy="7.5" r="1.2"/>',
 'Ajustes':'<circle cx="12" cy="12" r="3.2"/><path d="M12 2.6v2.6M12 18.8v2.6M21.4 12h-2.6M5.2 12H2.6M18.6 5.4l-1.8 1.8M7.2 16.8l-1.8 1.8M18.6 18.6l-1.8-1.8M7.2 7.2 5.4 5.4"/>'}
def NAV(act):
    rows=[]
    for lab,ic in ICONS.items():
        on=(lab==act); bg='background:#161B22;' if on else ''
        col='#E6EDF3' if on else '#8B949E'; st='#22D3EE' if on else '#6E7681'
        bar='<div style="position:absolute;left:0;top:9px;bottom:9px;width:3px;border-radius:0 3px 3px 0;background:#22D3EE;"></div>' if on else ''
        rows.append('<div style="position:relative;display:flex;align-items:center;gap:12px;padding:11px 14px;border-radius:8px;%s">%s<svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="%s" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">%s</svg><span style="font:%s 14.5px %s,system-ui;color:%s;">%s</span></div>'%(bg,bar,st,ic,'600' if on else '400',F,col,lab))
    return '<div style="width:224px;flex-shrink:0;background:#0B0E12;border-right:1px solid #21262D;display:flex;flex-direction:column;padding:22px 0;"><div style="padding:0 20px 26px;display:flex;align-items:center;gap:10px;"><svg width="25" height="25" viewBox="0 0 24 24" fill="none" stroke="#22D3EE" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">%s</svg><span style="font:600 17px %s,system-ui;color:#E6EDF3;letter-spacing:-.3px;">Antena<span style="color:#22D3EE;">787</span></span></div><div style="display:flex;flex-direction:column;gap:3px;padding:0 12px;flex-grow:1;">%s</div><div style="padding:16px 20px 0;border-top:1px solid #21262D;margin:0 12px;"><div style="font:500 12px %s;color:#6E7681;letter-spacing:.4px;">CANAL</div><div style="font:500 14px %s;color:#8B949E;margin-top:5px;">Caribbean Advantage TV</div></div></div>'%(ICONS['Al aire'],F,''.join(rows),F,F)

COL={'Get Smart':'#39301e','I Dream of Jeannie':'#2e3a24','Tarzan':'#26361f','Kojak':'#2b3346','Comics 9th Art':'#3a2a3f',
"Gilligan's Island":'#1f3a37','RadioOnce Live!':'#0f3a44','Los Simuladores':'#3a2226',"You're Under Arrest":'#2a3348',
'Samurai X':'#3d2420','Zoids':'#243b30','Mazinger Z':'#3c2a1c','Green Hornet':'#1f3a2b','Zorro 57':'#3b2f1a',
'Los Lorcanitos':'#3a2a20','Gaming Longplays':'#2a2545','Carmen Sandiego':'#3a2436','Don Quijote':'#34301f',
'TinTin':'#1e3444','Familia Robinson':'#2e3a24','Sonic':'#1d2f4a','Astroboy':'#33253a','Corrector Yui':'#2b3a44',
'Voyagesea':'#20333f','SaberMarionette':'#312a3e',"BT'x":'#20333f'}
SEM=[('Get Smart',360,30),('I Dream of Jeannie',390,30),('Tarzan',420,60),('Kojak',480,60),('Comics 9th Art',540,30),
 ("Gilligan's Island",570,30),('RadioOnce Live!',600,180),('Los Simuladores',780,60),("You're Under Arrest",840,30),
 ('Samurai X',870,30),('Zoids',900,30),('Mazinger Z',930,30),('Green Hornet',960,30),('Zorro 57',990,30),
 ('Los Lorcanitos',1020,60),('Gaming Longplays',1080,300),("You're Under Arrest",1380,30),('Samurai X',1410,30)]
FIN=[('Carmen Sandiego',450,30),('Don Quijote',480,30),('TinTin',510,30),('Familia Robinson',540,30),('Sonic',570,30),
 ('Astroboy',600,30),('Corrector Yui',630,30),('Voyagesea',840,60),('Gaming Longplays',1080,300),
 ('SaberMarionette',1380,30),("BT'x",1410,30)]
SAB=[x for x in FIN if x[0]!='Familia Robinson']
M0,M1=360,1440; W=968.0; PX=W/(M1-M0)
VAC='repeating-linear-gradient(45deg,rgba(248,81,73,.5),rgba(248,81,73,.5) 4px,rgba(248,81,73,.16) 4px,rgba(248,81,73,.16) 8px)'
def fila(dia,prog,sel,vac):
    seg=''; cur=M0
    for t,st,du in sorted(prog,key=lambda x:x[1]):
        if st>cur: seg+='<div style="width:%.1fpx;flex-shrink:0;background:%s;border-radius:2px;margin-right:1px;"></div>'%((st-cur)*PX-1,VAC)
        w=du*PX-1
        lab='<span style="font:500 10px %s;color:rgba(230,237,243,.85);padding-left:5px;white-space:nowrap;overflow:hidden;">%s</span>'%(F,t) if w>68 else ''
        bd='box-shadow:inset 0 0 0 1.5px #22D3EE;' if t=='RadioOnce Live!' else ''
        seg+='<div style="width:%.1fpx;flex-shrink:0;background:%s;border-radius:2px;margin-right:1px;display:flex;align-items:center;overflow:hidden;%s">%s</div>'%(w,COL.get(t,'#2a3040'),bd,lab)
        cur=st+du
    if cur<M1: seg+='<div style="width:%.1fpx;flex-shrink:0;background:%s;border-radius:2px;"></div>'%((M1-cur)*PX,VAC)
    bg='background:rgba(34,211,238,.06);' if sel else ''
    vc='#F85149' if vac>=7 else '#D29922'
    return '<div style="display:flex;align-items:center;gap:11px;padding:5px 7px;border-radius:7px;%s"><span style="width:32px;flex-shrink:0;font:%s 13px %s;color:%s;">%s</span><div style="display:flex;height:40px;">%s</div><span style="width:72px;flex-shrink:0;text-align:right;font:500 11.5px %s;color:%s;">%s h vacías</span></div>'%(bg,'600' if sel else '400',F,'#E6EDF3' if sel else '#8B949E',dia,seg,F,vc,vac)
filas=''.join([fila('Dom',FIN,False,10),fila('Lun',SEM,False,5),fila('Mar',SEM,False,5),fila('Mié',SEM,False,5),
 fila('Jue',SEM,False,5),fila('Vie',SEM,True,5),fila('Sáb',SAB,False,10)])
horas=''.join('<div style="position:absolute;left:%.0fpx;top:0;"><div style="width:1px;height:7px;background:#21262D;"></div><span class="mono" style="font-size:10.5px;color:#6E7681;position:absolute;top:10px;left:-2px;white-space:nowrap;">%s</span></div>'%((h*60-M0)*PX,('%d AM'%h if h<12 else ('12 PM' if h==12 else '%d PM'%(h-12)))) for h in [6,8,10,12,14,16,18,20,22])
nowx=(13*60+45-M0)*PX
tabs=''.join('<span style="padding:8px 18px;border-radius:7px;font:%s 13.5px %s;%s">%s</span>'%('600' if t=='Semana' else '500',F,'background:#161B22;border:1px solid #22D3EE;color:#E6EDF3;' if t=='Semana' else 'border:1px solid #21262D;color:#8B949E;',t) for t in ['Semana','Mes','Guía'])
HEAD='''<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <script src="./support.js"></script>
</head>
<body>
<x-dc>
<helmet>
  <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=IBM+Plex+Sans:wght@400;500;600;700&family=IBM+Plex+Mono:wght@400;500;600&display=swap">
  <style>
    body { margin:0; background:#0E1116; font-family:'IBM Plex Sans',system-ui,sans-serif; color:#E6EDF3; -webkit-font-smoothing:antialiased; }
    a { color:#22D3EE; text-decoration:none; } a:hover { color:#67E8F9; }
    .mono { font-family:'IBM Plex Mono',ui-monospace,monospace; font-variant-numeric:tabular-nums; }
    @keyframes tally { 0%,100%{opacity:1;} 50%{opacity:.25;} }
  </style>
</helmet>
'''
out=HEAD+'''<div style="display:flex;width:1440px;height:900px;background:#0E1116;">
%s
<div style="flex-grow:1;padding:26px 32px;display:flex;flex-direction:column;gap:18px;overflow:hidden;">
  <div style="display:flex;align-items:flex-end;justify-content:space-between;">
    <div><div style="font:700 24px %s;letter-spacing:-.4px;color:#E6EDF3;">Semana del 6 al 12 de septiembre</div>
    <div style="font:400 13.5px %s;color:#8B949E;margin-top:3px;">De 6:00 AM a medianoche. El día de emisión empieza a las 6:00 AM.</div></div>
    <div style="display:flex;gap:7px;">%s</div></div>
  <div style="position:relative;height:26px;margin-left:43px;">%s</div>
  <div style="position:relative;display:flex;flex-direction:column;gap:3px;">%s
    <div style="position:absolute;left:%.0fpx;top:-30px;bottom:0;width:2px;background:#22D3EE;">
      <div style="position:absolute;top:-9px;left:-27px;background:#22D3EE;color:#06141a;font:600 10.5px 'IBM Plex Mono';padding:3px 7px;border-radius:4px;white-space:nowrap;">1:45 PM</div></div></div>
  <div style="display:flex;align-items:center;gap:20px;margin-top:2px;">
    <div style="display:flex;align-items:center;gap:8px;"><div style="width:22px;height:9px;border-radius:2px;background:#2b3346;"></div><span style="font:400 12.5px %s;color:#8B949E;">programado</span></div>
    <div style="display:flex;align-items:center;gap:8px;"><div style="width:22px;height:9px;border-radius:2px;background:%s;"></div><span style="font:400 12.5px %s;color:#8B949E;">vacío</span></div>
    <div style="display:flex;align-items:center;gap:8px;"><div style="width:22px;height:9px;border-radius:2px;background:#0f3a44;box-shadow:inset 0 0 0 1.5px #22D3EE;"></div><span style="font:400 12.5px %s;color:#8B949E;">en vivo</span></div>
    <div style="flex-grow:1;"></div>
    <span style="font:600 12.5px %s;color:#F85149;">Además, 1:00 – 6:00 AM está vacío los siete días</span></div>
  <div style="display:flex;align-items:center;gap:20px;height:100px;border-radius:9px;border:1.5px dashed rgba(248,81,73,.5);background:repeating-linear-gradient(45deg,rgba(248,81,73,.09),rgba(248,81,73,.09) 7px,transparent 7px,transparent 14px);padding:0 26px;margin-top:8px;">
    <div style="flex-grow:1;">
      <div style="font:700 18px %s;color:#F85149;letter-spacing:.3px;">El fin de semana tiene el doble de aire vacío</div>
      <div style="font:400 13.5px %s;color:#8B949E;margin-top:4px;">10 horas el sábado y el domingo, contra 5 de lunes a viernes. Lo que no se llena, sale en negro.</div></div>
    <div style="display:flex;align-items:center;gap:10px;background:#22D3EE;color:#06141a;border-radius:8px;padding:13px 24px;font:600 15px %s;">
      <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#06141a" stroke-width="2.3" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>Llenar el fin de semana</div>
    <div style="border:1px solid #21262D;color:#8B949E;border-radius:8px;padding:13px 20px;font:500 14.5px %s;">Escoger yo</div></div>
</div></div>
</x-dc>
</body>
</html>
'''%(NAV('Parrilla'),F,F,tabs,horas,filas,nowx+43,F,VAC,F,F,F,F,F,F,F)
io.open('Parrilla.dc.html','w',encoding='utf-8').write(out)
helmet=out[out.index('<helmet>')+8:out.index('</helmet>')]
body=out[out.index('</helmet>')+9:out.index('</x-dc>')]
io.open('_plain/Parrilla.html','w',encoding='utf-8').write('<!doctype html><html><head><meta charset="utf-8">'+helmet+'</head><body style="margin:0">'+body+'</body></html>')
print('ok')
