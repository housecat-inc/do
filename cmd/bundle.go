package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/housecat-inc/do/pkg/svelte"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var bundleVerbose bool

var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Bundle Svelte components into dist/app.min.js",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Find all .svelte files
		var components []string
		err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			if d.IsDir() {
				if name == "node_modules" || name == "dist" || (name != "." && strings.HasPrefix(name, ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			// Skip hidden files and non-svelte files
			if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".svelte") {
				return nil
			}
			components = append(components, path)
			return nil
		})
		if err != nil {
			return errors.WithStack(err)
		}

		if len(components) == 0 {
			fmt.Println("No .svelte files found")
			return nil
		}

		// Compile each component and build virtual filesystem
		var imports []string
		var exports []string
		stdinContents := make(map[string]string)

		for _, path := range components {
			src, err := os.ReadFile(path)
			if err != nil {
				return errors.WithStack(err)
			}

			code, err := svelte.Compile(string(src))
			if err != nil {
				return errors.Errorf("compile %s: %v", path, err)
			}

			// Export key matches filesystem: src/animate/Foo.svelte -> src/animate/Foo
			exportKey := strings.TrimSuffix(path, ".svelte")

			if bundleVerbose {
				fmt.Printf("%s -> %s\n", path, exportKey)
			}

			// Create safe identifier from path: src/forms/Button -> src_forms_Button
			ident := strings.ReplaceAll(exportKey, "/", "_")
			ident = strings.ReplaceAll(ident, "-", "_")
			ident = strings.ReplaceAll(ident, ".", "_")

			// Add to virtual filesystem
			virtualPath := ident + ".js"
			stdinContents[virtualPath] = code

			imports = append(imports, fmt.Sprintf("import %s from './%s'", ident, virtualPath))
			exports = append(exports, fmt.Sprintf("  '%s': %s", exportKey, ident))
		}

		// Create entry point
		entry := fmt.Sprintf("%s\n\nexport default {\n%s\n}\n",
			strings.Join(imports, "\n"),
			strings.Join(exports, ",\n"))

		// Bundle with esbuild
		result := api.Build(api.BuildOptions{
			Stdin: &api.StdinOptions{
				Contents:   entry,
				ResolveDir: ".",
				Loader:     api.LoaderJS,
			},
			Bundle:            true,
			MinifyWhitespace:  true,
			MinifyIdentifiers: true,
			MinifySyntax:      true,
			Format:            api.FormatESModule,
			External:          []string{"svelte", "svelte/*"},
			Outfile:           "dist/app.min.js",
			Write:             true,
			Plugins: []api.Plugin{{
				Name: "svelte-components",
				Setup: func(build api.PluginBuild) {
					build.OnResolve(api.OnResolveOptions{Filter: `^\.\/.*\.js$`},
						func(args api.OnResolveArgs) (api.OnResolveResult, error) {
							path := strings.TrimPrefix(args.Path, "./")
							if _, ok := stdinContents[path]; ok {
								return api.OnResolveResult{
									Path:      path,
									Namespace: "svelte-components",
								}, nil
							}
							return api.OnResolveResult{}, nil
						})
					build.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: "svelte-components"},
						func(args api.OnLoadArgs) (api.OnLoadResult, error) {
							contents := stdinContents[args.Path]
							return api.OnLoadResult{
								Contents: &contents,
								Loader:   api.LoaderJS,
							}, nil
						})
				},
			}},
		})

		if len(result.Errors) > 0 {
			for _, err := range result.Errors {
				fmt.Fprintf(os.Stderr, "esbuild: %s\n", err.Text)
			}
			return errors.New("esbuild bundling failed")
		}

		fmt.Printf("Bundled %d components into dist/app.min.js\n", len(components))
		return nil
	},
}

func init() {
	bundleCmd.Flags().BoolVarP(&bundleVerbose, "verbose", "v", false, "show each file and its export path")
	rootCmd.AddCommand(bundleCmd)
}
