// Unit-style server tests using httptest.
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
