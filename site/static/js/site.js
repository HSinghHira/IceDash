/* IceDash site: cursor background, copy buttons and tabs.
   Taken from the IceStore site's core.js, without its router and cookies. */
(()=>{
  'use strict';
  const ICE=window.ICE=window.ICE||{};
  const $=(s,r=document)=>r.querySelector(s), $$=(s,r=document)=>[...r.querySelectorAll(s)];
  const reduced=matchMedia('(prefers-reduced-motion:reduce)').matches;

  /* ---- background that follows the cursor ---- */
  const bg=$('.bg');
  if(bg&&!reduced){
    let x=innerWidth*.7,y=innerHeight*.25,tx=x,ty=y,run=false;
    const tick=()=>{
      x+=(tx-x)*.09;y+=(ty-y)*.09;
      bg.style.setProperty('--mx',x.toFixed(1)+'px');bg.style.setProperty('--my',y.toFixed(1)+'px');
      if(Math.abs(tx-x)+Math.abs(ty-y)>.5)requestAnimationFrame(tick);else run=false;
    };
    addEventListener('pointermove',e=>{tx=e.clientX;ty=e.clientY;if(!run){run=true;requestAnimationFrame(tick)}},{passive:true});
  }

  /* ---- copy buttons: data-copy="text" or data-copy-from="#selector" ---- */
  document.addEventListener('click',async e=>{
    const b=e.target.closest('[data-copy],[data-copy-from]');if(!b)return;
    const text=b.dataset.copy!==undefined?b.dataset.copy:($(b.dataset.copyFrom)||{textContent:''}).textContent.trim();
    try{await navigator.clipboard.writeText(text)}
    catch(_){const t=document.createElement('textarea');t.value=text;t.style.cssText='position:fixed;opacity:0';document.body.append(t);t.select();try{document.execCommand('copy')}catch(_){}t.remove()}
    if(b._t)return;
    const old=b.innerHTML;b.innerHTML=ICE.icon('check');b.classList.add('ok');
    b._t=setTimeout(()=>{b.innerHTML=old;b.classList.remove('ok');b._t=0},1600);
  });

  /* ---- tabs (install steps, screenshots); #zimaos or #casaos picks one ---- */
  const tabs=()=>$$('[role=tablist]').forEach(list=>{
    const ts=$$('[role=tab]',list),ps=ts.map(t=>document.getElementById(t.getAttribute('aria-controls')));
    const show=i=>ts.forEach((t,j)=>{t.setAttribute('aria-selected',j===i);t.tabIndex=j===i?0:-1;ps[j].hidden=j!==i});
    ts.forEach((t,i)=>{
      t.onclick=()=>show(i);
      t.onkeydown=e=>{const d={ArrowRight:1,ArrowLeft:-1}[e.key];if(!d)return;e.preventDefault();const j=(i+d+ts.length)%ts.length;show(j);ts[j].focus()};
    });
    const h=ts.findIndex(t=>t.dataset.hash&&'#'+t.dataset.hash===location.hash);
    show(h>-1?h:0);
    if(h>-1)setTimeout(()=>ts[h].scrollIntoView({block:'center'}),60);
  });

  const init=()=>{tabs()};
  if(document.readyState==='loading')addEventListener('DOMContentLoaded',init);else init();
})();
