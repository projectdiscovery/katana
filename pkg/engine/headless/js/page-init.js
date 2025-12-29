// This script initializes the page and hooks up event listeners
// and other interesting stuff needed to make the crawling work.
//
// Actions performed:
//
// 1. Hook addTargetListener to capture all the event listeners added on the page.
//    These are accessible via window.__eventListeners
// 2. Hook window.open to capture all the opened pages.
//    These are accessible via window.__navigatedLinks
// 3. Hook setTimeout and setInterval to speed up delayed actions
// 4. Hook form reset to prevent the form from being reset
// 5. Hook window.close to prevent the page from being closed
// 6. Hook history pushState and replaceState for new links
// 7. Add event listener for hashchange to identify new navigations
// 8. TODO: Hook inline event listeners so that layer0 event listeners can be tracked as well
(function pageInitAndHook() {
    const markElementReadonlyProperties = {
      writable: false,
      configurable: false,
    };
  
    // hookNavigatedLinkSinks hooks the navigated link sinks
    // on the page to capture all the navigated links.
    function hookNavigatedLinkSinks() {
      window.__navigatedLinks = [];
  
      // Hook history.pushState and history.replaceState to capture all the navigated links
      const __origPushState = window.history.pushState.bind(window.history);
      const __origReplaceState = window.history.replaceState.bind(window.history);
      function __wrappedPushState(a, b, c) {
        try { window.__navigatedLinks.push({ url: c, source: "history.pushState" }); } catch (_) {}
        return __origPushState(a, b, c);
      }
      function __wrappedReplaceState(a, b, c) {
        try { window.__navigatedLinks.push({ url: c, source: "history.replaceState" }); } catch (_) {}
        return __origReplaceState(a, b, c);
      }
      Object.defineProperty(window.history, "pushState", { value: __wrappedPushState, writable: false, configurable: false });
      Object.defineProperty(window.history, "replaceState", { value: __wrappedReplaceState, writable: false, configurable: false });
      // Hook window.open to capture all the opened pages
      const __origOpen = window.open.bind(window);
      function __wrappedOpen(url, ...rest) {
        try { window.__navigatedLinks.push({ url, source: "window.open" }); } catch (_) {}
        return __origOpen(url, ...rest);
      }
      Object.defineProperty(window, "open", { value: __wrappedOpen, writable: false, configurable: false });
  
      // Add event listener for hashchange
      window.addEventListener("hashchange", function () {
        window.__navigatedLinks.push({
          url: document.location.href,
          source: "hashchange",
        });
      });
  
      const __OrigWebSocket = window.WebSocket;
      function __WrappedWebSocket(url, protocols) {
        try { window.__navigatedLinks.push({ url, source: "websocket" }); } catch (_) {}
        return Reflect.construct(__OrigWebSocket, [url, protocols], new.target || __WrappedWebSocket);
      }
      __WrappedWebSocket.prototype = __OrigWebSocket.prototype;
      Object.setPrototypeOf(__WrappedWebSocket, __OrigWebSocket);
      Object.defineProperty(window, "WebSocket", { value: __WrappedWebSocket, writable: false, configurable: false });

      const __OrigEventSource = window.EventSource;
      function __WrappedEventSource(url, eventSourceInitDict) {
        try { window.__navigatedLinks.push({ url, source: "eventsource" }); } catch (_) {}
        return Reflect.construct(__OrigEventSource, [url, eventSourceInitDict], new.target || __WrappedEventSource);
      }
      __WrappedEventSource.prototype = __OrigEventSource.prototype;
      Object.setPrototypeOf(__WrappedEventSource, __OrigEventSource);
      Object.defineProperty(window, "EventSource", { value: __WrappedEventSource, writable: false, configurable: false });
  
      const __origFetch = window.fetch.bind(window);
      function __wrappedFetch(...args) {
        const url = args[0] instanceof Request ? args[0].url : args[0];
        try { window.__navigatedLinks.push({ url: url, source: "fetch" }); } catch (_) {}
        return __origFetch(...args);
      }
      Object.defineProperty(window, "fetch", { value: __wrappedFetch, writable: false, configurable: false });
    }
  
    // hookMiscellaneousUtilities performs miscellaneous hooks
    // on the page to prevent certain actions from happening
    // and to speed up certain actions.
    function hookMiscellaneousUtilities() {
      // Hook form reset to conditionally prevent the form from being reset
      const __origFormReset = HTMLFormElement.prototype.reset;
      const __wrappedFormReset = function (...args) {
        if (window.__katanaHooksOptions?.preventFormReset === true) {
          try { console.log("[hook] cancel reset form"); } catch (_) {}
          return;
        }
        return __origFormReset.apply(this, args);
      };
      Object.defineProperty(
        HTMLFormElement.prototype,
        "reset",
        { value: __wrappedFormReset, writable: false, configurable: false }
      );
  
      // Hook window.close to prevent the page from being closed
      const __wrappedClose = function () {
        console.log("[hook] trying to close page.");
      };
      Object.defineProperty(window, "close", { value: __wrappedClose, writable: false, configurable: false });
  
      // Hook setTimeout and setInterval to speed up delayed actions
      // on the page. This is useful where there is some request happening
      // on the page after a delay or some animation happening after a delay.
      const __origSetTimeout = window.setTimeout;
      const __origSetInterval = window.setInterval;

      const speedUpFactor = 0.1;

      function __wrappedSetTimeout(callback, delay, ...args) {
        return __origSetTimeout(callback, delay * speedUpFactor, ...args);
      }
      function __wrappedSetInterval(callback, delay, ...args) {
        return __origSetInterval(callback, delay * speedUpFactor, ...args);
      }
      Object.defineProperty(window, "setTimeout", { value: __wrappedSetTimeout, writable: false, configurable: false });
      Object.defineProperty(window, "setInterval", { value: __wrappedSetInterval, writable: false, configurable: false });
    }
  
    // hookAddEventListener hooks the addTargetListener to capture
    // all the event listeners added on the page
    function hookAddEventListener() {
      const originalAddEventListener = Element.prototype.addEventListener;
  
      window.__eventListeners = [];
      Element.prototype.addEventListener = function (type, listener, options) {
        // Ensure `this` is a valid element and has the necessary properties
        if (!this || !this.tagName) {
          return originalAddEventListener.call(this, type, listener, options);
        }
  
        if (this.tagName == "BODY") {
          return originalAddEventListener.call(this, type, listener, options);
        }
        let item = {
          element: {
            tagName: this.tagName,
            id: this.id,
            classes: this.className,
            outerHTML: this.outerHTML.slice(0, 100), // Capture a snippet of the element's outerHTML
            xpath: window.getXPath(this),
            cssSelector: window.getCssPath(this),
            attributes: window.getElementAttributes(this),
            textContent: this.textContent.trim(),
            hidden: this.hidden,
            name: this.name,
            type: this.type,
            value: this.value,
          },
          type: type,
          listener: listener.toString(),
          options: options || {},
        };
        console.log("[hook] got event listener", item);
        window.__eventListeners.push(item);
        return originalAddEventListener.call(this, type, listener, options);
      };
    }
  
    // Main hook initialization part
    const __opts = window.__katanaHooksOptions || { hooked: false };
    try { if (__opts.hooked === true) hookAddEventListener(); } catch (_) {}
    try { if (__opts.hooked === true) hookNavigatedLinkSinks(); } catch (_) {}
    try { if (__opts.hooked === true) hookMiscellaneousUtilities(); } catch (_) {}
  })();
  