// Polyfills for QuickJS
if (typeof structuredClone === "undefined") {
  globalThis.structuredClone = function(obj) {
    return JSON.parse(JSON.stringify(obj));
  };
}
if (typeof console === "undefined") {
  globalThis.console = { log: function() {}, warn: function() {}, error: function() {} };
}

// Compile function - expects `source` global to be set
function compile(source) {
  const result = svelte.compile(source, {
    generate: "client",
    runes: true,
    name: "Component",
    css: "injected", // Inject CSS into the JS
  });
  return JSON.stringify({
    code: result.js.code,
    css: result.css ? result.css.code : "",
    error: null,
  });
}

// Check function - returns diagnostics (warnings and errors) without generating code
function check(source, filename) {
  try {
    const result = svelte.compile(source, {
      generate: false, // Don't generate code, just check
      runes: true,
      name: "Component",
      filename: filename || "Component.svelte",
    });

    const diagnostics = (result.warnings || []).map(w => ({
      type: "warning",
      code: w.code || "",
      message: w.message || "",
      filename: w.filename || filename || "",
      start: w.start ? { line: w.start.line, column: w.start.column } : null,
      end: w.end ? { line: w.end.line, column: w.end.column } : null,
    }));

    return JSON.stringify({ diagnostics: diagnostics, error: null });
  } catch (e) {
    // Parse errors come as exceptions
    const diagnostic = {
      type: "error",
      code: e.code || "parse_error",
      message: e.message || String(e),
      filename: e.filename || filename || "",
      start: e.start ? { line: e.start.line, column: e.start.column } : null,
      end: e.end ? { line: e.end.line, column: e.end.column } : null,
    };
    return JSON.stringify({ diagnostics: [diagnostic], error: null });
  }
}
