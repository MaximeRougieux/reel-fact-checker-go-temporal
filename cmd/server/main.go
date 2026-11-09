package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"

	"reel-fact-checker-go-temporal/factcheck"
	"reel-fact-checker-go-temporal/internal/config"
)

//go:embed web/index.html
var web embed.FS

var idPattern = regexp.MustCompile(`^factcheck-[A-Za-z0-9_-]{5,20}$`)

type server struct{ c client.Client }

func main() {
	config.LoadDotEnv(".env")
	c, err := client.Dial(client.Options{HostPort: config.Address(), Namespace: config.Namespace()})
	if err != nil {
		log.Fatalln("unable to create client:", err)
	}
	defer c.Close()
	s := &server{c: c}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		b, _ := web.ReadFile("web/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	})
	mux.HandleFunc("POST /api/check", s.start)
	mux.HandleFunc("GET /api/check/{id}", s.status)

	// Localhost only by default: this endpoint triggers paid model and search calls.
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Println("listening on http://" + addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// start returns the cached result when a run for this reel already completed,
// attaches to a run in progress, and otherwise starts a new one.
func (s *server) start(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<14)
	var req struct {
		URL     string `json:"url"`
		Refresh bool   `json:"refresh"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "invalid request")
		return
	}
	canonical, err := factcheck.ParseReelURL(req.URL) // validated before anything else happens
	if err != nil {
		fail(w, http.StatusBadRequest, "not an Instagram reel link")
		return
	}
	id := factcheck.WorkflowID(canonical)

	if !req.Refresh {
		d, err := s.c.DescribeWorkflowExecution(r.Context(), id, "")
		if err == nil && d.WorkflowExecutionInfo.Status == enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED {
			writeJSON(w, http.StatusOK, map[string]any{"workflowId": id, "cached": true})
			return
		}
	}

	reuse := enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY
	if req.Refresh {
		reuse = enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE
	}
	_, err = s.c.ExecuteWorkflow(r.Context(), client.StartWorkflowOptions{
		ID:                       id,
		TaskQueue:                config.TaskQueue(),
		WorkflowIDReusePolicy:    reuse,
		WorkflowIDConflictPolicy: enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING, // concurrent requests share one run
	}, factcheck.FactCheckWorkflow, canonical)
	var already *serviceerror.WorkflowExecutionAlreadyStarted
	switch {
	case errors.As(err, &already): // completed between the check and the start
		writeJSON(w, http.StatusOK, map[string]any{"workflowId": id, "cached": true})
	case err != nil:
		log.Println("start workflow:", err)
		fail(w, http.StatusBadGateway, "could not start the check")
	default:
		writeJSON(w, http.StatusAccepted, map[string]any{"workflowId": id, "cached": false})
	}
}

func (s *server) status(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	d, err := s.c.DescribeWorkflowExecution(ctx, id, "")
	var nf *serviceerror.NotFound
	if errors.As(err, &nf) {
		fail(w, http.StatusNotFound, "no such check")
		return
	}
	if err != nil {
		fail(w, http.StatusBadGateway, "temporal unavailable")
		return
	}
	switch d.WorkflowExecutionInfo.Status {
	case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING:
		stage := "starting"
		if v, err := s.c.QueryWorkflow(ctx, id, "", "stage"); err == nil {
			_ = v.Get(&stage)
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "running", "stage": stage})
	case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:
		var report factcheck.Report
		if err := s.c.GetWorkflow(ctx, id, "").Get(ctx, &report); err != nil {
			fail(w, http.StatusBadGateway, "could not read the result")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":      "completed",
			"report":      report,
			"completedAt": d.WorkflowExecutionInfo.CloseTime.AsTime(),
		})
	default:
		msg := "the check failed"
		var appErr *temporal.ApplicationError
		if err := s.c.GetWorkflow(ctx, id, "").Get(ctx, nil); errors.As(err, &appErr) {
			msg = appErr.Message()
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "failed", "error": msg})
	}
}
