// This file contains utility JS functions that are utilised by
// the main crawling JS code to perform actions.
(function initUtilityFunctions() {
    // getElementAttributes returns the attributes of an element
    window.getElementAttributes = function (element) {
      const attrs = {};
      for (let attr of element.attributes) {
        attrs[attr.name] = attr.value;
      }
      return attrs;
    };
  
    // _elementDataFromElement returns the data for an element
    window._elementDataFromElement = function (el) {
      return {
        tagName: el.tagName,
        id: el.id,
        classes: typeof el.className === 'string' ? el.className : Array.from(el.classList).join(' '),
        attributes: window.getElementAttributes(el),
        hidden: el.hidden,
        outerHTML: el.outerHTML,
        name: el.name,
        type: el.type,
        value: el.value != null ? String(el.value) : '',
        textContent: el.textContent.trim(),
        xpath: window.getXPath(el),
        cssSelector: window.getCssPath(el),
      };
    };
  
    // getAllElements returns all the elements for a query
    // selector on the page
    window.getAllElements = function (selector) {
      try {
        const nodes = document.querySelectorAll(selector);
        return Array.from(nodes).map((el) => _elementDataFromElement(el));
      } catch (_) {
        return [];
      }
    };

    window.getElementFromXPath = function (xpath) {
      try {
        const element = document
          .evaluate(xpath, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null)
          .singleNodeValue;
        return element ? _elementDataFromElement(element) : null;
      } catch (_) {
        return null;
      }
    }
  
    // getAllElementsWithEventListeners returns all the elements
    // on the page along with their event listeners
    // TODO: Is it optimized? or do we need to do something else?
    window.getAllElementsWithEventListeners = function () {
      const elements = document.querySelectorAll("*");
      const elementsWithListeners = [];
      for (let el of elements) {
        const listeners = getEventListeners(el);
        if (listeners && listeners.length) {
          elementsWithListeners.push({
            element: _elementDataFromElement(el),
            listeners: listeners,
          });
        }
      }
      return elementsWithListeners;
    };
  
    // getEventListeners returns all the event listeners
    // attached to an element
    function getEventListeners(element) {
      const listeners = [];
      for (let event in element) {
        if (event.startsWith("on")) {
          const listener = element[event];
          if (typeof listener === "function") {
            listeners.push({
              type: event,
              listener: listener.toString(),
            });
          }
        }
      }
      return listeners;
    }
  
    // getAllForms returns all the forms on the page
    // along with their elements
    window.getAllForms = function () {
      const forms = document.querySelectorAll("form");
      const pseudoForms = document.querySelectorAll("div.form");
      
      const allForms = [...forms, ...pseudoForms];
      return Array.from(allForms).map((form) => ({
        tagName: form.tagName,
        id: form.id,
        classes: typeof form.className === 'string' ? form.className : Array.from(form.classList).join(' '),
        attributes: window.getElementAttributes(form),
        outerHTML: form.outerHTML,
        action: form.action ? String(form.action) : '',
        method: form.method,
        xpath: window.getXPath(form),
        cssSelector: window.getCssPath(form),
        elements: form.elements ? 
          Array.from(form.elements).map((el) => _elementDataFromElement(el)) :
          Array.from(form.querySelectorAll('input, select, textarea, button')).map((el) => _elementDataFromElement(el))
      }));
    };
  
    // Copyright (C) Chrome Authors
    // The below code is part of the Chrome DevTools project
    // and is adapted from there.
  
    // Utility to get the CSS selector path for an element.
    window.getCssPath = function (node, optimized = false) {
      if (node.nodeType !== Node.ELEMENT_NODE) return "";
  
      const steps = [];
      let contextNode = node;
      while (contextNode) {
        const step = window._cssPathStep(
          contextNode,
          optimized,
          contextNode === node
        );
        if (!step) break; // Error - bail out early.
        steps.push(step.value);
        if (step.optimized) break;
        contextNode = contextNode.parentNode;
      }
  
      steps.reverse();
      return steps.join(" > ");
    };
  
    // Utility to get the XPath for an element.
    window.getXPath = function (node, optimized = false) {
      if (node.nodeType === Node.DOCUMENT_NODE) return "/";
  
      const steps = [];
      let contextNode = node;
      while (contextNode) {
        const step = window._xPathValue(contextNode, optimized);
        if (!step) break; // Error - bail out early.
        steps.push(step.value);
        if (step.optimized) break;
        contextNode = contextNode.parentNode;
      }
  
      steps.reverse();
      return (steps.length && steps[0].optimized ? "" : "/") + steps.join("/");
    };
  
    // Helper to create a step in the CSS path.
    window._cssPathStep = function (node, optimized, isTargetNode) {
      if (node.nodeType !== Node.ELEMENT_NODE) return null;
  
      const id = node.getAttribute("id");
      if (optimized) {
        if (id) return { value: `#${id}`, optimized: true };
        const nodeNameLower = node.nodeName.toLowerCase();
        if (
          nodeNameLower === "body" ||
          nodeNameLower === "head" ||
          nodeNameLower === "html"
        )
          return { value: node.nodeName, optimized: true };
      }
      const nodeName = node.nodeName;
  
      if (id) return { value: `${nodeName}#${id}`, optimized: true };
      const parent = node.parentNode;
      if (!parent || parent.nodeType === Node.DOCUMENT_NODE)
        return { value: nodeName, optimized: true };
  
      const prefixedOwnClassNamesArray = window.prefixedElementClassNames(node);
      let needsClassNames = false;
      let needsNthChild = false;
      let ownIndex = -1;
      let elementIndex = -1;
      const siblings = parent.children;
      for (
        let i = 0;
        (ownIndex === -1 || !needsNthChild) && i < siblings.length;
        ++i
      ) {
        const sibling = siblings[i];
        if (sibling.nodeType !== Node.ELEMENT_NODE) continue;
        elementIndex += 1;
        if (sibling === node) {
          ownIndex = elementIndex;
          continue;
        }
        if (needsNthChild) continue;
        if (sibling.nodeName.toLowerCase() !== nodeName.toLowerCase()) continue;
  
        needsClassNames = true;
        const ownClassNames = new Set(prefixedOwnClassNamesArray);
        if (!ownClassNames.size) {
          needsNthChild = true;
          continue;
        }
        const siblingClassNamesArray = window.prefixedElementClassNames(sibling);
        for (let j = 0; j < siblingClassNamesArray.length; ++j) {
          const siblingClass = siblingClassNamesArray[j];
          if (!ownClassNames.has(siblingClass)) continue;
          ownClassNames.delete(siblingClass);
          if (!ownClassNames.size) {
            needsNthChild = true;
            break;
          }
        }
      }
  
      let result = nodeName;
      if (
        isTargetNode &&
        nodeName.toLowerCase() === "input" &&
        node.getAttribute("type") &&
        !node.getAttribute("id") &&
        !node.getAttribute("class")
      )
        result += '[type="' + node.getAttribute("type") + '"]';
      if (needsNthChild) {
        result += `:nth-child(${ownIndex + 1})`;
      } else if (needsClassNames) {
        for (const prefixedName of prefixedOwnClassNamesArray)
          result += "." + window.escapeIdentifierIfNeeded(prefixedName.substr(1));
      }
  
      return { value: result, optimized: false };
    };
  
    // Helper to get class names prefixed with '$' for mapping purposes.
    window.prefixedElementClassNames = function (node) {
      const classAttribute = node.getAttribute("class");
      if (!classAttribute) return [];
  
      return classAttribute
        .split(/\s+/g)
        .filter(Boolean)
        .map(function (name) {
          return "$" + name;
        });
    };
  
    // Helper to escape identifiers for use in CSS selectors.
    window.escapeIdentifierIfNeeded = function (ident) {
      if (window.isCSSIdentifier(ident)) return ident;
      const shouldEscapeFirst = /^(?:[0-9]|-[0-9-]?)/.test(ident);
      const lastIndex = ident.length - 1;
      return ident.replace(/./g, function (c, i) {
        return (shouldEscapeFirst && i === 0) || !window.isCSSIdentChar(c)
          ? window.escapeAsciiChar(c, i === lastIndex)
          : c;
      });
    };
  
    // Helper to determine if a character is valid in a CSS identifier.
    window.isCSSIdentChar = function (c) {
      if (/[a-zA-Z0-9_-]/.test(c)) return true;
      return c.charCodeAt(0) >= 0xa0;
    };
  
    // Helper to determine if a string is a valid CSS identifier.
    window.isCSSIdentifier = function (value) {
      return /^-{0,2}[a-zA-Z_][a-zA-Z0-9_-]*$/.test(value);
    };
  
    // Helper to escape ASCII characters for use in CSS selectors.
    window.escapeAsciiChar = function (c, isLast) {
      return (
        "\\" + c.charCodeAt(0).toString(16).padStart(2, "0") + (isLast ? "" : " ")
      );
    };
  
    // Helper to get the XPath step for a node.
    window._xPathValue = function (node, optimized) {
      let ownValue;
      const ownIndex = window._xPathIndex(node);
      if (ownIndex === -1) return null;
  
      switch (node.nodeType) {
        case Node.ELEMENT_NODE:
          if (optimized && node.getAttribute("id"))
            return {
              value: `//*[@id="${node.getAttribute("id")}"]`,
              optimized: true,
            };
          ownValue = node.localName;
          break;
        case Node.ATTRIBUTE_NODE:
          ownValue = "@" + node.nodeName;
          break;
        case Node.TEXT_NODE:
        case Node.CDATA_SECTION_NODE:
          ownValue = "text()";
          break;
        case Node.PROCESSING_INSTRUCTION_NODE:
          ownValue = "processing-instruction()";
          break;
        case Node.COMMENT_NODE:
          ownValue = "comment()";
          break;
        case Node.DOCUMENT_NODE:
          ownValue = "";
          break;
        default:
          ownValue = "";
          break;
      }
  
      if (ownIndex > 0) ownValue += `[${ownIndex}]`;
  
      return { value: ownValue, optimized: node.nodeType === Node.DOCUMENT_NODE };
    };
  
    // Helper to get the XPath index for a node.
    window._xPathIndex = function (node) {
      function areNodesSimilar(left, right) {
        if (left === right) return true;
  
        if (
          left.nodeType === Node.ELEMENT_NODE &&
          right.nodeType === Node.ELEMENT_NODE
        )
          return left.localName === right.localName;
  
        if (left.nodeType === right.nodeType) return true;
  
        const leftType =
          left.nodeType === Node.CDATA_SECTION_NODE
            ? Node.TEXT_NODE
            : left.nodeType;
        const rightType =
          right.nodeType === Node.CDATA_SECTION_NODE
            ? Node.TEXT_NODE
            : right.nodeType;
        return leftType === rightType;
      }
  
      const siblings = node.parentNode ? node.parentNode.children : null;
      if (!siblings) return 0;
      let hasSameNamedElements = false;
      for (let i = 0; i < siblings.length; ++i) {
        if (areNodesSimilar(node, siblings[i]) && siblings[i] !== node) {
          hasSameNamedElements = true;
          break;
        }
      }
      if (!hasSameNamedElements) return 0;
      let ownIndex = 1;
      for (let i = 0; i < siblings.length; ++i) {
        if (areNodesSimilar(node, siblings[i])) {
          if (siblings[i] === node) return ownIndex;
          ++ownIndex;
        }
      }
      return -1;
    };
  })();
  