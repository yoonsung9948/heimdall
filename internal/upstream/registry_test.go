package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolsPerServerCopies(t *testing.T) {
	tc1 := make(map[string][]*mcp.Tool)
	tc1["server1"] = []*mcp.Tool{
		{
			Description: "fake tool",
		},
	}
	r1 := Registry{
		routeMap:  nil,
		clientMap: nil,
		toolCache: tc1,
	}
	t.Run("ToolsPerServer should return a copy", func(t *testing.T) {
		out := r1.ToolsPerServer(context.TODO())
		if reflect.ValueOf(out).Pointer() == reflect.ValueOf(tc1).Pointer() {
			t.Errorf("map instance not a copy")
		}
		for serverName, tools := range r1.toolCache {
			copiedTools := out[serverName]
			if reflect.ValueOf(copiedTools).Pointer() == reflect.ValueOf(tools).Pointer() {
				t.Errorf("tools list not a copy")
			}
		}
	})
}

func TestRegister(t *testing.T) {
	client := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search documents"},
		},
	}

	tests := []struct {
		name        string
		wantRoutes  map[string]string
		wantTools   map[string][]*mcp.Tool
		wantClients map[string]UpstreamClient
		serverName  string
		client      UpstreamClient

		wantErr error
	}{
		{
			name:        "empty server name errors",
			serverName:  "",
			client:      client,
			wantErr:     ErrEmptyServerName,
			wantRoutes:  map[string]string{},
			wantTools:   map[string][]*mcp.Tool{},
			wantClients: map[string]UpstreamClient{},
		},
		{
			name:        "nil client errors",
			serverName:  "docs",
			client:      nil,
			wantErr:     ErrMissingUpstreamClient,
			wantRoutes:  map[string]string{},
			wantTools:   map[string][]*mcp.Tool{},
			wantClients: map[string]UpstreamClient{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			err := r.Register(context.Background(), tt.serverName, tt.client)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Register() error = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(r.routeMap, tt.wantRoutes) {
				t.Errorf("routes = %#v, want %#v", r.routeMap, tt.wantRoutes)
			}
			if !reflect.DeepEqual(r.toolCache, tt.wantTools) {
				t.Errorf("tools = %#v, want %#v", r.toolCache, tt.wantTools)
			}
			if !reflect.DeepEqual(r.clientMap, tt.wantClients) {
				t.Errorf("clients = %#v, want %#v", r.clientMap, tt.wantClients)
			}
		})
	}
}

func TestRegister_FetchFailureAllowsRetry(t *testing.T) {
	fetchErr := errors.New("fetch failed")
	client := &fakeClient{err: fetchErr}
	r := NewRegistry()

	err := r.Register(context.Background(), "docs", client)

	if !errors.Is(err, fetchErr) {
		t.Fatalf("Register() error = %v, want wrapped %v", err, fetchErr)
	}

	if len(r.routeMap) != 0 ||
		len(r.toolCache) != 0 ||
		len(r.clientMap) != 0 {
		t.Fatal("failed registration left registry state behind")
	}

	client.err = nil
	client.tools = []*mcp.Tool{{Name: "search"}}

	if err := r.Register(context.Background(), "docs", client); err != nil {
		t.Fatalf("retry failed: %v", err)
	}

	got, serverName, err := r.ClientForTool(context.Background(), "docs.search")
	if err != nil {
		t.Fatalf("lookup after retry failed: %v", err)
	}
	if got != client || serverName != "docs" {
		t.Error("retry did not register the expected client and route")
	}
}
func TestRegister_DuplicateServer(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry()

	original := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search documents"},
		},
	}
	duplicate := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "delete", Description: "Delete documents"},
		},
	}

	if err := r.Register(ctx, "docs", original); err != nil {
		t.Fatalf("initial registration failed: %v", err)
	}

	err := r.Register(ctx, "docs", duplicate)
	if !errors.Is(err, ErrDuplicateServer) {
		t.Errorf("Register() error = %v, want %v", err, ErrDuplicateServer)
	}

	got, serverName, err := r.ClientForTool(ctx, "docs.search")
	if err != nil {
		t.Fatalf("original tool lookup failed: %v", err)
	}
	if got != original || serverName != "docs" {
		t.Error("duplicate registration changed the original route or client")
	}

	_, _, err = r.ClientForTool(ctx, "docs.delete")
	if !errors.Is(err, ErrMissingTool) {
		t.Errorf("duplicate tool lookup error = %v, want %v", err, ErrMissingTool)
	}

	wantTools := map[string][]*mcp.Tool{
		"docs": {
			{Name: "docs.search", Description: "Search documents"},
		},
	}
	if !reflect.DeepEqual(r.toolCache, wantTools) {
		t.Errorf("tool cache = %#v, want %#v", r.toolCache, wantTools)
	}
}

