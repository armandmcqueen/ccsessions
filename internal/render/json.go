package render

import (
	"bytes"
	"encoding/json"

	"github.com/armandmcqueen/ccsessions/internal/model"
)

// JSON renders a session as indented JSON of the parsed model, plus extracted
// image assets. Image bytes are written as files and referenced by Ref in the
// json (the raw bytes are not embedded).
type JSON struct{}

func (JSON) Name() string { return "json" }

func (JSON) MainExt() string { return ".json" }

func (JSON) Render(s *model.Session) ([]Output, error) {
	stem := OutputStem(s)
	assets := assignImageRefs(stem, s) // sets Image.Ref before marshalling

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return nil, err
	}

	outputs := make([]Output, 0, 1+len(assets))
	outputs = append(outputs, Output{RelPath: stem + ".json", Bytes: buf.Bytes()})
	outputs = append(outputs, assets...)
	return outputs, nil
}
