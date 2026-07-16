(token) => {
  document.querySelectorAll('textarea[name="h-captcha-response"]').forEach(el => { el.value = token; });
  document.querySelectorAll('textarea[name="g-recaptcha-response"]').forEach(el => { el.value = token; });

  let called = false;

  if (!called) {
    const el = document.querySelector('.h-captcha[data-callback]');
    if (el) {
      const name = el.getAttribute('data-callback');
      if (name && typeof window[name] === 'function') {
        window[name](token);
        called = true;
      }
    }
  }

  // callback alone may not trigger navigation
  const form = document.querySelector('form:has(.h-captcha)') ||
               document.querySelector('form:has([name="h-captcha-response"])');
  if (form) form.submit();
}
