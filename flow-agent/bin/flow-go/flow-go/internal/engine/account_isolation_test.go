package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/kodelyx/flow-go/flow-go/internal/batchexecute"
	"github.com/kodelyx/flow-go/flow-go/internal/cookiejar"
	"github.com/kodelyx/flow-go/flow-go/internal/httpx"
)

func TestConcurrentAccountEnginesKeepCookiesProjectAndAuthuserTogether(t *testing.T) {
	type requestSnapshot struct {
		Cookie   string
		Project  string
		Authuser string
		RPC      string
		Captcha  string
	}
	var mu sync.Mutex
	var requests []requestSnapshot
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var envelope [][][]any
		if err := json.Unmarshal([]byte(r.PostForm.Get("f.req")), &envelope); err != nil || len(envelope) == 0 || len(envelope[0]) == 0 || len(envelope[0][0]) < 2 {
			t.Errorf("decode batchexecute envelope: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		argumentJSON, ok := envelope[0][0][1].(string)
		if !ok {
			t.Errorf("generation argument has type %T, want JSON string", envelope[0][0][1])
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var argument []any
		if err := json.Unmarshal([]byte(argumentJSON), &argument); err != nil || len(argument) < 2 {
			t.Errorf("decode image generation argument: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		request, ok := argument[1].([]any)
		if !ok || len(request) == 0 {
			t.Errorf("generation request has unexpected shape")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		request, ok = request[0].([]any)
		if !ok || len(request) <= 7 {
			t.Errorf("generation request context has unexpected shape")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requestContext, ok := request[7].([]any)
		if !ok || len(requestContext) <= 10 {
			t.Errorf("generation captcha context has unexpected shape")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		captchaPair, ok := requestContext[10].([]any)
		if !ok || len(captchaPair) == 0 {
			t.Errorf("generation captcha pair has unexpected shape")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		captcha, ok := captchaPair[0].(string)
		if !ok {
			t.Errorf("generation captcha has type %T, want string", captchaPair[0])
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		requests = append(requests, requestSnapshot{
			Cookie: r.Header.Get("Cookie"), Project: r.URL.Query().Get("source-path"),
			Authuser: r.URL.Query().Get("authuser"), RPC: r.URL.Query().Get("rpcids"), Captcha: captcha,
		})
		mu.Unlock()
		payload, _ := json.Marshal("null")
		_, _ = fmt.Fprintf(w, ")]}'\n\n[[\"wrb.fr\",%q,%s,null,null,null,\"generic\"]]\n",
			r.URL.Query().Get("rpcids"), payload)
	}))
	defer server.Close()
	previousOrigin := batchexecute.Origin
	batchexecute.Origin = server.URL
	defer func() { batchexecute.Origin = previousOrigin }()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	jarA := cookiejar.FromRaw("SAPISID=sapisid-a; SID=session-a", parsed.Hostname(), "account-a")
	jarB := cookiejar.FromRaw("SAPISID=sapisid-b; SID=session-b", parsed.Hostname(), "account-b")
	engineA := &Engine{primaryJar: jarA, accountIndex: 2}
	engineB := &Engine{primaryJar: jarB, accountIndex: 5}
	clientA, err := httpx.New(httpx.WithQUIC(false))
	if err != nil {
		t.Fatal(err)
	}
	clientB, err := httpx.New(httpx.WithQUIC(false))
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, tc := range []struct {
		engine  *Engine
		http    *httpx.Client
		project string
		prompt  string
		captcha string
	}{{engineA, clientA, "project-a", "prompt for account a", "captcha-a"}, {engineB, clientB, "project-b", "prompt for account b", "captcha-b"}} {
		wg.Add(1)
		go func(tc struct {
			engine  *Engine
			http    *httpx.Client
			project string
			prompt  string
			captcha string
		}) {
			defer wg.Done()
			<-start
			batchexecuteClient := tc.engine.newBatchexecuteClient(tc.engine.Jar(), tc.http)
			_, callErr := batchexecuteClient.Generate(context.Background(), batchexecute.GenerateRequest{
				ProjectID: tc.project, Model: "NARWHAL", Prompt: tc.prompt, CaptchaToken: tc.captcha,
			}, batchexecute.CallOptions{})
			results <- callErr
		}(tc)
	}
	close(start)
	wg.Wait()
	close(results)
	for callErr := range results {
		if callErr != nil {
			t.Fatalf("fake upstream generation call failed: %v", callErr)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("fake upstream received %d requests, want 2", len(requests))
	}
	seen := map[string]bool{}
	for _, request := range requests {
		if request.RPC != batchexecute.RPCIDGenerate {
			t.Errorf("RPC = %q, want image generation RPC", request.RPC)
		}
		switch request.Cookie {
		case "SAPISID=sapisid-a; SID=session-a":
			if request.Project != "/project/project-a" || request.Authuser != "2" || request.Captcha != "captcha-a" {
				t.Errorf("account A request crossed identity: %+v", request)
			}
			seen["a"] = true
		case "SAPISID=sapisid-b; SID=session-b":
			if request.Project != "/project/project-b" || request.Authuser != "5" || request.Captcha != "captcha-b" {
				t.Errorf("account B request crossed identity: %+v", request)
			}
			seen["b"] = true
		default:
			t.Errorf("request carried unknown/mixed cookies: %+v", request)
		}
	}
	if !seen["a"] || !seen["b"] {
		t.Fatalf("fake upstream did not observe both account-bound calls: %v", seen)
	}
}
