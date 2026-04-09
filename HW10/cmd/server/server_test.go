// Unit tests using httptest.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/karthikeyansura/CS6650/HW10/internal/kv"
)

// newTestApp creates a minimal app instance for in-memory handler tests.
func newTestApp(mode, strategy string) *app {
	return &app{
		store:      kv.NewStore(),
		client:     &http.Client{Timeout: 2 * time.Second},
		mode:       mode,
		strategy:   strategy,
		nodeID:     "test",
		selfURL:    "http://self",
		leader:     "http://self",
		nodeSuffix: 1,
	}
}

// TestLeaderConsistencyAfterAck verifies follower consistency after leader ACK.
func TestLeaderConsistencyAfterAck(t *testing.T) {
	leader := newTestApp(modeLeaderFollower, strategyW5R1)

	followers := make([]*app, 4)
	servers := make([]*httptest.Server, 0, 5)
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	for i := 0; i < 4; i++ {
		followers[i] = newTestApp(modeLeaderFollower, strategyW5R1)
		mux := http.NewServeMux()
		mux.HandleFunc("/internal/replicate", followers[i].internalReplicate)
		mux.HandleFunc("/internal/read", followers[i].internalRead)
		mux.HandleFunc("/local_read", followers[i].localRead)
		srv := httptest.NewServer(mux)
		servers = append(servers, srv)
	}

	leaderMux := http.NewServeMux()
	leaderMux.HandleFunc("/set", leader.set)
	leaderMux.HandleFunc("/get", leader.get)
	leaderMux.HandleFunc("/internal/cluster_get", leader.internalClusterGet)
	leaderSrv := httptest.NewServer(leaderMux)
	servers = append(servers, leaderSrv)

	leader.selfURL = leaderSrv.URL
	leader.leader = leaderSrv.URL
	leader.nodes = []string{leaderSrv.URL}
	for _, s := range servers[:4] {
		leader.followers = append(leader.followers, s.URL)
		leader.nodes = append(leader.nodes, s.URL)
	}

	payload, _ := json.Marshal(setReq{Key: "k1", Value: "v1"})
	res, err := http.Post(leaderSrv.URL+"/set", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("set request failed: %v", err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 from leader set, got %d", res.StatusCode)
	}

	gRes, err := http.Get(leaderSrv.URL + "/get?key=k1")
	if err != nil {
		t.Fatalf("leader get failed: %v", err)
	}
	if gRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from leader get, got %d", gRes.StatusCode)
	}

	fRes, err := http.Get(servers[0].URL + "/local_read?key=k1")
	if err != nil {
		t.Fatalf("follower local_read failed: %v", err)
	}
	if fRes.StatusCode != http.StatusOK {
		t.Fatalf("expected follower to be consistent after ack, got %d", fRes.StatusCode)
	}
}

// Health endpoint

func TestHealthEndpoint(t *testing.T) {
	a := newTestApp(modeLeaderFollower, strategyW5R1)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	a.health(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status 'ok', got %v", body["status"])
	}
	if body["node"] != "test" {
		t.Fatalf("expected node 'test', got %v", body["node"])
	}
}

// API contract tests

