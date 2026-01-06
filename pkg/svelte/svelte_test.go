package svelte_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/housecat-inc/do/pkg/svelte"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck(t *testing.T) {
	ctx := t.Context()
	_ = ctx
	r := require.New(t)
	a := assert.New(t)

	tests := []struct {
		name         string
		src          string
		wantCode     string
		wantErrors   bool
		wantWarnings int
	}{
		{
			name: "valid_component",
			src: `<script>
	let count = $state(0);
</script>
<button onclick={() => count++}>Clicks: {count}</button>`,
			wantErrors:   false,
			wantWarnings: 0,
		},
		{
			name: "unused_css",
			src: `<div>Hello</div>
<style>.unused { color: red; }</style>`,
			wantCode:     "css_unused_selector",
			wantErrors:   false,
			wantWarnings: 1,
		},
		{
			name:         "a11y_warning",
			src:          `<img src="test.png">`,
			wantCode:     "a11y_missing_attribute",
			wantErrors:   false,
			wantWarnings: 1,
		},
		{
			name:       "syntax_error",
			src:        `<script>let x = </script>`,
			wantErrors: true,
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			diags, err := svelte.Check(ts.src, ts.name+".svelte")

			if ts.wantErrors {
				hasError := err != nil
				if !hasError && len(diags) > 0 {
					for _, d := range diags {
						if d.Type == "error" {
							hasError = true
							break
						}
					}
				}
				a.True(hasError, "expected error for %s", ts.name)
				return
			}

			r.NoError(err)

			warnings := 0
			for _, d := range diags {
				if d.Type == "warning" {
					warnings++
				}
			}
			a.Equal(ts.wantWarnings, warnings, "warning count mismatch for %s", ts.name)

			if ts.wantCode != "" && len(diags) > 0 {
				a.Equal(ts.wantCode, diags[0].Code)
			}
		})
	}
}

func TestCheckDir(t *testing.T) {
	ctx := t.Context()
	_ = ctx
	r := require.New(t)
	a := assert.New(t)

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "Good.svelte"), []byte(`<div>Hello</div>`), 0644)
	r.NoError(err)
	err = os.WriteFile(filepath.Join(tmpDir, "Bad.svelte"), []byte(`<img src="x.png">`), 0644)
	r.NoError(err)

	diags, err := svelte.CheckDir(tmpDir)
	r.NoError(err)
	a.Len(diags, 1)
	a.Equal("warning", diags[0].Type)
	a.Equal("a11y_missing_attribute", diags[0].Code)
	a.Contains(diags[0].Filename, "Bad.svelte")
}