func TestRegister_SuccessfulRegistration(t *testing.T) {
	r := NewRegistry()
	client := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search documents"},
		}}
	err := r.Register(context.Background(), "docs", client)

	if err != nil {
		t.Fatalf("initial registration failed: %v", err)
	}
	c, serverName, err := r.ClientForTool(context.Background(), "docs.search")
	if err != nil {
		t.Fatalf("ClientForTool() unexpected error: %v", err)
	}
	if serverName != "docs" {
		t.Errorf("server name = %q, want %q", serverName, "docs")
	}
	if c != client {
		t.Error("ClientForTool() returned a different client instance")
	}
}
func TestRegister_ToolPrefix(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry()

	testTools := []*mcp.Tool{
		{Name: "search", Description: "Search documents"},
	}
	client := &fakeClient{tools: testTools}

	if err := r.Register(ctx, "docs", client); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}

	gotClient, serverName, err := r.ClientForTool(ctx, "docs.search")
	if err != nil {
		t.Fatalf("ClientForTool() unexpected error: %v", err)
	}
	if serverName != "docs" {
		t.Errorf("server name = %q, want %q", serverName, "docs")
	}
	if gotClient != client {
		t.Error("ClientForTool() returned a different client instance")
	}

	// Registration must leave the original tool unchanged.
	wantOriginal := []*mcp.Tool{
		{Name: "search", Description: "Search documents"},
	}
	if !reflect.DeepEqual(testTools, wantOriginal) {
		t.Errorf("original tools = %#v, want %#v", testTools, wantOriginal)
	}

	// The registry advertises a prefixed copy, preserving its description.
	tools := r.AllTools(ctx)

	wantPrefixed := []*mcp.Tool{
		{Name: "docs.search", Description: "Search documents"},
	}
	if !reflect.DeepEqual(tools, wantPrefixed) {
		t.Errorf("registered tools = %#v, want %#v", tools, wantPrefixed)
	}
}

func TestRegister_SameToolNameAcrossServers(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()
	fakeClient1 := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search documents in local database"},
		},
	}
	fakeClient2 := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search documents on github"},
		},
	}
	err := r.Register(ctx, "db", fakeClient1)
	if err != nil {
		t.Fatalf("register db: %v", err)
	}
	err = r.Register(ctx, "github", fakeClient2)
	if err != nil {
		t.Fatalf("register github: %v", err)
	}

	tools := r.AllTools(ctx)

	slices.SortFunc(tools, func(a, b *mcp.Tool) int {
		return strings.Compare(a.Name, b.Name)
	})
	wantPrefixed := []*mcp.Tool{
		{Name: "db.search", Description: "Search documents in local database"},
		{Name: "github.search", Description: "Search documents on github"},
	}
	if !reflect.DeepEqual(tools, wantPrefixed) {
		t.Errorf("registered tools = %#v, want %#v", tools, wantPrefixed)
	}

	c, s, err := r.ClientForTool(ctx, "db.search")
	if err != nil {
		t.Fatalf("lookup db.search: %v", err)
	}
	if c != fakeClient1 || s != "db" {
		t.Error("db.search did not route to the database client")
	}

	c, s, err = r.ClientForTool(ctx, "github.search")
	if err != nil {
		t.Fatalf("lookup github.search: %v", err)
	}
	if c != fakeClient2 || s != "github" {
		t.Error("github.search did not route to the GitHub client")
	}
}

func TestRegister_NoTools(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	original := &fakeClient{tools: []*mcp.Tool{}}
	if err := r.Register(ctx, "db", original); err != nil {
		t.Fatalf("register db: %v", err)
	}

	duplicate := &fakeClient{tools: []*mcp.Tool{}}
	if err := r.Register(ctx, "db", duplicate); !errors.Is(err, ErrDuplicateServer) {
		t.Errorf("duplicate registration error = %v, want %v", err, ErrDuplicateServer)
	}

	if got := r.clientMap["db"]; got != original {
		t.Error("registry did not preserve the original client")
	}

	tools := r.AllTools(ctx)

	if len(tools) != 0 {
		t.Errorf("tool count = %d, want 0", len(tools))
	}
	if len(r.routeMap) != 0 {
		t.Errorf("route count = %d, want 0", len(r.routeMap))
	}
}

