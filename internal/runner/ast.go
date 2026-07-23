package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// runPropertyContainsCall checks whether an object property named `property`
// contains a call to `callee` anywhere in its value. It mirrors the upstream
// propertyContainsCall AST check using scenario-specific string inspection.
func runPropertyContainsCall(in Input, params map[string]any) (bool, string) {
	file := stringParam(params, "file")
	property := stringParam(params, "property")
	callee := stringParam(params, "callee")
	if file == "" {
		return false, "missing 'file' parameter"
	}
	if property == "" {
		return false, "missing 'property' parameter"
	}
	if callee == "" {
		return false, "missing 'callee' parameter"
	}

	content, err := os.ReadFile(filepath.Join(in.WorkDir, file))
	if err != nil {
		return false, fmt.Sprintf("could not read %s: %v", file, err)
	}
	src := removeComments(string(content))
	callRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(callee) + `\s*\(`)

	positions := findPropertyKeyPositions(src, property)
	if len(positions) == 0 {
		return false, fmt.Sprintf("property %q not found in %s", property, file)
	}

	for _, pos := range positions {
		var region string
		switch pos.kind {
		case propAssign:
			region = extractObjectValueRegion(src, pos.end)
		case propMethod:
			bodyStart := findMethodBodyStart(src, pos.end)
			if bodyStart >= 0 {
				region = extractBracedBlock(src, bodyStart)
			}
		case propShorthand:
			region = extractDeclarationBlock(src, property)
		}
		if region != "" && callRe.MatchString(region) {
			return true, fmt.Sprintf("property %q contains call to %s in %s", property, callee, file)
		}
	}

	return false, fmt.Sprintf("property %q does not contain call to %s in %s", property, callee, file)
}

// runFileCalls checks whether `file` contains a call to `callee`.
func runFileCalls(in Input, params map[string]any) (bool, string) {
	file := stringParam(params, "file")
	callee := stringParam(params, "callee")
	if file == "" {
		return false, "missing 'file' parameter"
	}
	if callee == "" {
		return false, "missing 'callee' parameter"
	}

	content, err := os.ReadFile(filepath.Join(in.WorkDir, file))
	if err != nil {
		return false, fmt.Sprintf("could not read %s: %v", file, err)
	}
	src := removeComments(string(content))
	callRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(callee) + `\s*\(`)
	if callRe.MatchString(src) {
		return true, fmt.Sprintf("%s called in %s", callee, file)
	}
	return false, fmt.Sprintf("%s not called in %s", callee, file)
}

// runJsxPassesProp checks whether a JSX element named `component` passes the
// prop `prop` (or uses a spread).
func runJsxPassesProp(in Input, params map[string]any) (bool, string) {
	file := stringParam(params, "file")
	component := stringParam(params, "component")
	prop := stringParam(params, "prop")
	if file == "" {
		return false, "missing 'file' parameter"
	}
	if component == "" {
		return false, "missing 'component' parameter"
	}
	if prop == "" {
		return false, "missing 'prop' parameter"
	}

	content, err := os.ReadFile(filepath.Join(in.WorkDir, file))
	if err != nil {
		return false, fmt.Sprintf("could not read %s: %v", file, err)
	}
	src := removeComments(string(content))

	tagPattern := regexp.MustCompile(`<` + regexp.QuoteMeta(component) + `\b`)
	propPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(prop) + `\s*=|\{\s*\.\.\.`)

	for _, tag := range extractJSXOpeningTags(src, component) {
		if propPattern.MatchString(tag) {
			return true, fmt.Sprintf("%s passes prop %q in %s", component, prop, file)
		}
	}
	_ = tagPattern

	return false, fmt.Sprintf("%s does not pass prop %q in %s", component, prop, file)
}

// propKind describes how a property key is used in an object literal.
type propKind int

const (
	propAssign propKind = iota
	propMethod
	propShorthand
)

type propPos struct {
	kind propKind
	end  int // index in src just after the property name
}

