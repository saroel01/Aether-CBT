package soalpkg

import (
	"strings"
	"testing"
)

func TestStripMobileLauncherRedirect_RemovesRedirectBlock(t *testing.T) {
	input := `<html><head>
<script>(function(){var useMobilePlayer=navigator.userAgent.match(/iPhone|iPad|iPod/i);if(useMobilePlayer){location.replace("ismplayer.html");}})();</script>
<script src="data/browsersupport.js"></script>
</head><body><div id="content"></div></body></html>`
	out := StripMobileLauncherRedirect([]byte(input))
	if strings.Contains(string(out), "ismplayer.html") {
		t.Error("redirect masih ada")
	}
	if strings.Contains(string(out), "useMobilePlayer") {
		t.Error("var masih ada")
	}
	if !strings.Contains(string(out), "browsersupport.js") {
		t.Error("script lain ikut terhapus")
	}
	if !strings.Contains(string(out), `<div id="content">`) {
		t.Error("body rusak")
	}
}

func TestStripMobileLauncherRedirect_Idempotent(t *testing.T) {
	in := []byte(`<script>(function(){var useMobilePlayer=true;if(useMobilePlayer){location.replace("ismplayer.html");}})();</script>`)
	if string(StripMobileLauncherRedirect(in)) !=
		string(StripMobileLauncherRedirect(StripMobileLauncherRedirect(in))) {
		t.Error("tidak idempotent")
	}
}

func TestStripMobileLauncherRedirect_NoOpWhenNoRedirect(t *testing.T) {
	in := []byte(`<html><script src="data/player.js"></script></html>`)
	if string(StripMobileLauncherRedirect(in)) != string(in) {
		t.Error("harus no-op bila tidak ada redirect")
	}
}
