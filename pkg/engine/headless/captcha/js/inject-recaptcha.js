(token) => {
  document.querySelectorAll('[id="g-recaptcha-response"], [name="g-recaptcha-response"]').forEach(el => {
    el.value = token;
    el.style.display = 'block';
  });

  let called = false;

  if (!called) {
    const el = document.querySelector('.g-recaptcha[data-callback]');
    if (el) {
      const name = el.getAttribute('data-callback');
      if (name && typeof window[name] === 'function') {
        window[name](token);
        called = true;
      }
    }
  }

  // ___grecaptcha_cfg.clients holds internal reCAPTCHA state including registered callbacks
  if (!called && typeof ___grecaptcha_cfg !== 'undefined' && ___grecaptcha_cfg.clients) {
    try {
      for (const key in ___grecaptcha_cfg.clients) {
        const client = ___grecaptcha_cfg.clients[key];
        const find = (obj, depth) => {
          if (depth > 4 || !obj || typeof obj !== 'object') return null;
          if (obj instanceof Node || obj instanceof Window) return null;
          try {
            for (const k in obj) {
              try {
                const v = obj[k];
                if (k === 'callback' && typeof v === 'function') return v;
                if (v && typeof v === 'object' && !(v instanceof Node) && !(v instanceof Window)) {
                  if (typeof v.callback === 'function') return v.callback;
                  const found = find(v, depth + 1);
                  if (found) return found;
                }
              } catch(e) { continue; }
            }
          } catch(e) { return null; }
          return null;
        };
        const cb = find(client, 0);
        if (cb) { cb(token); called = true; break; }
      }
    } catch(e) {}
  }

  // callback alone may not trigger navigation
  const form = document.querySelector('form:has(#g-recaptcha-response)') ||
               document.querySelector('form:has(.g-recaptcha)');
  if (form) form.submit();
}
