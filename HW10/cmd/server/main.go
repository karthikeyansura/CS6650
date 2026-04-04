// Server: replicated in-memory KV service for leader-follower and leaderless modes.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/karthikeyansura/CS6650/HW10/internal/kv"
)

const (
	modeLeaderFollower = "leader-follower"
	modeLeaderless     = "leaderless"

	strategyW5R1 = "W5R1"
	strategyW1R5 = "W1R5"
	strategyW3R3 = "W3R3"
)

// app holds runtime configuration, local state, and HTTP client wiring.
type app struct {
	store     *kv.Store
	client    *http.Client
	mode      string
	strategy  string
	nodeID    string
	selfURL   string
	leader    string
	nodes     []string
	followers []string

	muVersion    sync.Mutex
	lastVersion  int64
	nodeSuffix   int64
	requestCount atomic.Int64
}

// setReq is the client payload for /set.
type setReq struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// replicateReq is the internal replication payload between nodes.
type replicateReq struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

// getResp is the standard value+version response body.
type getResp struct {
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

// main initializes configuration and starts the HTTP server.
func main() {
	a := &app{
		store:    kv.NewStore(),
		client:   &http.Client{Timeout: 3 * time.Second},
		mode:     envOr("MODE", modeLeaderFollower),
		strategy: envOr("STRATEGY", strategyW5R1),
		nodeID:   envOr("NODE_ID", "node"),
		selfURL:  strings.TrimRight(envOr("SELF_URL", ""), "/"),
		leader:   strings.TrimRight(envOr("LEADER_URL", ""), "/"),
		nodes:    splitCSV(envOr("NODES", "")),
	}

	a.nodeSuffix = int64(hashNodeID(a.nodeID))
	if a.selfURL == "" && len(a.nodes) > 0 {
		a.selfURL = strings.TrimRight(a.nodes[0], "/")
	}
	if a.leader == "" {
		a.leader = a.selfURL
	}

	for _, n := range a.nodes {
		nn := strings.TrimRight(n, "/")
		if nn != "" && nn != a.selfURL {
			a.followers = append(a.followers, nn)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", a.health)
	mux.HandleFunc("/set", a.set)
	mux.HandleFunc("/get", a.get)
	mux.HandleFunc("/local_read", a.localRead)
	mux.HandleFunc("/internal/replicate", a.internalReplicate)
	mux.HandleFunc("/internal/read", a.internalRead)
	mux.HandleFunc("/internal/cluster_get", a.internalClusterGet)

	port := envOr("PORT", "8080")
	log.Printf("start node=%s mode=%s strategy=%s self=%s leader=%s nodes=%v", a.nodeID, a.mode, a.strategy, a.selfURL, a.leader, a.nodes)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

// health returns lightweight node metadata for probes and debugging.
func (a *app) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"node":     a.nodeID,
		"mode":     a.mode,
		"strategy": a.strategy,
		"requests": a.requestCount.Load(),
	})
}

// set handles client write requests.
func (a *app) set(w http.ResponseWriter, r *http.Request) {
	a.requestCount.Add(1)

	var req setReq
	if !decodePost(w, r, &req) || !isValidKey(w, req.Key) {
		return
	}

	switch a.mode {
	case modeLeaderFollower:
		if a.selfURL != a.leader {
			a.proxyToLeader(w, r, "/set")
			return
		}
		a.leaderFollowerSet(w, req)
	case modeLeaderless:
		a.leaderlessSet(w, req)
	default:
		http.Error(w, "unknown mode", http.StatusBadRequest)
	}
}

// get handles client read requests.
func (a *app) get(w http.ResponseWriter, r *http.Request) {
	a.requestCount.Add(1)

	key, ok := decodeGet(w, r)
	if !ok {
		return
	}

	switch a.mode {
	case modeLeaderFollower:
		if a.selfURL != a.leader {
			a.proxyToLeader(w, r, "/get?key="+url.QueryEscape(key))
			return
		}
		a.clusterRead(w, key)
	case modeLeaderless:
		rec, ok := a.store.Get(key)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, getResp{Value: rec.Value, Version: rec.Version})
	default:
		http.Error(w, "unknown mode", http.StatusBadRequest)
	}
}

// localRead returns only node-local data and is intended for testing windows.
func (a *app) localRead(w http.ResponseWriter, r *http.Request) {
	key, ok := decodeGet(w, r)
	if !ok {
		return
	}
	rec, ok := a.store.Get(key)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, getResp{Value: rec.Value, Version: rec.Version})
}

