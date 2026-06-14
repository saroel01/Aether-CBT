package soalpkg

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// ErrPathTraversal is returned when a requested content path resolves outside the package
// directory (Requirement 8.3, Property 4).
var ErrPathTraversal = errors.New("soalpkg: requested path escapes the package directory")

// ResolvePath joins relPath onto packageDir and rejects any result that escapes the
// package directory (Requirement 8.3, Property 4).
func ResolvePath(packageDir, relPath string) (string, error) {
	target := filepath.Join(packageDir, relPath)
	if !isWithin(packageDir, target) {
		return "", ErrPathTraversal
	}
	return target, nil
}

// ServeContent streams the resolved file to w without loading it fully into memory
// (Requirement 8.6, 13.3). A missing file surfaces as an os.PathError for the caller to
// map to HTTP 404.
func ServeContent(w io.Writer, packageDir, relPath string) error {
	target, err := ResolvePath(packageDir, relPath)
	if err != nil {
		return err
	}
	f, err := os.Open(target)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

// headOpenRe matches the opening <head ...> tag, case-insensitive. The shim is injected
// immediately after it so the network-layer overrides are installed before the player
// loads (Requirement 9.1, AD-3).
var headOpenRe = regexp.MustCompile(`(?i)<head[^>]*>`)

// ServeIndexWithShim streams the package's entry HTML with the shim injected right after
// the opening <head> tag. The file on disk is never modified - the injection exists only
// in the response stream (Requirement 9.5, Property 12).
//
// Note: the entry HTML is read fully so the injection point can be located. At very large
// scale this can be made streaming (scan for <head> then io.Copy the tail) - tracked as a
// follow-up to AD-4.
func ServeIndexWithShim(w io.Writer, packageDir, entryPath string, ctx ShimContext) error {
	target, err := ResolvePath(packageDir, entryPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	_, err = w.Write(injectShim(data, ctx))
	return err
}

func injectShim(html []byte, ctx ShimContext) []byte {
	block := []byte(InjectionHTML(ctx))
	loc := headOpenRe.FindIndex(html)
	if loc == nil {
		// No <head> tag: prepend the shim so it still runs before any body script.
		return append(block, html...)
	}
	out := make([]byte, 0, len(html)+len(block))
	out = append(out, html[:loc[1]]...) // through the end of <head...>
	out = append(out, block...)
	out = append(out, html[loc[1]:]...)
	return out
}
