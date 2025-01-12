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
      window.history.pushState = function (a, b, c) {
        window.__navigatedLinks.push({ url: c, source: "history.pushState" });
      };
      window.history.replaceState = function (a, b, c) {
        window.__navigatedLinks.push({ url: c, source: "history.replaceState" });
      };
      Object.defineProperty(
        window.history,
        "pushState",
        markElementReadonlyProperties
      );
      Object.defineProperty(
        window.history,
        "replaceState",
        markElementReadonlyProperties
      );
      // Hook window.open to capture all the opened pages
      window.open = function (url) {
        console.log("[hook] open url request", url);
        window.__navigatedLinks.push({ url: url, source: "window.open" });
      };
      Object.defineProperty(window, "open", markElementReadonlyProperties);
  
      // Add event listener for hashchange
      window.addEventListener("hashchange", function () {
        window.__navigatedLinks.push({
          url: document.location.href,
          source: "hashchange",
        });
      });
  
      var oldWebSocket = window.WebSocket;
      window.WebSocket = function (url, arg) {
        window.__navigatedLinks.push({ url: url, source: "websocket" });
        return new oldWebSocket(url, arg);
      };
  
      var oldEventSource = window.EventSource;
      window.EventSource = function (url, arg) {
        window.__navigatedLinks.push({ url: url, source: "eventsource" });
        return new oldEventSource(url, arg);
      };
  
      var originalFetch = window.fetch;
      window.fetch = function (...args) {
        const url = args[0] instanceof Request ? args[0].url : args[0];
        window.__navigatedLinks.push({ url: url, source: "fetch" });
        return originalFetch.apply(this, args);
      };
    }
  
    // hookMiscellaneousUtilities performs miscellaneous hooks
    // on the page to prevent certain actions from happening
    // and to speed up certain actions.
    function hookMiscellaneousUtilities() {
      // Hook form reset to prevent the form from being reset
      HTMLFormElement.prototype.reset = function () {
        console.log("[hook] cancel reset form");
      };
      Object.defineProperty(
        HTMLFormElement.prototype,
        "reset",
        markElementReadonlyProperties
      );
  
      // Hook window.close to prevent the page from being closed
      window.close = function () {
        console.log("[hook] trying to close page.");
      };
      Object.defineProperty(window, "close", markElementReadonlyProperties);
  
      // Hook setTimeout and setInterval to speed up delayed actions
      // on the page. This is useful where there is some request happening
      // on the page after a delay or some animation happening after a delay.
      const originalSetTimeout = window.setTimeout;
      const originalSetInterval = window.setInterval;
  
      const speedUpFactor = 0.1; // For example, 10 times faster
  
      window.setTimeout = function (callback, delay, ...args) {
        return originalSetTimeout(callback, delay * speedUpFactor, ...args);
      };
      window.setInterval = function (callback, delay, ...args) {
        return originalSetInterval(callback, delay * speedUpFactor, ...args);
      };
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
    // hookAddEventListener();
    // hookNavigatedLinkSinks();
    // hookMiscellaneousUtilities();
  })();
  