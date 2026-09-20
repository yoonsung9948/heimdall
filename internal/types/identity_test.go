package types

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/yoonsung9948/heimdall/internal/config"
)

func TestWithIdentity_RoundTrip(t *testing.T) {
	want := &Identity{User: "alice", Client: "cli-1", Groups: []string{"admins"}}

	ctx := WithIdentity(context.Background(), want)

	got, ok := IdentityFromContext(ctx)
	if !ok {
		t.Fatal("IdentityFromContext() ok = false, want true")
	}
	if got != want {
		t.Error("IdentityFromContext() returned a different pointer than was stored")
	}
}

func TestIdentityFromContext_NoIdentityAttached(t *testing.T) {
	got, ok := IdentityFromContext(context.Background())
	if ok {
		t.Errorf("IdentityFromContext() ok = true, want false")
	}
	if got != nil {
		t.Errorf("IdentityFromContext() = %v, want nil", got)
	}
}

func TestIdentityFromContext_WrongValueType(t *testing.T) {
	// Something stored under the same context key but not an *Identity —
	// the type assertion should fail cleanly rather than panic.
	ctx := context.WithValue(context.Background(), identityKey, "not-an-identity")

	got, ok := IdentityFromContext(ctx)
	if ok {
		t.Errorf("IdentityFromContext() ok = true, want false")
	}
	if got != nil {
		t.Errorf("IdentityFromContext() = %v, want nil", got)
	}
}

func TestWithIdentity_NilIdentity(t *testing.T) {
	// Documents current behavior: a stored nil *Identity still satisfies the
	// type assertion, so ok comes back true even though the identity itself
	// is nil. Callers must check for a nil identity, not just ok.
	ctx := WithIdentity(context.Background(), nil)

	got, ok := IdentityFromContext(ctx)
	if !ok {
		t.Fatal("IdentityFromContext() ok = false, want true for a stored nil identity")
	}
	if got != nil {
		t.Errorf("IdentityFromContext() = %v, want nil", got)
	}
}

func TestIdentityFromClient(t *testing.T) {
	cc := config.ClientConfig{
		User:   "alice",
		Groups: []string{"admins", "readers"},
	}

	got := IdentityFromClient("cli-1", cc)

	want := Identity{
		User:   "alice",
		Client: "cli-1",
		Groups: []string{"admins", "readers"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("IdentityFromClient() = %+v, want %+v", got, want)
	}
}

func TestBuildIdentityList(t *testing.T) {
	tests := []struct {
		name    string
		clients map[string]config.ClientConfig
		want    []Identity
	}{
		{name: "nil clients map", clients: nil, want: []Identity{}},
		{name: "empty clients map", clients: map[string]config.ClientConfig{}, want: []Identity{}},
		{
			name: "single client",
			clients: map[string]config.ClientConfig{
				"cli-1": {User: "alice", Groups: []string{"admins"}},
			},
			want: []Identity{
				{User: "alice", Client: "cli-1", Groups: []string{"admins"}},
			},
		},
		{
			name: "multiple clients",
			clients: map[string]config.ClientConfig{
				"cli-1": {User: "alice", Groups: []string{"admins"}},
				"cli-2": {User: "bob", Groups: []string{"readers"}},
			},
			want: []Identity{
				{User: "alice", Client: "cli-1", Groups: []string{"admins"}},
				{User: "bob", Client: "cli-2", Groups: []string{"readers"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{
				Identity: config.IdentityConfig{
					Clients: tt.clients,
				},
			}

			got := BuildIdentityList(cfg)

			sort.Slice(got, func(i, j int) bool { return got[i].Client < got[j].Client })
			sort.Slice(tt.want, func(i, j int) bool { return tt.want[i].Client < tt.want[j].Client })

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildIdentityList() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