func findPropertyKeyPositions(src, name string) []propPos {
	var positions []propPos
	wordRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	for _, loc := range wordRe.FindAllStringIndex(src, -1) {
		end := loc[1]
		// skip plain references like variables named "loader" outside object literals
		skip := false
		next := end
		for next < len(src) && (src[next] == ' ' || src[next] == '\t' || src[next] == '\n' || src[next] == '\r') {
			next++
		}
		if next >= len(src) {
			continue
		}
		ch := src[next]
		var kind propKind
		switch ch {
		case ':':
			kind = propAssign
		case '(':
			kind = propMethod
		case ',', '}':
			kind = propShorthand
		default:
			// could be part of another identifier or property access; skip
			skip = true
		}
		if skip {
			continue
		}
		positions = append(positions, propPos{kind: kind, end: end})
	}
	return positions
}

// extractObjectValueRegion returns the source region after a colon up to (but
// not including) the matching top-level comma or closing brace.
func extractObjectValueRegion(src string, start int) string {
	idx := start
	for idx < len(src) && src[idx] != ':' {
		idx++
	}
	if idx >= len(src) {
		return ""
	}
	idx++ // skip ':'
	depth := 0
	inString := byte(0)
	escape := false
	for i := idx; i < len(src); i++ {
		c := src[i]
		if inString != 0 {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == inString {
				inString = 0
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			inString = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				return src[idx:i]
			}
		}
	}
	return src[idx:]
}

// findMethodBodyStart returns the index of the opening brace of a method body.
func findMethodBodyStart(src string, afterName int) int {
	depth := 0
	inString := byte(0)
	escape := false
	for i := afterName; i < len(src); i++ {
		c := src[i]
		if inString != 0 {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == inString {
				inString = 0
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			inString = c
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
				if depth == 0 {
					// next non-space should be '{'
					for j := i + 1; j < len(src); j++ {
						if src[j] == '{' {
							return j
						}
						if src[j] != ' ' && src[j] != '\t' && src[j] != '\n' && src[j] != '\r' {
							return -1
						}
					}
				}
			}
		}
	}
	return -1
}

// extractDeclarationBlock finds a top-level variable or function declaration
// with the given name and returns its braced body if present, or the whole
// declaration otherwise.
func extractDeclarationBlock(src, name string) string {
	quoted := regexp.QuoteMeta(name)
	patterns := []string{
		`(?:export\s+)?(?:async\s+)?function\s+` + quoted + `\s*\([\s\S]*?\)\s*\{`,
		`(?:export\s+)?(?:const|let|var)\s+` + quoted + `\s*=\s*[\s\S]*?=>\s*\{`,
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if loc := re.FindStringIndex(src); loc != nil {
			return extractBracedBlock(src, loc[0])
		}
	}
	return ""
}

// extractJSXOpeningTags returns the raw source of each opening/self-closing JSX
// element with the given component name.
func extractJSXOpeningTags(src, component string) []string {
	var tags []string
	tagStartRe := regexp.MustCompile(`<` + regexp.QuoteMeta(component) + `\b`)
	for _, loc := range tagStartRe.FindAllStringIndex(src, -1) {
		start := loc[0]
		end := findTagEnd(src, start)
		if end > start {
			tags = append(tags, src[start:end+1])
		}
	}
	return tags
}

