package svelte

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"modernc.org/quickjs"
)

// Position represents a location in source code.
type Position struct {
	Column int `json:"column"`
	Line   int `json:"line"`
}

// Diagnostic represents a compiler warning or error.
type Diagnostic struct {
	Code     string    `json:"code"`
	End      *Position `json:"end"`
	Filename string    `json:"filename"`
	Message  string    `json:"message"`
	Start    *Position `json:"start"`
	Type     string    `json:"type"`
}

//go:generate curl -so compiler.min.js https://esm.sh/svelte@5.46.1/compiler/index.js?raw

//go:embed compile.js
var compileJS string

//go:embed compiler.min.js
var compilerJS string

// html serves a pre-compiled Svelte component with esm.sh runtime
const html = `<!DOCTYPE html>
<html>
<head>
	<title>Test</title>
	<script type="importmap">
	{
		"imports": {
			"svelte": "https://esm.sh/svelte@5.46.1",
			"svelte/": "https://esm.sh/svelte@5.46.1/"
		}
	}
	</script>
</head>
<body>
	<div id="app"></div>
	<script type="module">
%s

		import { mount } from 'svelte';
		mount(Component, { target: document.getElementById('app') });
	</script>
</body>
</html>
`

// Compile compiles a Svelte component using QuickJS and returns the JS code.
func Compile(src string) (string, error) {
	vm, err := quickjs.NewVM()
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer func() { _ = vm.Close() }()

	if _, err = vm.Eval(compilerJS, 0); err != nil {
		return "", errors.WithStack(err)
	}

	if _, err = vm.Eval(compileJS, 0); err != nil {
		return "", errors.WithStack(err)
	}

	sourceJSON, err := json.Marshal(src)
	if err != nil {
		return "", errors.WithStack(err)
	}

	result, err := vm.Eval(fmt.Sprintf("compile(%s)", sourceJSON), 0)
	if err != nil {
		return "", errors.WithStack(err)
	}

	var out struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.(string)), &out); err != nil {
		return "", errors.WithStack(err)
	}
	if out.Error != "" {
		return "", errors.Errorf("svelte: %s", out.Error)
	}

	return out.Code, nil
}

// Check validates a Svelte component and returns diagnostics (warnings/errors).
// Unlike Compile, it does not generate output code - it only checks for issues.
func Check(src, filename string) ([]Diagnostic, error) {
	vm, err := quickjs.NewVM()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer func() { _ = vm.Close() }()

	if _, err = vm.Eval(compilerJS, 0); err != nil {
		return nil, errors.WithStack(err)
	}

	if _, err = vm.Eval(compileJS, 0); err != nil {
		return nil, errors.WithStack(err)
	}

	sourceJSON, err := json.Marshal(src)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	filenameJSON, err := json.Marshal(filename)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	result, err := vm.Eval(fmt.Sprintf("check(%s, %s)", sourceJSON, filenameJSON), 0)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var out struct {
		Diagnostics []Diagnostic `json:"diagnostics"`
		Error       string       `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.(string)), &out); err != nil {
		return nil, errors.WithStack(err)
	}
	if out.Error != "" {
		return nil, errors.Errorf("svelte: %s", out.Error)
	}

	return out.Diagnostics, nil
}

// CheckDir walks a directory and checks all .svelte files, returning all diagnostics.
// It skips node_modules and hidden directories by default.
func CheckDir(root string) ([]Diagnostic, error) {
	var all []Diagnostic

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".svelte") {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return errors.WithStack(err)
		}

		diags, err := Check(string(src), path)
		if err != nil {
			all = append(all, Diagnostic{
				Code:     "check_error",
				Filename: path,
				Message:  err.Error(),
				Type:     "error",
			})
			return nil
		}

		all = append(all, diags...)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return all, nil
}

// Handler returns an http.Handler that serves a compiled Svelte 5 component.
func Handler(src string) (http.Handler, error) {
	code, err := Compile(src)
	if err != nil {
		return nil, err
	}

	html := fmt.Sprintf(html, code)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}), nil
}