func TestSetEmptyKeyReturns400(t *testing.T) {
	a := newTestApp(modeLeaderFollower, strategyW5R1)
	req := httptest.NewRequest(http.MethodPost, "/set",
		strings.NewReader(`{"key":"","value":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.set(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty key, got %d", rec.Code)
	}
}

func TestGetEmptyKeyReturns400(t *testing.T) {
	a := newTestApp(modeLeaderFollower, strategyW5R1)
	req := httptest.NewRequest(http.MethodGet, "/get", nil)
	rec := httptest.NewRecorder()
	a.get(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing key param, got %d", rec.Code)
	}
}

func TestSetWrongMethodReturns405(t *testing.T) {
	a := newTestApp(modeLeaderFollower, strategyW5R1)
	req := httptest.NewRequest(http.MethodGet, "/set", nil)
	rec := httptest.NewRecorder()
	a.set(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET on /set, got %d", rec.Code)
	}
}

func TestGetWrongMethodReturns405(t *testing.T) {
	a := newTestApp(modeLeaderFollower, strategyW5R1)
	req := httptest.NewRequest(http.MethodPost, "/get?key=k",
		strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	a.get(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST on /get, got %d", rec.Code)
	}
}

func TestSetMalformedJSONReturns400(t *testing.T) {
	a := newTestApp(modeLeaderFollower, strategyW5R1)
	req := httptest.NewRequest(http.MethodPost, "/set",
		strings.NewReader("not json at all"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.set(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed json, got %d", rec.Code)
	}
}

func TestGetMissingKeyReturns404(t *testing.T) {
	a := newTestApp(modeLeaderFollower, strategyW5R1)
	req := httptest.NewRequest(http.MethodGet, "/get?key=nonexistent", nil)
	rec := httptest.NewRecorder()
	a.get(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent key, got %d", rec.Code)
	}
}

func TestSetEmptyValueAllowed(t *testing.T) {
	a := newTestApp(modeLeaderFollower, strategyW5R1)
	req := httptest.NewRequest(http.MethodPost, "/set",
		strings.NewReader(`{"key":"k","value":""}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.set(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for empty value (valid per assignment), got %d", rec.Code)
	}
}

func TestLocalReadMissingReturns404(t *testing.T) {
	a := newTestApp(modeLeaderFollower, strategyW5R1)
	req := httptest.NewRequest(http.MethodGet, "/local_read?key=missing", nil)
	rec := httptest.NewRecorder()
	a.localRead(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// Strategy-specific: W5R1 failure path

func TestW5R1FailsOnFollowerError(t *testing.T) {
	leader := newTestApp(modeLeaderFollower, strategyW5R1)

	brokenMux := http.NewServeMux()
	brokenMux.HandleFunc("/internal/replicate", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "simulated failure", http.StatusInternalServerError)
	})
	brokenSrv := httptest.NewServer(brokenMux)
	defer brokenSrv.Close()

	leader.followers = []string{brokenSrv.URL}

	req := httptest.NewRequest(http.MethodPost, "/set",
		strings.NewReader(`{"key":"k1","value":"v1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	leader.set(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when follower replication fails, got %d", rec.Code)
	}
}

// Strategy-specific: W1R5

func TestW1R5WriteAcksQuickly(t *testing.T) {
	leader := newTestApp(modeLeaderFollower, strategyW1R5)

	follower := newTestApp(modeLeaderFollower, strategyW1R5)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/replicate", follower.internalReplicate)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	leader.followers = []string{srv.URL}

	req := httptest.NewRequest(http.MethodPost, "/set",
		strings.NewReader(`{"key":"fast","value":"v1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	start := time.Now()
	leader.set(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("W1R5 write should return near-instantly, took %v", elapsed)
	}
}

func TestW1R5ReadReturnsHighestVersion(t *testing.T) {
	leader := newTestApp(modeLeaderFollower, strategyW1R5)

	followers := make([]*app, 2)
	servers := make([]*httptest.Server, 0, 3)
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	for i := 0; i < 2; i++ {
		followers[i] = newTestApp(modeLeaderFollower, strategyW1R5)
		mux := http.NewServeMux()
		mux.HandleFunc("/internal/read", followers[i].internalRead)
		srv := httptest.NewServer(mux)
		servers = append(servers, srv)
	}

	leaderMux := http.NewServeMux()
	leaderMux.HandleFunc("/get", leader.get)
	leaderSrv := httptest.NewServer(leaderMux)
	servers = append(servers, leaderSrv)

	leader.selfURL = leaderSrv.URL
	leader.leader = leaderSrv.URL
	for _, s := range servers[:2] {
		leader.followers = append(leader.followers, s.URL)
	}

	leader.store.Set("k", "leader-val", 10)
	followers[0].store.Set("k", "follower-newer", 20)

	gRes, err := http.Get(leaderSrv.URL + "/get?key=k")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if gRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", gRes.StatusCode)
	}
	var resp getResp
	if err := json.NewDecoder(gRes.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Version != 20 {
		t.Fatalf("expected version 20 from follower, got %d", resp.Version)
	}
	if resp.Value != "follower-newer" {
		t.Fatalf("expected value 'follower-newer', got %q", resp.Value)
	}
}

// Strategy-specific: W3R3

func TestW3R3AcksAfterQuorum(t *testing.T) {
	leader := newTestApp(modeLeaderFollower, strategyW3R3)

	servers := make([]*httptest.Server, 0, 4)
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	for i := 0; i < 2; i++ {
		f := newTestApp(modeLeaderFollower, strategyW3R3)
		mux := http.NewServeMux()
		mux.HandleFunc("/internal/replicate", f.internalReplicate)
		srv := httptest.NewServer(mux)
		servers = append(servers, srv)
	}

	for i := 0; i < 2; i++ {
		mux := http.NewServeMux()
		mux.HandleFunc("/internal/replicate", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "down", http.StatusInternalServerError)
		})
		srv := httptest.NewServer(mux)
		servers = append(servers, srv)
	}

	leader.followers = []string{
		servers[0].URL, servers[1].URL,
		servers[2].URL, servers[3].URL,
	}

	req := httptest.NewRequest(http.MethodPost, "/set",
		strings.NewReader(`{"key":"q","value":"quorum"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	leader.set(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 with quorum (1 leader + 2 healthy = 3), got %d", rec.Code)
	}
}

func TestW3R3FailsWhenQuorumUnreachable(t *testing.T) {
	leader := newTestApp(modeLeaderFollower, strategyW3R3)

	servers := make([]*httptest.Server, 0, 4)
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	for i := 0; i < 4; i++ {
		mux := http.NewServeMux()
		mux.HandleFunc("/internal/replicate", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "down", http.StatusInternalServerError)
		})
		srv := httptest.NewServer(mux)
		servers = append(servers, srv)
	}

	leader.followers = []string{
		servers[0].URL, servers[1].URL,
		servers[2].URL, servers[3].URL,
	}

	req := httptest.NewRequest(http.MethodPost, "/set",
		strings.NewReader(`{"key":"q","value":"v"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	leader.set(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 without quorum (acks=1 < 3), got %d", rec.Code)
	}
}

// Leaderless guarantees

func TestLeaderlessWriteReplicatesAllAndReturnsCoordinator(t *testing.T) {
	const n = 3
	apps := make([]*app, n)
	srvs := make([]*httptest.Server, n)
	defer func() {
		for _, s := range srvs {
			s.Close()
		}
	}()

	for i := 0; i < n; i++ {
		apps[i] = &app{
			store:      kv.NewStore(),
			client:     &http.Client{Timeout: 2 * time.Second},
			mode:       modeLeaderless,
			nodeID:     fmt.Sprintf("ll%d", i+1),
			nodeSuffix: int64(i + 1),
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/set", apps[i].set)
		mux.HandleFunc("/get", apps[i].get)
		mux.HandleFunc("/internal/replicate", apps[i].internalReplicate)
		srvs[i] = httptest.NewServer(mux)
	}

	allNodes := make([]string, n)
	for i := 0; i < n; i++ {
		allNodes[i] = srvs[i].URL
	}
	for i := 0; i < n; i++ {
		apps[i].selfURL = srvs[i].URL
		apps[i].nodes = allNodes
		for j := 0; j < n; j++ {
			if i != j {
				apps[i].followers = append(apps[i].followers, srvs[j].URL)
			}
		}
	}

	payload, _ := json.Marshal(setReq{Key: "ll-key", Value: "ll-val"})
	res, err := http.Post(srvs[0].URL+"/set", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", res.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	coord, _ := body["coordinator"].(string)
	if coord != "ll1" {
		t.Fatalf("expected coordinator 'll1', got %q", coord)
	}
	if body["version"] == nil {
		t.Fatal("expected version in response")
	}
	if k, _ := body["key"].(string); k != "ll-key" {
		t.Fatalf("expected key 'll-key', got %q", k)
	}

	for i := 0; i < n; i++ {
		gRes, err := http.Get(srvs[i].URL + "/get?key=ll-key")
		if err != nil {
			t.Fatalf("get from node %d failed: %v", i, err)
		}
		if gRes.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 from node %d after W=N write, got %d", i, gRes.StatusCode)
		}
		var resp getResp
		if err := json.NewDecoder(gRes.Body).Decode(&resp); err != nil {
			t.Fatalf("decode from node %d failed: %v", i, err)
		}
		if resp.Value != "ll-val" {
			t.Fatalf("node %d: expected 'll-val', got %q", i, resp.Value)
		}
	}
}

// Version generation integrity

func TestNextVersionMonotonicity(t *testing.T) {
	a := newTestApp(modeLeaderFollower, strategyW5R1)
	prev := int64(0)
	for i := 0; i < 1000; i++ {
		v := a.nextVersion()
		if v <= prev {
			t.Fatalf("version %d not greater than previous %d at iteration %d", v, prev, i)
		}
		prev = v
	}
}

func TestNextVersionConcurrentUniqueness(t *testing.T) {
	a := newTestApp(modeLeaderFollower, strategyW5R1)
	const goroutines = 10
	const perGoroutine = 100
	ch := make(chan int64, goroutines*perGoroutine)
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				ch <- a.nextVersion()
			}
		}()
	}
	wg.Wait()
	close(ch)

	seen := make(map[int64]bool)
	for v := range ch {
		if seen[v] {
			t.Fatalf("duplicate version detected: %d", v)
		}
		seen[v] = true
	}
	if len(seen) != goroutines*perGoroutine {
		t.Fatalf("expected %d unique versions, got %d", goroutines*perGoroutine, len(seen))
	}
}