// findTagEnd returns the index of the '>' that closes the JSX opening tag,
// skipping quoted strings. JSX expressions inside tag attributes are rare
// enough in our scenarios that we do not need full brace tracking here.
func findTagEnd(src string, start int) int {
	inString := byte(0)
	escape := false
	for i := start + 1; i < len(src); i++ {
		c := src[i]
		if inString != 0 {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == inString {
				inString = 0
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			inString = c
		case '>':
			return i
		}
	}
	return -1
}

// removeComments strips // and /* */ comments from JavaScript/TypeScript source.
func removeComments(src string) string {
	re := regexp.MustCompile(`//[^\n]*|/\*[\s\S]*?\*/`)
	return re.ReplaceAllString(src, "")
}

// runComponentUsesHook checks whether a component function body contains a call
// to the named hook. Mirrors upstream componentUsesHook.
func runComponentUsesHook(in Input, params map[string]any) (bool, string) {
	file := stringParam(params, "file")
	component := stringParam(params, "component")
	hook := stringParam(params, "hook")
	if file == "" {
		return false, "missing 'file' parameter"
	}
	if component == "" {
		return false, "missing 'component' parameter"
	}
	if hook == "" {
		return false, "missing 'hook' parameter"
	}

	content, err := os.ReadFile(filepath.Join(in.WorkDir, file))
	if err != nil {
		return false, fmt.Sprintf("could not read %s: %v", file, err)
	}
	src := removeComments(string(content))

	body := extractDeclarationBlock(src, component)
	if body == "" {
		return false, fmt.Sprintf("could not find component %q in %s", component, file)
	}

	callRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(hook) + `\s*\(`)
	if callRe.MatchString(body) {
		return true, fmt.Sprintf("%s uses %s in %s", component, hook, file)
	}
	return false, fmt.Sprintf("%s does not use %s in %s", component, hook, file)
}

// runFirstDirective checks the first module directive (e.g. "use client") in a file.
// Mirrors upstream firstDirective.
func runFirstDirective(in Input, params map[string]any) (bool, string) {
	file := stringParam(params, "file")
	expected := stringParam(params, "expected")
	if file == "" {
		return false, "missing 'file' parameter"
	}
	if expected == "" {
		return false, "missing 'expected' parameter"
	}

	content, err := os.ReadFile(filepath.Join(in.WorkDir, file))
	if err != nil {
		return false, fmt.Sprintf("could not read %s: %v", file, err)
	}
	src := removeComments(string(content))

	re := regexp.MustCompile(`^\s*["']([^"']+)["']\s*;?`)
	m := re.FindStringSubmatch(src)
	if len(m) < 2 {
		return false, fmt.Sprintf("no module directive found in %s", file)
	}
	if m[1] == expected {
		return true, fmt.Sprintf("first directive is %q in %s", expected, file)
	}
	return false, fmt.Sprintf("first directive is %q, expected %q in %s", m[1], expected, file)
}

// runFileImports checks whether a file imports from one of the specified modules.
// Mirrors upstream importsOf with a contains check.
func runFileImports(in Input, params map[string]any) (bool, string) {
	file := stringParam(params, "file")
	module := stringParam(params, "module")
	if file == "" {
		return false, "missing 'file' parameter"
	}
	if module == "" {
		return false, "missing 'module' parameter"
	}

	content, err := os.ReadFile(filepath.Join(in.WorkDir, file))
	if err != nil {
		return false, fmt.Sprintf("could not read %s: %v", file, err)
	}
	src := removeComments(string(content))

	re := regexp.MustCompile(`from\s+["']` + regexp.QuoteMeta(module) + `["']`)
	if re.MatchString(src) {
		return true, fmt.Sprintf("%s imports from %s", file, module)
	}
	return false, fmt.Sprintf("%s does not import from %s", file, module)
}

// runFileNotImports checks whether a file does NOT import from the specified module.
func runFileNotImports(in Input, params map[string]any) (bool, string) {
	file := stringParam(params, "file")
	module := stringParam(params, "module")
	if file == "" {
		return false, "missing 'file' parameter"
	}
	if module == "" {
		return false, "missing 'module' parameter"
	}

	content, err := os.ReadFile(filepath.Join(in.WorkDir, file))
	if err != nil {
		return false, fmt.Sprintf("could not read %s: %v", file, err)
	}
	src := removeComments(string(content))

	re := regexp.MustCompile(`from\s+["']` + regexp.QuoteMeta(module) + `["']`)
	if !re.MatchString(src) {
		return true, fmt.Sprintf("%s does not import from %s", file, module)
	}
	return false, fmt.Sprintf("%s unexpectedly imports from %s", file, module)
}