// internalReplicate applies follower-side replication updates.
func (a *app) internalReplicate(w http.ResponseWriter, r *http.Request) {
	var req replicateReq
	if !decodePost(w, r, &req) || !isValidKey(w, req.Key) {
		return
	}

	// Assignment delay to widen inconsistency windows during replication.
	time.Sleep(100 * time.Millisecond)
	rec := a.store.SetIfNewer(req.Key, req.Value, req.Version)
	writeJSON(w, http.StatusCreated, rec)
}

// internalRead serves leader-triggered follower reads.
func (a *app) internalRead(w http.ResponseWriter, r *http.Request) {
	key, ok := decodeGet(w, r)
	if !ok {
		return
	}

	if r.Header.Get("X-From-Leader") == "true" {
		// Assignment delay for follower reads triggered by leader coordination.
		time.Sleep(50 * time.Millisecond)
	}

	rec, ok := a.store.Get(key)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, getResp{Value: rec.Value, Version: rec.Version})
}

// internalClusterGet exposes leader-only strategy-aware reads.
func (a *app) internalClusterGet(w http.ResponseWriter, r *http.Request) {
	if a.selfURL != a.leader {
		http.Error(w, "leader only", http.StatusForbidden)
		return
	}

	key, ok := decodeGet(w, r)
	if !ok {
		return
	}

	a.clusterRead(w, key)
}

