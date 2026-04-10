package display

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// PrettyJSON indents raw JSON and, when colors are enabled, runs the
// result through chroma's JSON lexer + a terminal formatter for proper
// token-level syntax highlighting.
//
// If body is not valid JSON, PrettyJSON returns it unchanged — probe
// output for plain-text FS commands ("status", "uptime") should still
// work.
func PrettyJSON(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return body
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(trimmed), "", "  "); err != nil {
		return body
	}
	pretty := buf.String()
	if !colorEnabled {
		return pretty
	}
	return highlightJSON(pretty)
}

func highlightJSON(src string) string {
	lexer := lexers.Get("json")
	if lexer == nil {
		return src
	}
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		return src
	}
	iterator, err := lexer.Tokenise(nil, src)
	if err != nil {
		return src
	}
	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return src
	}
	return buf.String()
}
