package format_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// format and the client decode blocks fetched over the network, so they must
// not link segment's file I/O. pebble/vfs reaches sentry-go, net/http and
// crypto/tls through cockroachdb/errors, and nothing on the decode path needs
// them. The client does its own networking, so only format excludes net.
var storageDeps = []string{
	"github.com/bluesky-social/jetstream/segment",
	"github.com/cockroachdb/",
	"github.com/getsentry/",
}

func TestNoStorageDependencies(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		pkg       string
		forbidden []string
	}{
		{".", append([]string{"net", "net/", "crypto/tls"}, storageDeps...)},
		{"github.com/bluesky-social/jetstream", storageDeps},
	} {
		out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", tc.pkg).Output()
		require.NoError(t, err)
		var offenders []string
		for _, dep := range strings.Fields(string(out)) {
			for _, f := range tc.forbidden {
				if dep == f || (strings.HasSuffix(f, "/") && strings.HasPrefix(dep, f)) {
					offenders = append(offenders, dep)
				}
			}
		}
		require.Empty(t, offenders, "%s must not depend on segment's storage", tc.pkg)
	}
}
