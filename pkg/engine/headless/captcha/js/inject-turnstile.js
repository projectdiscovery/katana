(token) => {
  document.querySelectorAll('input[name="cf-turnstile-response"]').forEach(el => { el.value = token; });

  let called = false;

  if (!called) {
    const el = document.querySelector('.cf-turnstile[data-callback]');
    if (el) {
      const name = el.getAttribute('data-callback');
      if (name && typeof window[name] === 'function') {
        window[name](token);
        called = true;
      }
    }
  }

  // callback alone may not trigger navigation
  const form = document.querySelector('form:has(.cf-turnstile)') ||
               document.querySelector('form:has([name="cf-turnstile-response"])');
  if (form) form.submit();
}
