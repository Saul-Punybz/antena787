import io
exec(open('base.py').read())
# tira de miniaturas: 14 dias, con estados
TONOS=['#1e2a38','#22303f','#1a2632','#26343f','#1f2b36']
segs=''
for d in range(14):
    if d<2:   # ya borrado
        segs+='<div style="flex:1;margin-right:2px;background:repeating-linear-gradient(45deg,rgba(110,118,129,.18),rgba(110,118,129,.18) 3px,transparent 3px,transparent 7px);border-radius:3px;"></div>'
    elif d<4: # por borrarse
        segs+='<div style="flex:1;margin-right:2px;background:%s;border-radius:3px;position:relative;opacity:.5;"><div style="position:absolute;inset:0;border:1px solid rgba(210,153,34,.5);border-radius:3px;"></div></div>'%TONOS[d%5]
    else:
        segs+='<div style="flex:1;margin-right:2px;background:%s;border-radius:3px;"></div>'%TONOS[d%5]
ejes=''.join('<span style="flex:1;font-family:ui-monospace,monospace;font-size:10px;color:#4d545c;text-align:center;">%s</span>'%t for t in ['','hace 12 d','','hace 10 d','','hace 8 d','','hace 6 d','','hace 4 d','','hace 2 d','','hoy'])

out=HEAD+'''<div style="display:flex;width:1440px;height:900px;background:#0E1116;">
%s
<div style="flex-grow:1;padding:24px 30px;display:flex;flex-direction:column;gap:18px;overflow:hidden;">

  <div style="display:flex;align-items:flex-end;justify-content:space-between;">
    <div><div style="font:700 24px %s;letter-spacing:-.4px;color:#E6EDF3;">Grabación y diferido</div>
    <div style="font:400 13.5px %s;color:#8B949E;margin-top:3px;">Todo lo que sale al aire queda grabado. De madrugada se retransmite el día.</div></div>
    <div style="display:flex;align-items:center;gap:9px;background:#161B22;border:1px solid #21262D;border-radius:20px;padding:9px 17px;">
      <span style="display:inline-block;width:8px;height:8px;border-radius:50%%;background:#FF3B30;animation:tally 1.6s ease-in-out infinite;"></span>
      <span style="font:500 13.5px %s;color:#8B949E;">Grabando · te quedan <span style="color:#E6EDF3;">12 días</span> de historial</span></div></div>

  <div style="display:flex;gap:16px;">
    <div style="flex:1;">
      <div style="display:flex;align-items:center;gap:9px;margin-bottom:9px;">
        <span style="display:inline-block;width:9px;height:9px;border-radius:50%%;background:#FF3B30;animation:tally 1.6s ease-in-out infinite;"></span>
        <span style="font:700 12px %s;letter-spacing:1.3px;color:#E6EDF3;">EN VIVO — ESTO ES LO QUE SALE AHORA</span></div>
      <div style="position:relative;height:298px;border-radius:11px;overflow:hidden;border:2px solid #FF3B30;background:linear-gradient(155deg,#3a2226,#1a1f28 58%%,#10141a);">
        <div style="position:absolute;inset:0;background:radial-gradient(ellipse at 42%% 46%%,rgba(255,59,48,.09),transparent 62%%);"></div>
        <div style="position:absolute;left:0;right:0;bottom:0;height:110px;background:linear-gradient(transparent,rgba(6,9,13,.92));"></div>
        <div style="position:absolute;left:18px;right:18px;bottom:15px;">
          <div style="font:700 22px %s;color:#E6EDF3;">Los Simuladores</div>
          <div class="mono" style="font-size:13px;color:#8B949E;margin-top:4px;">1:45 PM · en vivo</div></div></div>
      <div style="font:400 12px %s;color:#6E7681;margin-top:9px;">Este panel nunca se detiene, pase lo que pase en el otro.</div>
    </div>

    <div style="flex:1;">
      <div style="display:flex;align-items:center;gap:9px;margin-bottom:9px;">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#D29922" stroke-width="2.4" stroke-linecap="round"><path d="M3 12a9 9 0 1 0 3-6.7"/><path d="M3 4v5h5"/></svg>
        <span style="font:700 12px %s;letter-spacing:1.3px;color:#D29922;">REVISANDO — HACE 3 HORAS</span></div>
      <div style="position:relative;height:298px;border-radius:11px;overflow:hidden;border:2px dashed rgba(210,153,34,.6);background:linear-gradient(155deg,#2b3346,#181d26 58%%,#10141a);">
        <div style="position:absolute;left:0;right:0;bottom:0;height:110px;background:linear-gradient(transparent,rgba(6,9,13,.92));"></div>
        <div style="position:absolute;top:14px;left:16px;background:rgba(210,153,34,.9);color:#1a1508;border-radius:5px;padding:4px 10px;font:600 11px 'IBM Plex Mono';">10:45 AM</div>
        <div style="position:absolute;left:18px;right:18px;bottom:15px;">
          <div style="font:700 22px %s;color:#E6EDF3;">RadioOnce Live!</div>
          <div class="mono" style="font-size:13px;color:#8B949E;margin-top:4px;">grabado · 3 h 00 m atrás</div></div></div>
      <div style="display:flex;align-items:center;gap:11px;margin-top:9px;">
        <span style="background:#22D3EE;color:#06141a;border-radius:7px;padding:7px 16px;font:600 12.5px %s;">Volver a ahora</span>
        <span style="font:400 12px %s;color:#6E7681;">Arrastra sobre la tira de abajo para moverte en el tiempo.</span></div>
    </div>
  </div>

  <div style="background:#161B22;border:1px solid #21262D;border-radius:11px;padding:18px 22px;">
    <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:12px;">
      <span style="font:600 11px %s;letter-spacing:1.2px;color:#6E7681;">TODO LO GRABADO</span>
      <div style="display:flex;gap:17px;">
        <span style="display:flex;align-items:center;gap:6px;"><span style="width:16px;height:9px;border-radius:2px;background:#22303f;display:inline-block;"></span><span style="font:400 11.5px %s;color:#8B949E;">disponible</span></span>
        <span style="display:flex;align-items:center;gap:6px;"><span style="width:16px;height:9px;border-radius:2px;background:#22303f;opacity:.5;border:1px solid rgba(210,153,34,.5);display:inline-block;"></span><span style="font:400 11.5px %s;color:#8B949E;">se borra pronto</span></span>
        <span style="display:flex;align-items:center;gap:6px;"><span style="width:16px;height:9px;border-radius:2px;background:repeating-linear-gradient(45deg,rgba(110,118,129,.18),rgba(110,118,129,.18) 3px,transparent 3px,transparent 7px);display:inline-block;"></span><span style="font:400 11.5px %s;color:#8B949E;">ya se borró</span></span></div></div>
    <div style="display:flex;height:52px;">%s</div>
    <div style="display:flex;margin-top:7px;">%s</div>
  </div>

  <div style="display:flex;align-items:center;gap:26px;background:#161B22;border:1px solid #21262D;border-radius:11px;padding:20px 24px;">
    <div style="flex-grow:1;">
      <div style="font:600 11px %s;letter-spacing:1.2px;color:#6E7681;margin-bottom:14px;">EL DIFERIDO DE MADRUGADA</div>
      <div style="display:flex;align-items:center;gap:0;">
        <div style="text-align:center;"><div class="mono" style="font-size:12px;color:#22D3EE;">AHORA</div><div style="width:3px;height:26px;background:#22D3EE;margin:6px auto 0;border-radius:2px;"></div></div>
        <div style="flex-grow:1;height:3px;background:linear-gradient(90deg,#22D3EE,#D29922);margin:0 2px;position:relative;top:12px;"></div>
        <div style="text-align:center;"><div class="mono" style="font-size:12px;color:#D29922;">EN AIRE</div><div style="width:3px;height:26px;background:#D29922;margin:6px auto 0;border-radius:2px;"></div></div>
      </div>
      <div style="font:500 13.5px %s;color:#E6EDF3;margin-top:12px;">De 1:00 a 6:00 AM se repite lo que salió de 7:00 AM a 12:00 PM.</div>
      <div style="font:400 12.5px %s;color:#8B949E;margin-top:4px;">Cinco horas que hoy salen en negro, llenas con contenido que ya tienes.</div></div>
    <div style="text-align:right;flex-shrink:0;">
      <div class="mono" style="font-size:34px;color:#D29922;font-weight:600;">18 h</div>
      <div style="font:400 12px %s;color:#6E7681;margin-top:2px;">de retraso</div></div>
    <div style="border:1px solid #21262D;color:#E6EDF3;border-radius:8px;padding:12px 20px;font:500 14px %s;flex-shrink:0;">Cambiar</div>
  </div>
</div></div>
'''%(NAV('Parrilla'),F,F,F,F,F,F,F,F,F,F,F,F,F,F,segs,ejes,F,F,F,F,F)+FOOT
io.open('Diferido.dc.html','w',encoding='utf-8').write(out); plain('Diferido',out)
print('Diferido ok')
