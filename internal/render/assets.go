package render

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/armandmcqueen/ccsessions/internal/model"
)

// extImage maps an image media type to a file extension.
func extImage(mediaType string) string {
	switch mediaType {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "image/svg+xml":
		return "svg"
	default:
		return "bin"
	}
}

// assignImageRefs walks every tool result image in the session, computes a stable
// content-addressed asset path (<stem>.assets/img-<sha8>.<ext>), records it on the
// image as Ref, and returns the deduplicated asset Outputs to be written. Both
// renderers call this so the extracted files and the links agree regardless of
// which formats are selected.
func assignImageRefs(stem string, s *model.Session) []Output {
	var outputs []Output
	seen := make(map[string]bool)
	assetDir := stem + ".assets"

	for ti := range s.Turns {
		for ri := range s.Turns[ti].Responses {
			calls := s.Turns[ti].Responses[ri].ToolCalls
			for ci := range calls {
				imgs := calls[ci].ResultImages
				for ii := range imgs {
					img := &imgs[ii]
					sum := sha256.Sum256(img.Data)
					name := "img-" + hex.EncodeToString(sum[:])[:8] + "." + extImage(img.MediaType)
					rel := assetDir + "/" + name
					img.Ref = rel
					if !seen[rel] {
						seen[rel] = true
						outputs = append(outputs, Output{RelPath: rel, Bytes: img.Data})
					}
				}
			}
		}
	}
	return outputs
}
