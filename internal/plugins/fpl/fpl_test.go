package fpl

import (
	"github.com/aioproxy/aioproxy/internal/core"
	"strings"
	"testing"
)

func TestParseFPL(t *testing.T) {
	rep := &core.ImportReport{}
	c := parse(strings.NewReader("http://1.2.3.4:80\nsocks5://5.6.7.8:1080\nsocks4://1.1.1.1:1\n"), rep)
	if len(c) != 2 {
		t.Fatalf("got %d", len(c))
	}
	if rep.SkipReasons["socks4_unsupported"] != 1 {
		t.Fatalf("skip=%v", rep.SkipReasons)
	}
}