// runMutationRefreshesQuery checks whether a useMutation call has an
// onSuccess/onSettled handler that calls invalidateQueries or refetchQueries
// with the specified queryKey. Mirrors upstream mutationRefreshesQuery.
func runMutationRefreshesQuery(in Input, params map[string]any) (bool, string) {
	file := stringParam(params, "file")
	mutation := stringParam(params, "mutation")
	queryKey := stringParam(params, "queryKey")
	if file == "" {
		return false, "missing 'file' parameter"
	}
	if mutation == "" {
		return false, "missing 'mutation' parameter"
	}
	if queryKey == "" {
		return false, "missing 'queryKey' parameter"
	}

	content, err := os.ReadFile(filepath.Join(in.WorkDir, file))
	if err != nil {
		return false, fmt.Sprintf("could not read %s: %v", file, err)
	}
	src := removeComments(string(content))

	body := extractDeclarationBlock(src, mutation)
	if body == "" {
		return false, fmt.Sprintf("could not find mutation %q in %s", mutation, file)
	}

	hasUseMutation := regexp.MustCompile(`\buseMutation\s*\(`).MatchString(body)
	if !hasUseMutation {
		return false, fmt.Sprintf("%s does not use useMutation in %s", mutation, file)
	}

	hasHandler := regexp.MustCompile(`\b(?:onSuccess|onSettled)\s*[:(]`).MatchString(body)
	if !hasHandler {
		return false, fmt.Sprintf("%s has no onSuccess/onSettled handler in %s", mutation, file)
	}

	hasInvalidate := regexp.MustCompile(`\b(?:invalidateQueries|refetchQueries)\s*\(`).MatchString(body)
	if !hasInvalidate {
		return false, fmt.Sprintf("%s handler does not call invalidateQueries/refetchQueries in %s", mutation, file)
	}

	keyRe := regexp.MustCompile(`["']` + regexp.QuoteMeta(queryKey) + `["']`)
	if !keyRe.MatchString(body) {
		return false, fmt.Sprintf("%s handler does not reference queryKey %q in %s", mutation, queryKey, file)
	}

	return true, fmt.Sprintf("%s invalidates query %q in %s", mutation, queryKey, file)
}

// runUseFormUsesResolver checks whether a useForm call wires resolver to the
// named schema. Mirrors upstream useFormUsesResolver.
func runUseFormUsesResolver(in Input, params map[string]any) (bool, string) {
	file := stringParam(params, "file")
	schema := stringParam(params, "schema")
	if file == "" {
		return false, "missing 'file' parameter"
	}
	if schema == "" {
		return false, "missing 'schema' parameter"
	}

	content, err := os.ReadFile(filepath.Join(in.WorkDir, file))
	if err != nil {
		return false, fmt.Sprintf("could not read %s: %v", file, err)
	}
	src := removeComments(string(content))

	hasUseForm := regexp.MustCompile(`\buseForm\s*\(`).MatchString(src)
	if !hasUseForm {
		return false, fmt.Sprintf("no useForm call found in %s", file)
	}

	hasResolver := regexp.MustCompile(`\bresolver\s*:\s*\w+Resolver\s*\(\s*` + regexp.QuoteMeta(schema) + `\s*\)`).MatchString(src)
	if hasResolver {
		return true, fmt.Sprintf("useForm uses resolver with %s in %s", schema, file)
	}

	hasResolver2 := regexp.MustCompile(`\bresolver\s*:\s*\w+\s*\(\s*` + regexp.QuoteMeta(schema) + `\s*\)`).MatchString(src)
	if hasResolver2 {
		return true, fmt.Sprintf("useForm uses resolver with %s in %s", schema, file)
	}

	return false, fmt.Sprintf("useForm does not wire resolver to %s in %s", schema, file)
}