func TestRegister_ConcurrentSameServer(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	fakeClient1 := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search from client one"},
		},
	}
	fakeClient2 := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search from client two"},
		},
	}

	var wg sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})
	clients := make(chan UpstreamClient, 2)

	for _, client := range []UpstreamClient{fakeClient1, fakeClient2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := r.Register(ctx, "github", client)
			results <- err
			if err == nil {
				clients <- client
			}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(clients)

	var successes, duplicates int

	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrDuplicateServer):
			duplicates++
		default:
			t.Errorf("unexpected registration error: %v", err)
		}
	}

	if successes != 1 || duplicates != 1 {
		t.Errorf("got %d successes and %d duplicates, want 1 each",
			successes, duplicates)
	}
	c, _, err := r.ClientForTool(ctx, "github.search")
	if err != nil {
		t.Fatalf("lookup github.search: %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("got %d winners, want 1", len(clients))
	}
	winner := <-clients

	if c != winner {
		t.Error("registered client differs from the successful registration")
	}

	wantDescription := "Search from client one"
	if winner == fakeClient2 {
		wantDescription = "Search from client two"
	}
	wantTools := []*mcp.Tool{
		{Name: "github.search", Description: wantDescription},
	}
	if got := r.AllTools(ctx); !reflect.DeepEqual(got, wantTools) {
		t.Errorf("registered tools = %#v, want winner's tools %#v", got, wantTools)
	}
}
func TestRegister_ConcurrentDifferentServers(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	dbClient := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search database documents"},
		},
	}
	githubClient := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search GitHub documents"},
		},
	}

	registrations := []struct {
		serverName string
		client     UpstreamClient
	}{
		{"db", dbClient},
		{"github", githubClient},
	}

	type registerResult struct {
		serverName string
		err        error
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan registerResult, len(registrations))

	for _, registration := range registrations {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- registerResult{
				serverName: registration.serverName,
				err:        r.Register(ctx, registration.serverName, registration.client),
			}
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	for result := range results {
		if result.err != nil {
			t.Errorf("register %q: %v", result.serverName, result.err)
		}
	}

	for _, registration := range registrations {
		toolName := registration.serverName + ".search"
		client, serverName, err := r.ClientForTool(ctx, toolName)
		if err != nil {
			t.Errorf("lookup %q: %v", toolName, err)
			continue
		}
		if client != registration.client || serverName != registration.serverName {
			t.Errorf("lookup %q returned the wrong client or server", toolName)
		}
	}

	tools := r.AllTools(ctx)
	slices.SortFunc(tools, func(a, b *mcp.Tool) int {
		return strings.Compare(a.Name, b.Name)
	})

	want := []*mcp.Tool{
		{Name: "db.search", Description: "Search database documents"},
		{Name: "github.search", Description: "Search GitHub documents"},
	}
	if !reflect.DeepEqual(tools, want) {
		t.Errorf("registered tools = %#v, want %#v", tools, want)
	}
}

