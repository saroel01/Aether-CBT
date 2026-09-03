package soalpkg

import (
	"bytes"
	"regexp"
)

// scriptBlockPattern matches a complete <script ...>...</script> block (including its content),
// case-insensitive, across newlines. Used to surgically remove iSpring's mobile-launcher
// redirect block without disturbing other scripts (Requirement: P1 layer 1, defense-in-depth).
var scriptBlockPattern = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)

// StripMobileLauncherRedirect removes iSpring QuizMaker's mobile-launcher redirect block from an
// index.html payload.
//
// Published iSpring packages include a <head> script that, when opened on iPhone/iPad/Android,
// calls location.replace("ismplayer.html") to show an "Open in app / View in browser" launcher
// instead of running the quiz inline. On a CBT platform the quiz must run inline in the browser
// (inside the same-origin exam iframe), so that redirect must be neutralized.
//
// This function scans every <script> block and drops only the one that both (a) references the
// "useMobilePlayer" detection variable and (b) targets "ismplayer.html". Other scripts
// (browsersupport.js, player.js, anything else) are left untouched. It is idempotent and a no-op
// when no such block is present, so it is safe to call on any index.html, including future
// iSpring versions that may not embed the redirect.
//
// This is layer 1 of a 3-layer defense-in-depth (server strip -> shim inject point unchanged ->
// client-side location.replace/assign override in the shim). Layers 2 and 3 catch the case where a
// future iSpring version changes the redirect mechanism enough to evade this regex.
func StripMobileLauncherRedirect(html []byte) []byte {
	if !bytes.Contains(html, []byte("ismplayer.html")) {
		return html
	}
	return scriptBlockPattern.ReplaceAllFunc(html, func(match []byte) []byte {
		lower := bytes.ToLower(match)
		if bytes.Contains(lower, []byte("usemobileplayer")) &&
			bytes.Contains(lower, []byte("ismplayer.html")) {
			return nil // drop the block
		}
		return match
	})
}
