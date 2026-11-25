// Package archive writes a notebook folder to a single .tessivo.zip file and
// reads one back (NB-4). Archives come from other people, so reading treats
// them as hostile: sizes and counts are bounded while reading (declared sizes
// are not trusted), names may not leave the destination, links are refused,
// and nothing is left behind when an archive is refused.
package archive

import "errors"

// The archive layout: archive.json, then the notebook's own files under Root.
const (
	FormatVersion = 1
	MetaName      = "archive.json"
	Root          = "notebook/"

	KindNotebook = "notebook"
	KindResearch = "research"
)

// Limits on what Extract accepts.
const (
	MaxEntries = 5000
	MaxTotal   = 512 << 20
	MaxEntry   = 64 << 20
	maxMeta    = 64 << 10
)

// The limits Extract uses; tests lower them.
var (
	maxEntries int64 = MaxEntries
	maxTotal   int64 = MaxTotal
	maxEntry   int64 = MaxEntry
)

// Errors Extract wraps, so callers can tell a bad archive from a disk error.
var (
	ErrTooLarge   = errors.New("archive is too large")
	ErrUnsafePath = errors.New("archive has an unsafe entry")
	ErrFormat     = errors.New("not a Tessivo archive")
)

// Meta is archive.json.
type Meta struct {
	Kind           string   `json:"kind"` // KindNotebook or KindResearch
	FormatVersion  int      `json:"formatVersion"`
	TessivoVersion string   `json:"tessivoVersion,omitempty"`
	CreatedAt      string   `json:"createdAt,omitempty"`
	Includes       []string `json:"includes,omitempty"` // optional parts included, e.g. "outputs"
}
