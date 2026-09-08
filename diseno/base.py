import io
F="'IBM Plex Sans'"
ICONS={'Al aire':'<path d="M12 20v-7"/><path d="M8.5 13 12 4l3.5 9"/><path d="M4.9 16.5a9 9 0 0 1 0-9"/><path d="M19.1 7.5a9 9 0 0 1 0 9"/>',
 'Parrilla':'<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18M8 3v4M16 3v4"/>',
 'Reglas':'<rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/>',
 'Biblioteca':'<rect x="3" y="3" width="7" height="9" rx="1.5"/><rect x="14" y="3" width="7" height="9" rx="1.5"/><rect x="3" y="15" width="7" height="6" rx="1.5"/><rect x="14" y="15" width="7" height="6" rx="1.5"/>',
 'Anuncios':'<path d="M20.6 13.4 13.4 20.6a2 2 0 0 1-2.8 0l-7.2-7.2A2 2 0 0 1 2.8 12V4.8A2 2 0 0 1 4.8 2.8H12a2 2 0 0 1 1.4.6l7.2 7.2a2 2 0 0 1 0 2.8Z"/><circle cx="7.5" cy="7.5" r="1.2"/>',
 'Ajustes':'<circle cx="12" cy="12" r="3.2"/><path d="M12 2.6v2.6M12 18.8v2.6M21.4 12h-2.6M5.2 12H2.6M18.6 5.4l-1.8 1.8M7.2 16.8l-1.8 1.8M18.6 18.6l-1.8-1.8M7.2 7.2 5.4 5.4"/>'}
def NAV(act, ads=True, tint=''):
    rows=[]
    for lab,ic in ICONS.items():
        if lab=='Anuncios' and not ads: continue
        on=(lab==act); bg='background:#161B22;' if on else ''
        col='#E6EDF3' if on else '#8B949E'; st='#22D3EE' if on else '#6E7681'
        bar='<div style="position:absolute;left:0;top:9px;bottom:9px;width:3px;border-radius:0 3px 3px 0;background:#22D3EE;"></div>' if on else ''
        rows.append('<div style="position:relative;display:flex;align-items:center;gap:12px;padding:11px 14px;border-radius:8px;%s">%s<svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="%s" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">%s</svg><span style="font:%s 14.5px %s,system-ui;color:%s;">%s</span></div>'%(bg,bar,st,ic,'600' if on else '400',F,col,lab))
    return '<div style="width:224px;flex-shrink:0;background:%s;border-right:1px solid #21262D;display:flex;flex-direction:column;padding:22px 0;"><div style="padding:0 20px 26px;display:flex;align-items:center;gap:10px;"><svg width="25" height="25" viewBox="0 0 24 24" fill="none" stroke="#22D3EE" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">%s</svg><span style="font:600 17px %s,system-ui;color:#E6EDF3;letter-spacing:-.3px;">Antena<span style="color:#22D3EE;">787</span></span></div><div style="display:flex;flex-direction:column;gap:3px;padding:0 12px;flex-grow:1;">%s</div><div style="padding:16px 20px 0;border-top:1px solid #21262D;margin:0 12px;"><div style="font:500 12px %s;color:#6E7681;letter-spacing:.4px;">CANAL</div><div style="font:500 14px %s;color:#8B949E;margin-top:5px;">Caribbean Advantage TV</div></div></div>'%(tint or '#0B0E12',ICONS['Al aire'],F,''.join(rows),F,F)
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
    @keyframes pulso { 0%,100%{opacity:1;} 50%{opacity:.55;} }
  </style>
</helmet>
'''
FOOT='</x-dc>\n</body>\n</html>\n'
def plain(name, src):
    helmet=src[src.index('<helmet>')+8:src.index('</helmet>')]
    body=src[src.index('</helmet>')+9:src.index('</x-dc>')]
    io.open('_plain/%s.html'%name,'w',encoding='utf-8').write('<!doctype html><html><head><meta charset="utf-8">'+helmet+'</head><body style="margin:0">'+body+'</body></html>')
