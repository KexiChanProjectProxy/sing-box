package log

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var approvedLegacyExceptions = map[string]struct{}{
	// Build/tool CLI commands — not runtime logging, uses package-level log for CLI output
	"cmd/internal/app_store_connect/main.go":     {},
	"cmd/internal/build/main.go":              {},
	"cmd/internal/build_libbox/main.go":        {},
	"cmd/internal/build_shared/sdk.go":         {},
	"cmd/internal/format_docs/main.go":         {},
	"cmd/internal/read_tag/main.go":            {},
	"cmd/internal/tun_bench/main.go":           {},
	"cmd/internal/update_android_version/main.go": {},
	"cmd/internal/update_apple_version/main.go": {},
	"cmd/internal/update_certificates/main.go": {},
	"cmd/sing-box/cmd.go":                     {},
	"cmd/sing-box/cmd_check.go":               {},
	"cmd/sing-box/cmd_format.go":              {},
	"cmd/sing-box/cmd_generate.go":            {},
	"cmd/sing-box/cmd_generate_ech.go":        {},
	"cmd/sing-box/cmd_generate_tls.go":         {},
	"cmd/sing-box/cmd_generate_vapid.go":      {},
	"cmd/sing-box/cmd_generate_wireguard.go":  {},
	"cmd/sing-box/cmd_geoip.go":               {},
	"cmd/sing-box/cmd_geoip_export.go":       {},
	"cmd/sing-box/cmd_geoip_list.go":         {},
	"cmd/sing-box/cmd_geoip_lookup.go":        {},
	"cmd/sing-box/cmd_geosite.go":             {},
	"cmd/sing-box/cmd_geosite_export.go":      {},
	"cmd/sing-box/cmd_geosite_list.go":        {},
	"cmd/sing-box/cmd_geosite_lookup.go":      {},
	"cmd/sing-box/cmd_merge.go":                {},
	"cmd/sing-box/cmd_rule_set_compile.go":    {},
	"cmd/sing-box/cmd_rule_set_convert.go":    {},
	"cmd/sing-box/cmd_rule_set_decompile.go":  {},
	"cmd/sing-box/cmd_rule_set_format.go":     {},
	"cmd/sing-box/cmd_rule_set_match.go":      {},
	"cmd/sing-box/cmd_rule_set_merge.go":      {},
	"cmd/sing-box/cmd_rule_set_upgrade.go":    {},
	"cmd/sing-box/cmd_run.go":                 {},
	"cmd/sing-box/cmd_tools_connect.go":       {},
	"cmd/sing-box/cmd_tools_fetch.go":        {},
	"cmd/sing-box/cmd_tools_networkquality.go": {},
	"cmd/sing-box/cmd_tools_stun.go":          {},
	"cmd/sing-box/cmd_tools_synctime.go":     {},
	"cmd/sing-box/generate_completions.go":   {},
	"cmd/sing-box/main.go":                   {},
	"constant/goos/gengoos.go":              {},
	"debug_http.go":                         {},
}

func TestGuardrail(t *testing.T) {
	_, currentFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(currentFile))
	legacyMethods := map[string]bool{"Trace": true, "Debug": true, "Info": true, "Warn": true, "Error": true, "Fatal": true, "Panic": true, "TraceContext": true, "DebugContext": true, "InfoContext": true, "WarnContext": true, "ErrorContext": true, "FatalContext": true, "PanicContext": true}
	var violations []string
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if strings.HasPrefix(relSlash, "log/") {
			return nil
		}
		if _, approved := approvedLegacyExceptions[relSlash]; approved {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !legacyMethods[selector.Sel.Name] {
				return true
			}
			if ident, ok := selector.X.(*ast.Ident); ok && (ident.Name == "log" || ident.Name == "logger") {
				position := fset.Position(selector.Pos())
				violations = append(violations, filepath.ToSlash(rel)+":"+strconvItoa(position.Line)+":"+selector.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("legacy logger calls must migrate to structured *Event APIs or be added to approvedLegacyExceptions with rationale:\n%s", strings.Join(violations, "\n"))
	}
}

func strconvItoa(value int) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}