// leaderFollowerSet coordinates writes according to selected W strategy.
func (a *app) leaderFollowerSet(w http.ResponseWriter, req setReq) {
	ver := a.nextVersion()
	a.store.Set(req.Key, req.Value, ver)
	msg := replicateReq{Key: req.Key, Value: req.Value, Version: ver}

	switch a.strategy {
	case strategyW5R1:
		for _, follower := range a.followers {
			if err := a.replicateTo(follower, msg); err != nil {
				http.Error(w, "replication failed: "+err.Error(), http.StatusBadGateway)
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
	case strategyW1R5:
		go a.replicateEventually(msg)
	case strategyW3R3:
		acks := 1 // Leader write is already committed locally.
		replicated := map[string]bool{}
		for _, follower := range a.followers {
			if err := a.replicateTo(follower, msg); err == nil {
				acks++
				replicated[follower] = true
			}
			time.Sleep(200 * time.Millisecond)
			if acks >= 3 {
				break
			}
		}
		if acks < 3 {
			http.Error(w, "quorum write failed", http.StatusBadGateway)
			return
		}
		go a.replicateRemaining(msg, replicated)
	default:
		http.Error(w, "unknown strategy", http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"key": req.Key, "version": ver})
}

// leaderlessSet coordinates W=N writes from any receiving node.
func (a *app) leaderlessSet(w http.ResponseWriter, req setReq) {
	ver := a.nextVersion()
	a.store.Set(req.Key, req.Value, ver)
	msg := replicateReq{Key: req.Key, Value: req.Value, Version: ver}

	for _, node := range a.nodes {
		n := strings.TrimRight(node, "/")
		if n == "" || n == a.selfURL {
			continue
		}
		if err := a.replicateTo(n, msg); err != nil {
			http.Error(w, "W=N replication failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		// Assignment delay after each replication send.
		time.Sleep(200 * time.Millisecond)
	}

	writeJSON(w, http.StatusCreated, map[string]any{"key": req.Key, "version": ver, "coordinator": a.nodeID})
}

// clusterRead selects read behavior for the configured R strategy.
func (a *app) clusterRead(w http.ResponseWriter, key string) {
	switch a.strategy {
	case strategyW5R1:
		rec, ok := a.store.Get(key)
		if !ok {
			http.NotFound(w, &http.Request{})
			return
		}
		writeJSON(w, http.StatusOK, getResp{Value: rec.Value, Version: rec.Version})
	case strategyW1R5:
		rec, err := a.fetchLatestW1R5(key)
		if err != nil {
			a.respondReadError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, getResp{Value: rec.Value, Version: rec.Version})
	case strategyW3R3:
		nodes := append([]string{a.selfURL}, a.followers...)
		rand.Shuffle(len(nodes), func(i, j int) { nodes[i], nodes[j] = nodes[j], nodes[i] })
		rec, err := a.fetchLatest(key, nodes[:min(3, len(nodes))])
		if err != nil {
			a.respondReadError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, getResp{Value: rec.Value, Version: rec.Version})
	default:
		http.Error(w, "unknown strategy", http.StatusBadRequest)
	}
}

// respondReadError maps internal read errors to HTTP responses.
func (a *app) respondReadError(w http.ResponseWriter, err error) {
	if errors.Is(err, errNotFound) {
		http.NotFound(w, &http.Request{})
		return
	}
	http.Error(w, err.Error(), http.StatusBadGateway)
}

// replicateEventually sends asynchronous background replication.
func (a *app) replicateEventually(req replicateReq) {
	for _, follower := range a.followers {
		if err := a.replicateTo(follower, req); err != nil {
			log.Printf("async replicate follower=%s err=%v", follower, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// replicateRemaining sends async writes only to followers not yet replicated.
func (a *app) replicateRemaining(req replicateReq, already map[string]bool) {
	for _, follower := range a.followers {
		if already[follower] {
			continue
		}
		if err := a.replicateTo(follower, req); err != nil {
			log.Printf("async remaining replicate follower=%s err=%v", follower, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// replicateTo performs one HTTP replication request to a target node.
func (a *app) replicateTo(node string, req replicateReq) error {
	payload, _ := json.Marshal(req)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(node, "/")+"/internal/replicate", bytes.NewReader(payload))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

var errNotFound = errors.New("not found")

// fetchLatestW1R5 reads local leader state and all followers to pick max version.
func (a *app) fetchLatestW1R5(key string) (kv.Record, error) {
	best, found := a.store.Get(key)
	for _, follower := range a.followers {
		rec, ok, err := a.readNode(key, follower)
		if err != nil {
			return kv.Record{}, err
		}
		if !ok {
			continue
		}
		if !found || rec.Version > best.Version {
			best = rec
			found = true
		}
	}
	if !found {
		return kv.Record{}, errNotFound
	}
	return best, nil
}

// fetchLatest reads a specific node set and returns the highest version seen.
func (a *app) fetchLatest(key string, nodes []string) (kv.Record, error) {
	best := kv.Record{}
	found := false
	for _, node := range nodes {
		rec, ok, err := a.readNode(key, node)
		if err != nil {
			return kv.Record{}, err
		}
		if !ok {
			continue
		}
		if !found || rec.Version > best.Version {
			best = rec
			found = true
		}
	}
	if !found {
		return kv.Record{}, errNotFound
	}
	return best, nil
}

// readNode performs one internal read request against a specific node.
func (a *app) readNode(key, node string) (kv.Record, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	endpoint := strings.TrimRight(node, "/") + "/internal/read?key=" + url.QueryEscape(key)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if strings.TrimRight(node, "/") != a.selfURL {
		httpReq.Header.Set("X-From-Leader", "true")
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return kv.Record{}, false, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		return kv.Record{}, false, nil
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return kv.Record{}, false, fmt.Errorf("read status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out getResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return kv.Record{}, false, err
	}
	return kv.Record{Value: out.Value, Version: out.Version}, true, nil
}

// proxyToLeader forwards a client request to the configured leader node.
func (a *app) proxyToLeader(w http.ResponseWriter, r *http.Request, path string) {
	target := strings.TrimRight(a.leader, "/") + path

	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
	}
	proxyReq, _ := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(body))
	proxyReq.Header = r.Header.Clone()

	resp, err := a.client.Do(proxyReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// nextVersion generates monotonic logical versions with node suffix tie-break.
func (a *app) nextVersion() int64 {
	// Version format keeps monotonic ordering per node.
	a.muVersion.Lock()
	defer a.muVersion.Unlock()

	now := (time.Now().UnixNano() << 8) | (a.nodeSuffix & 0xff)
	if now <= a.lastVersion {
		now = a.lastVersion + 1
	}
	a.lastVersion = now
	return now
}

// hashNodeID computes a small deterministic suffix for tie-breaking.
func hashNodeID(id string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return h.Sum32()
}

// writeJSON serializes a value to JSON with status code and content type.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// splitCSV parses comma-separated node URLs.
func splitCSV(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// envOr returns an environment variable or fallback default.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ---------------------------------------------------------
// REFACTOR HELPERS TO ELIMINATE DUPLICATION
// ---------------------------------------------------------

// decodePost validates that the request is a POST and decodes the JSON body.
func decodePost(w http.ResponseWriter, r *http.Request, payload any) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	if err := json.NewDecoder(r.Body).Decode(payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return false
	}
	return true
}

// isValidKey ensures the provided key string is not empty.
func isValidKey(w http.ResponseWriter, key string) bool {
	if strings.TrimSpace(key) == "" {
		http.Error(w, "key cannot be empty", http.StatusBadRequest)
		return false
	}
	return true
}

// decodeGet validates that the request is a GET and extracts the required 'key' query parameter.
func decodeGet(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return "", false
	}
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	if key == "" {
		http.Error(w, "key cannot be empty", http.StatusBadRequest)
		return "", false
	}
	return key, true
}
