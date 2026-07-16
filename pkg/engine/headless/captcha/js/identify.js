() => {
  const hcap = document.querySelector('.h-captcha[data-sitekey]');
  if (hcap) {
    return { provider: "hcaptcha", sitekey: hcap.getAttribute("data-sitekey"), action: "" };
  }

  const cf = document.querySelector('.cf-turnstile[data-sitekey]');
  if (cf) {
    return { provider: "turnstile", sitekey: cf.getAttribute("data-sitekey"), action: "" };
  }

  const recapScripts = document.querySelectorAll(
    'script[src*="recaptcha/api.js"], script[src*="recaptcha/enterprise.js"]'
  );
  for (const s of recapScripts) {
    try {
      const u = new URL(s.src);
      const isEnterprise = u.pathname.includes("/enterprise.js");
      const renderParam = u.searchParams.get("render");
      if (renderParam && renderParam !== "explicit") {
        return {
          provider: isEnterprise ? "recaptchav3enterprise" : "recaptchav3",
          sitekey: renderParam,
          action: ""
        };
      }
    } catch {}
  }

  const isEnterprise = recapScripts.length > 0 &&
    Array.from(recapScripts).some(s => s.src.includes("/enterprise.js"));

  const recap = document.querySelector('.g-recaptcha[data-sitekey]');
  if (recap) {
    return {
      provider: isEnterprise ? "recaptchav2enterprise" : "recaptchav2",
      sitekey: recap.getAttribute("data-sitekey"),
      action: ""
    };
  }

  const generic = document.querySelector('[data-sitekey]');
  if (generic) {
    return {
      provider: isEnterprise ? "recaptchav2enterprise" : "recaptchav2",
      sitekey: generic.getAttribute("data-sitekey"),
      action: ""
    };
  }

  return null;
}
