package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.temporal.io/sdk/client"

	"reel-fact-checker-go-temporal/factcheck"
	"reel-fact-checker-go-temporal/internal/config"
)

func main() {
	question := "https://www.instagram.com/reel/Cabc123xyz/"
	if len(os.Args) > 1 {
		question = os.Args[1]
	}

	c, err := client.Dial(client.Options{HostPort: config.Address(), Namespace: config.Namespace()})
	if err != nil {
		log.Fatalln("unable to create client:", err)
	}
	defer c.Close()

	run, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		ID:        fmt.Sprintf("factcheck-%d", time.Now().UnixNano()),
		TaskQueue: config.TaskQueue(),
	}, factcheck.FactCheckWorkflow, question)
	if err != nil {
		log.Fatalln("unable to start workflow:", err)
	}
	log.Println("started", run.GetID(), run.GetRunID())

	var report factcheck.Report
	if err := run.Get(context.Background(), &report); err != nil {
		log.Fatalln("workflow failed:", err)
	}
	log.Println("result:\n" + report.Summary)
}