func TestRegister_ConcurrentReads(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	client := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search GitHub documents"},
		},
	}
	wantTools := []*mcp.Tool{
		{Name: "github.search", Description: "Search GitHub documents"},
	}

	const readerCount = 4
	const readsPerReader = 100

	var wg sync.WaitGroup
	start := make(chan struct{})
	registerResult := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		registerResult <- r.Register(ctx, "github", client)
	}()

	for i := 0; i < readerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			for j := 0; j < readsPerReader; j++ {
				tools := r.AllTools(ctx)

				// read can observe the registry before or after registration
				if len(tools) != 0 && !reflect.DeepEqual(tools, wantTools) {
					t.Errorf("unexpected advertised tools: %#v", tools)
					return
				}

				gotClient, serverName, err := r.ClientForTool(ctx, "github.search")
				if errors.Is(err, ErrMissingTool) {
					// registration cannot disappear after tools are advertised
					if len(tools) != 0 {
						t.Error("advertised tool has no route")
						return
					}
					continue
				}
				if err != nil {
					t.Errorf("lookup github.search: %v", err)
					return
				}
				if gotClient != client || serverName != "github" {
					t.Error("lookup returned the wrong client or server")
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()

	if err := <-registerResult; err != nil {
		t.Fatalf("register github: %v", err)
	}

	if tools := r.AllTools(ctx); !reflect.DeepEqual(tools, wantTools) {
		t.Errorf("final tools = %#v, want %#v", tools, wantTools)
	}
	gotClient, serverName, err := r.ClientForTool(ctx, "github.search")
	if err != nil {
		t.Fatalf("final lookup github.search: %v", err)
	}
	if gotClient != client || serverName != "github" {
		t.Error("final lookup returned the wrong client or server")
	}
}

func TestClientForTool_UnknownTool(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	client := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search documents"},
		},
	}
	if err := r.Register(ctx, "docs", client); err != nil {
		t.Fatalf("register docs: %v", err)
	}

	gotClient, serverName, err := r.ClientForTool(ctx, "docs.delete")
	if !errors.Is(err, ErrMissingTool) {
		t.Errorf("ClientForTool() error = %v, want %v", err, ErrMissingTool)
	}
	if gotClient != nil {
		t.Error("ClientForTool() returned a non-nil client for an unknown tool")
	}
	if serverName != "" {
		t.Errorf("server name = %q, want empty string", serverName)
	}
}

func TestToolsPerServer_ReturnsIndependentMapAndSlices(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	client := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search documents"},
		},
	}
	if err := r.Register(ctx, "docs", client); err != nil {
		t.Fatalf("register docs: %v", err)
	}

	result := r.ToolsPerServer(ctx)
	if len(result["docs"]) != 1 {
		t.Fatalf("docs tool count = %d, want 1", len(result["docs"]))
	}
	result["docs"][0] = &mcp.Tool{Name: "changed"}
	delete(result, "docs")

	newResult := r.ToolsPerServer(ctx)
	want := map[string][]*mcp.Tool{
		"docs": {
			{Name: "docs.search", Description: "Search documents"},
		},
	}
	if !reflect.DeepEqual(newResult, want) {
		t.Errorf("ToolsPerServer() = %#v, want %#v", newResult, want)
	}

}

func TestBuildSnapshot_PreservesEntriesAndSortsByToolName(t *testing.T) {
	toolsByServer := map[string][]*mcp.Tool{
		"docs": {
			{Name: "docs.search"},
			{Name: "docs.delete"},
		},
		"github": {
			{Name: "github.search"},
		},
	}
	capturedAt := time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC)

	snapshot := BuildSnapshot(toolsByServer, capturedAt)
	want := []ToolEntry{
		{ToolName: "docs.delete", ServerName: "docs"},
		{ToolName: "docs.search", ServerName: "docs"},
		{ToolName: "github.search", ServerName: "github"},
	}

	if !reflect.DeepEqual(snapshot.Tools, want) {
		t.Errorf("BuildSnapshot() tools = %#v, want %#v", snapshot.Tools, want)
	}
	if !snapshot.CapturedAt.Equal(capturedAt) {
		t.Errorf("CapturedAt = %v, want %v", snapshot.CapturedAt, capturedAt)
	}
}
func TestBuildSnapshot_EmptyInput(t *testing.T) {
	capturedAt := time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input map[string][]*mcp.Tool
	}{
		{name: "nil map", input: nil},
		{name: "empty map", input: map[string][]*mcp.Tool{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := BuildSnapshot(tt.input, capturedAt)

			if len(snapshot.Tools) != 0 {
				t.Errorf("tool count = %d, want 0", len(snapshot.Tools))
			}
			if !snapshot.CapturedAt.Equal(capturedAt) {
				t.Errorf("CapturedAt = %v, want %v", snapshot.CapturedAt, capturedAt)
			}
		})
	}
}

func TestToolsFromSnapshot_GroupsToolsByServer(t *testing.T) {
	snapshot := Snapshot{
		Tools: []ToolEntry{
			{ToolName: "docs.search", ServerName: "docs"},
			{ToolName: "github.search", ServerName: "github"},
			{ToolName: "docs.delete", ServerName: "docs"},
		},
	}
	toolsByServer := ToolsFromSnapshot(snapshot)

	want := map[string][]*mcp.Tool{
		"docs": {
			{Name: "docs.search"},
			{Name: "docs.delete"},
		},
		"github": {
			{Name: "github.search"},
		},
	}

	if !reflect.DeepEqual(toolsByServer, want) {
		t.Errorf("ToolsFromSnapshot() = %#v, want %#v", toolsByServer, want)
	}
}
func TestToolsFromSnapshot_EmptyInput(t *testing.T) {
	got := ToolsFromSnapshot(Snapshot{})
	if len(got) != 0 {
		t.Errorf("server count = %d, want 0", len(got))
	}
}

func TestSnapshotFile_RoundTrip(t *testing.T) {
	toolsByServer := map[string][]*mcp.Tool{
		"docs": {
			{Name: "docs.search"},
			{Name: "docs.delete"},
		},
		"github": {
			{Name: "github.search"},
		},
	}
	capturedAt := time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC)

	snapshot := BuildSnapshot(toolsByServer, capturedAt)

	path := filepath.Join(t.TempDir(), "snapshots.json")

	if err := WriteFile(snapshot, path); err != nil {
		t.Fatalf("WriteFile() unexpected error: %v", err)
	}

	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got.Tools, snapshot.Tools) {
		t.Errorf("round-trip tools = %#v, want %#v", got.Tools, snapshot.Tools)
	}
	if !got.CapturedAt.Equal(snapshot.CapturedAt) {
		t.Errorf("round-trip timestamp = %v, want %v",
			got.CapturedAt, snapshot.CapturedAt)
	}
}

func TestReadFile_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")

	got, err := ReadFile(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("ReadFile() error = %v, want %v", err, os.ErrNotExist)
	}
	if !reflect.DeepEqual(got, Snapshot{}) {
		t.Errorf("ReadFile() snapshot = %#v, want zero-value Snapshot", got)
	}
}

func TestReadFile_MalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	input := `{"Tools": [`

	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatalf("write test snapshot: %v", err)
	}

	got, err := ReadFile(path)

	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Errorf("ReadFile() error = %v, want *json.SyntaxError", err)
	}
	if !reflect.DeepEqual(got, Snapshot{}) {
		t.Errorf("ReadFile() snapshot = %#v, want zero-value Snapshot", got)
	}
}

func TestWriteFile_MissingParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "snapshot.json")
	snapshot := Snapshot{
		CapturedAt: time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC),
		Tools: []ToolEntry{
			{ToolName: "docs.search", ServerName: "docs"},
		},
	}

	err := WriteFile(snapshot, path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("WriteFile() error = %v, want %v", err, os.ErrNotExist)
	}
}
func TestAllTools_MutatingReturnedToolDoesNotAffectRegistry(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	client := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search documents"},
		},
	}
	if err := r.Register(ctx, "docs", client); err != nil {
		t.Fatalf("register docs: %v", err)
	}

	got := r.AllTools(ctx)
	if len(got) != 1 {
		t.Fatalf("tool count = %d, want 1", len(got))
	}

	// A caller downstream (formatting, redaction, whatever) mutates the tool
	// object it was handed. The registry must not see this.
	got[0].Description = "tampered"

	again := r.AllTools(ctx)
	if len(again) != 1 {
		t.Fatalf("tool count = %d, want 1", len(again))
	}
	if again[0].Description != "Search documents" {
		t.Errorf("AllTools() leaked a caller mutation into registry state: got %q, want %q",
			again[0].Description, "Search documents")
	}
}

func TestToolsPerServer_MutatingToolDoesNotAffectRegistry(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	client := &fakeClient{
		tools: []*mcp.Tool{
			{Name: "search", Description: "Search documents"},
		},
	}
	if err := r.Register(ctx, "docs", client); err != nil {
		t.Fatalf("register docs: %v", err)
	}

	got := r.ToolsPerServer(ctx)
	got["docs"][0].Description = "tampered"

	again := r.ToolsPerServer(ctx)
	if again["docs"][0].Description != "Search documents" {
		t.Errorf("ToolsPerServer() leaked a caller mutation into registry state: got %q, want %q",
			again["docs"][0].Description, "Search documents")
	}
}

type fakeClient struct {
	tools []*mcp.Tool
	err   error
}

func (f *fakeClient) Tools(context.Context) ([]*mcp.Tool, error) {
	return f.tools, f.err
}

func (f *fakeClient) Call(
	context.Context,
	*mcp.CallToolParams,
) (*mcp.CallToolResult, error) {
	panic("unexpected Call: Register should only fetch tools")
}
